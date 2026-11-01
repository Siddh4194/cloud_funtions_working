package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/moby/moby/client"

	"cloudfunctions.com/modules"
)

type server struct {
	docker    *client.Client
	maxUpload int64
}

func main() {
	dockerClient, err := client.New(client.FromEnv, client.WithUserAgent("cloud-functions-api"))
	if err != nil {
		log.Fatalf("docker client: %v", err)
	}
	defer dockerClient.Close()

	s := &server{
		docker:    dockerClient,
		maxUpload: envInt64("MAX_UPLOAD_BYTES", 50<<20),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /functions", s.handleDeploy)

	addr := ":" + env("API_PORT", "8080")
	log.Printf("cloud functions API listening on %s", addr)
	if err := http.ListenAndServe(addr, logRequests(mux)); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

func (s *server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, s.maxUpload)
	if err := r.ParseMultipartForm(s.maxUpload); err != nil {
		writeErr(w, http.StatusBadRequest, "failed to parse multipart form", err)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file field is required", err)
		return
	}
	defer file.Close()

	zipFile, err := os.CreateTemp("", "function-*.zip")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to stage upload", err)
		return
	}
	defer os.Remove(zipFile.Name())
	defer zipFile.Close()

	if _, err := io.Copy(zipFile, file); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save upload", err)
		return
	}
	if err := zipFile.Close(); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save upload", err)
		return
	}

	workDir, err := os.MkdirTemp("", "function-*")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create workspace", err)
		return
	}
	defer os.RemoveAll(workDir)

	if err := modules.ExtractZip(zipFile.Name(), workDir); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid function archive", err)
		return
	}

	moduleDir, err := modules.FindModuleRoot(workDir)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid function archive", err)
		return
	}

	if _, err := modules.ValidateModule(moduleDir); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid function module", err)
		return
	}

	result, err := modules.Deploy(ctx, s.docker, modules.DeployOptions{
		ModuleDir: moduleDir,
		TenantID:  r.FormValue("tenant_id"),
		Name:      r.FormValue("name"),
		Version:   r.FormValue("version"),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "deploy failed", err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string, err error) {
	detail := msg
	if err != nil {
		detail = fmt.Sprintf("%s: %v", msg, err)
	}
	log.Printf("error: %s", detail)
	writeJSON(w, status, map[string]any{"error": msg, "detail": detail})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	v, err := strconv.ParseInt(os.Getenv(key), 10, 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
