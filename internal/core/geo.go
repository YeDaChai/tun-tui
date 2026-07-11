package core

import (
	"os"
	"path/filepath"
	"strings"
)

func CleanupGeoFiles(dataDir string) {
	const minMMDBSize = 1024 * 1024
	const minGeositeSize = 1024 * 1024

	// Purge classic GeoIP.dat leftovers only. We run with geodata-mode:false and
	// ship geoip.metadb; a stale GeoIP.dat makes mihomo try (and hang on)
	// re-downloading it. Never remove geosite.dat — on macOS the FS is
	// case-insensitive, so deleting "GeoSite.dat" would wipe the bundled file.
	entries, err := os.ReadDir(dataDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			if lower == "geoip.dat" {
				path := filepath.Join(dataDir, name)
				_ = os.Remove(path)
				_ = os.Remove(path + ".download")
			}
		}
	}

	for _, item := range []struct {
		name    string
		minSize int64
	}{
		{"geoip.metadb", minMMDBSize},
		{"geosite.dat", minGeositeSize},
	} {
		path := filepath.Join(dataDir, item.name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() >= item.minSize {
			continue
		}
		_ = os.Remove(path)
		_ = os.Remove(path + ".download")
	}
}
