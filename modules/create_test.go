package modules

import (
	"context"
	"strings"
	"testing"
)

func TestDeployRejectsInvalidModule(t *testing.T) {
	_, err := Deploy(context.Background(), nil, DeployOptions{ModuleDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "function.json") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestContainerEnv(t *testing.T) {
	env := containerEnv("acme", "v1", 3000, "acme/hello")
	want := map[string]string{
		"PORT":             "3000",
		"FUNCTION_NAME":    "hello",
		"TENANT_ID":        "acme",
		"FUNCTION_VERSION": "v1",
	}
	got := make(map[string]string)
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("invalid env entry %q", entry)
		}
		got[key] = value
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %q, want %q", key, got[key], value)
		}
	}
}

func TestSanitize(t *testing.T) {
	tests := map[string]string{
		"Hello World": "hello-world",
		"Acme_Corp":   "acme_corp",
		"  v1.2.3  ":  "v1.2.3",
		"a/b":         "a-b",
	}
	for input, want := range tests {
		if got := sanitize(input); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", input, got, want)
		}
	}
}
