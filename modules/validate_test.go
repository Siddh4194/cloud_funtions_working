package modules

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func validModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "function.json", `{"name":"hello","runtime":"node20","entry":"index.js","port":3000}`)
	writeTestFile(t, dir, "package.json", `{"name":"hello"}`)
	writeTestFile(t, dir, "index.js", `module.exports = async () => ({ statusCode: 200 });`)
	writeTestFile(t, dir, "Dockerfile", "FROM node:20-alpine\n")
	return dir
}

func TestValidateModule(t *testing.T) {
	cfg, err := ValidateModule(validModule(t))
	if err != nil {
		t.Fatalf("ValidateModule: %v", err)
	}
	if cfg.Name != "hello" || cfg.Runtime != "node20" || cfg.Entry != "index.js" || cfg.Port != 3000 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestValidateModuleDefaults(t *testing.T) {
	dir := validModule(t)
	writeTestFile(t, dir, "function.json", `{"name":"hello","runtime":"node20"}`)

	cfg, err := ValidateModule(dir)
	if err != nil {
		t.Fatalf("ValidateModule: %v", err)
	}
	if cfg.Entry != defaultEntry || cfg.Port != defaultPort {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

func TestValidateModuleRejects(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(t *testing.T, dir string)
		wantErr bool
	}{
		{
			name: "missing dockerfile",
			mutate: func(t *testing.T, dir string) {
				os.Remove(filepath.Join(dir, "Dockerfile"))
			},
		},
		{
			name: "missing package json",
			mutate: func(t *testing.T, dir string) {
				os.Remove(filepath.Join(dir, "package.json"))
			},
		},
		{
			name: "entry escapes module",
			mutate: func(t *testing.T, dir string) {
				writeTestFile(t, dir, "function.json", `{"name":"hello","runtime":"node20","entry":"../evil.js"}`)
			},
		},
		{
			name: "unsupported runtime",
			mutate: func(t *testing.T, dir string) {
				writeTestFile(t, dir, "function.json", `{"name":"hello","runtime":"python3"}`)
			},
		},
		{
			name: "invalid name",
			mutate: func(t *testing.T, dir string) {
				writeTestFile(t, dir, "function.json", `{"name":"Hello World","runtime":"node20"}`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := validModule(t)
			tt.mutate(t, dir)
			if _, err := ValidateModule(dir); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}
