package core

import (
	"os"
	"path/filepath"
)

func CleanupGeoFiles(dataDir string) {
	const minMMDBSize = 1024 * 1024
	const minGeositeSize = 1024 * 1024
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
