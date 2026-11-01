package modules

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

type DeployOptions struct {
	ModuleDir string
	TenantID  string
	Name      string
	Version   string
}

type Deployment struct {
	ContainerID string `json:"container_id"`
	ImageName   string `json:"image_name"`
	ImageTag    string `json:"image_tag"`
	HostPort    string `json:"host_port"`
	Status      string `json:"status"`
}

type builtImage struct {
	name     string
	tag      string
	port     int
	buildLog string
}

func Deploy(ctx context.Context, docker *client.Client, opts DeployOptions) (*Deployment, error) {
	image, err := createImage(ctx, docker, opts)
	if err != nil {
		return nil, err
	}

	tenantID := opts.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	version := opts.Version
	if version == "" {
		version = "latest"
	}

	ctr, err := runContainer(ctx, docker, image, tenantID, version)
	if err != nil {
		return nil, err
	}

	return &Deployment{
		ContainerID: ctr.ID,
		ImageName:   image.name,
		ImageTag:    image.tag,
		HostPort:    ctr.HostPort,
		Status:      "running",
	}, nil
}

func createImage(ctx context.Context, docker *client.Client, opts DeployOptions) (*builtImage, error) {
	cfg, err := ValidateModule(opts.ModuleDir)
	if err != nil {
		return nil, err
	}

	dockerfile, err := findDockerfile(opts.ModuleDir)
	if err != nil {
		return nil, err
	}

	name := opts.Name
	if name == "" {
		name = cfg.Name
	}
	version := opts.Version
	if version == "" {
		version = "latest"
	}

	tenantID := opts.TenantID
	if tenantID == "" {
		tenantID = "default"
	}

	imageName := sanitize(tenantID) + "/" + sanitize(name)
	imageTag := imageName + ":" + sanitize(version)

	tarStream, err := createTarStream(opts.ModuleDir)
	if err != nil {
		return nil, err
	}

	result, err := docker.ImageBuild(ctx, tarStream, client.ImageBuildOptions{
		Dockerfile: dockerfile,
		Tags:       []string{imageTag},
		Remove:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("docker build: %w", err)
	}
	defer result.Body.Close()

	buildLog, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("read build log: %w", err)
	}

	return &builtImage{
		name:     imageName,
		tag:      imageTag,
		port:     cfg.Port,
		buildLog: truncate(string(buildLog), 4000),
	}, nil
}

type runningContainer struct {
	ID       string
	HostPort string
}

func runContainer(ctx context.Context, docker *client.Client, image *builtImage, tenantID, version string) (*runningContainer, error) {
	port, err := network.ParsePort(strconv.Itoa(image.port) + "/tcp")
	if err != nil {
		return nil, fmt.Errorf("invalid container port: %w", err)
	}

	name := sanitize(tenantID) + "-" + sanitize(strings.TrimPrefix(image.name, sanitize(tenantID)+"/"))
	suffix, err := randomHex(4)
	if err != nil {
		return nil, err
	}
	name += "-" + suffix

	cfg := &container.Config{
		Image:        image.tag,
		ExposedPorts: network.PortSet{port: {}},
		Env:          containerEnv(tenantID, version, image.port, image.name),
	}
	hostCfg := &container.HostConfig{
		PortBindings: network.PortMap{
			port: {{}},
		},
	}

	created, err := docker.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     cfg,
		HostConfig: hostCfg,
		Name:       name,
	})
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	if _, err := docker.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	hostPort := ""
	inspect, err := docker.ContainerInspect(ctx, created.ID, client.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("inspect container: %w", err)
	}
	if inspect.Container.NetworkSettings != nil {
		if bindings := inspect.Container.NetworkSettings.Ports[port]; len(bindings) > 0 {
			hostPort = bindings[0].HostPort
		}
	}

	return &runningContainer{ID: created.ID, HostPort: hostPort}, nil
}

func containerEnv(tenantID, version string, port int, imageName string) []string {
	return []string{
		"PORT=" + strconv.Itoa(port),
		"FUNCTION_NAME=" + strings.TrimPrefix(imageName, sanitize(tenantID)+"/"),
		"TENANT_ID=" + tenantID,
		"FUNCTION_VERSION=" + version,
	}
}

func createTarStream(sourceDir string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("error creating build context: %w", err)
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}

func sanitize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return '-'
	}, s)
	return strings.Trim(s, "-.")
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "... (truncated)"
}
