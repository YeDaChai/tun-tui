package config

import (
	"os"
	"path/filepath"
	"strings"
)

func modePath(dataDir string) string {
	return filepath.Join(dataDir, "mode")
}

func LoadMode(dataDir, fallback string) string {
	fallback = normalizeMode(fallback)
	if fallback == "" {
		fallback = "rule"
	}

	path := modePath(dataDir)
	if data, err := os.ReadFile(path); err == nil {
		if mode := normalizeMode(strings.TrimSpace(string(data))); mode != "" {
			return mode
		}
	}

	return fallback
}

func SaveMode(dataDir, mode string) error {
	mode = normalizeMode(mode)
	if mode == "" {
		return nil
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	path := modePath(dataDir)
	if err := os.WriteFile(path, []byte(mode+"\n"), 0o644); err != nil {
		return err
	}
	return chownToSudoUser(path)
}

func NormalizeMode(mode string) string {
	return normalizeMode(mode)
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "rule", "global", "direct":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return ""
	}
}

func applyMode(dataDir string, root map[string]any) {
	fallback, _ := root["mode"].(string)
	root["mode"] = LoadMode(dataDir, fallback)
}
