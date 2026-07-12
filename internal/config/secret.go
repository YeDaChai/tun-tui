package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func apiSecretFile(dataDir string) string {
	return filepath.Join(dataDir, "api.secret")
}

// LoadOrCreateAPISecret returns a per-install secret for the local controller.
func LoadOrCreateAPISecret(dataDir string) (string, error) {
	path := apiSecretFile(dataDir)
	if data, err := os.ReadFile(path); err == nil {
		if s := strings.TrimSpace(string(data)); s != "" {
			return s, nil
		}
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api secret: %w", err)
	}
	secret := hex.EncodeToString(buf)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write api secret: %w", err)
	}
	_ = chownToSudoUser(path)
	return secret, nil
}
