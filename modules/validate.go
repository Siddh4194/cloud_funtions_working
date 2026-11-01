package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultEntry = "index.js"
	defaultPort  = 3000
)

type FunctionConfig struct {
	Name    string `json:"name"`
	Runtime string `json:"runtime"`
	Entry   string `json:"entry"`
	Port    int    `json:"port"`
}

func ValidateModule(dir string) (*FunctionConfig, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("module directory is required")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("module directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("module path is not a directory: %s", dir)
	}

	cfg, err := readFunctionConfig(dir)
	if err != nil {
		return nil, err
	}

	if err := validateName(cfg.Name); err != nil {
		return nil, err
	}

	switch cfg.Runtime {
	case "node20":
	default:
		return nil, fmt.Errorf("unsupported runtime %q (supported: node20)", cfg.Runtime)
	}

	if cfg.Entry == "" {
		cfg.Entry = defaultEntry
	}
	if err := validateEntry(dir, cfg.Entry); err != nil {
		return nil, err
	}

	if err := validatePackageJSON(dir); err != nil {
		return nil, err
	}

	if _, err := findDockerfile(dir); err != nil {
		return nil, err
	}

	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("invalid port %d", cfg.Port)
	}

	return cfg, nil
}

func readFunctionConfig(dir string) (*FunctionConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, "function.json"))
	if err != nil {
		return nil, fmt.Errorf("function.json is required: %w", err)
	}

	var cfg FunctionConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid function.json: %w", err)
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("function.json: name is required")
	}
	if cfg.Runtime == "" {
		return nil, fmt.Errorf("function.json: runtime is required")
	}
	return &cfg, nil
}

func validateName(name string) error {
	if len(name) == 0 || len(name) > 63 {
		return fmt.Errorf("name must be between 1 and 63 characters")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return fmt.Errorf("name %q may only contain lowercase letters, digits, '-', '_' and '.'", name)
		}
	}
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "-") {
		return fmt.Errorf("name must not start with '.' or '-'")
	}
	return nil
}

func validateEntry(dir, entry string) error {
	clean := filepath.Clean(filepath.FromSlash(entry))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("entry must be a relative path inside the module")
	}

	switch strings.ToLower(filepath.Ext(clean)) {
	case ".js", ".cjs", ".mjs":
	default:
		return fmt.Errorf("entry must be a JavaScript file")
	}

	info, err := os.Stat(filepath.Join(dir, clean))
	if err != nil {
		return fmt.Errorf("entry %q not found: %w", entry, err)
	}
	if info.IsDir() {
		return fmt.Errorf("entry %q is a directory", entry)
	}
	return nil
}

func validatePackageJSON(dir string) error {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return fmt.Errorf("package.json is required: %w", err)
	}
	if !json.Valid(data) {
		return fmt.Errorf("package.json is not valid JSON")
	}
	return nil
}

func findDockerfile(dir string) (string, error) {
	var found string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !isDockerfileName(info.Name()) {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		found = filepath.ToSlash(rel)
		return filepath.SkipAll
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("Dockerfile is required")
	}
	return found, nil
}

func isDockerfileName(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	return base == "dockerfile" || strings.HasPrefix(base, "dockerfile.")
}
