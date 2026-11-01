package modules

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}

func TestExtractZipAndFindModuleRoot(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "bundle.zip")
	writeZip(t, zipPath, map[string]string{
		"hello/function.json": `{"name":"hello","runtime":"node20"}`,
		"hello/package.json":  `{"name":"hello"}`,
		"hello/index.js":      `module.exports = async () => ({});`,
		"hello/Dockerfile":    "FROM node:20-alpine\n",
	})

	dest := t.TempDir()
	if err := ExtractZip(zipPath, dest); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}

	root, err := FindModuleRoot(dest)
	if err != nil {
		t.Fatalf("FindModuleRoot: %v", err)
	}
	if root != filepath.Join(dest, "hello") {
		t.Fatalf("root = %q", root)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "evil.zip")
	writeZip(t, zipPath, map[string]string{
		"../evil.txt": "bad",
	})

	if err := ExtractZip(zipPath, t.TempDir()); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestFindModuleRootRejectsMultiple(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a/function.json", `{"name":"a","runtime":"node20"}`)
	writeTestFile(t, dir, "b/function.json", `{"name":"b","runtime":"node20"}`)

	if _, err := FindModuleRoot(dir); err == nil {
		t.Fatal("expected multiple functions error")
	}
}
