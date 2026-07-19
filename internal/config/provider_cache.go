package config

import (
	"os"
	"path/filepath"
)

const providerCacheFile = "subscription.yaml"

func providerCachePath(dataDir string) string {
	return filepath.Join(dataDir, "providers", providerCacheFile)
}

// ClearProviderCache drops the on-disk node list so the next Parse/Update fetches fresh.
// Call when the active subscription URL changes — not on ordinary connect (reuse cache).
func ClearProviderCache(dataDir string) error {
	if err := os.MkdirAll(filepath.Join(dataDir, "providers"), 0o755); err != nil {
		return err
	}
	_ = ChownToSudoUser(filepath.Join(dataDir, "providers"))
	_ = os.Remove(providerCachePath(dataDir))
	_ = os.Remove(filepath.Join(dataDir, "providers", "subscription.source"))
	return nil
}
