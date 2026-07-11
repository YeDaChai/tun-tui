package config

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	providerCacheFile  = "subscription.yaml"
	providerSourceFile = "subscription.source"
)

func ProviderCachePath(dataDir string) string {
	return filepath.Join(dataDir, "providers", providerCacheFile)
}

func providerSourcePath(dataDir string) string {
	return filepath.Join(dataDir, "providers", providerSourceFile)
}

// SyncProviderCache keeps the on-disk node list when the active subscription URL
// still matches. If the URL changed, it drops the stale cache so the next start
// fetches a fresh list.
func SyncProviderCache(dataDir, currentURL string) error {
	currentURL = strings.TrimSpace(currentURL)
	if currentURL == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "providers"), 0o755); err != nil {
		return err
	}
	_ = chownToSudoUser(filepath.Join(dataDir, "providers"))

	cachePath := ProviderCachePath(dataDir)
	sourcePath := providerSourcePath(dataDir)
	source := readProviderSource(sourcePath)

	info, err := os.Stat(cachePath)
	hasCache := err == nil && info.Size() > 0
	if hasCache {
		if source == "" {
			// Existing installs: treat the cached nodes as belonging to the
			// current URL so startup can load them without a forced refresh.
			return MarkProviderCache(dataDir, currentURL)
		}
		if source == currentURL {
			return nil
		}
	}

	_ = os.Remove(cachePath)
	_ = os.Remove(sourcePath)
	return nil
}

// MarkProviderCache records which subscription URL produced the local node cache.
func MarkProviderCache(dataDir, url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "providers"), 0o755); err != nil {
		return err
	}
	path := providerSourcePath(dataDir)
	if err := os.WriteFile(path, []byte(url+"\n"), 0o600); err != nil {
		return err
	}
	_ = chownToSudoUser(path)
	_ = chownToSudoUser(ProviderCachePath(dataDir))
	return nil
}

func readProviderSource(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// HasProviderCache reports whether a non-empty local node list is available.
func HasProviderCache(dataDir string) bool {
	info, err := os.Stat(ProviderCachePath(dataDir))
	return err == nil && info.Size() > 0
}
