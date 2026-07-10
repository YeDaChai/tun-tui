package core

import (
	"os"
	"path/filepath"
)

func CleanupGeoFiles(dataDir string) {
	const minMMDBSize = 1024 * 1024
	for _, name := range []string{"geoip.metadb", "geoip.metadb.download"} {
		path := filepath.Join(dataDir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if name == "geoip.metadb" && info.Size() >= minMMDBSize {
			continue
		}
		_ = os.Remove(path)
	}
}
