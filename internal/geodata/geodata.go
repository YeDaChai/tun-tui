package geodata

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

const (
	minGeoIPSize    = 1024 * 1024
	minGeositeSize  = 1024 * 1024
	geoIPFileName   = "geoip.metadb"
	geositeFileName = "geosite.dat"
)

//go:embed geoip.metadb
var bundledGeoIP []byte

//go:embed geosite.dat
var bundledGeosite []byte

func Install(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	if err := installFile(dataDir, geoIPFileName, bundledGeoIP, minGeoIPSize); err != nil {
		return err
	}
	if err := installFile(dataDir, geositeFileName, bundledGeosite, minGeositeSize); err != nil {
		return err
	}
	return nil
}

// Ready reports whether bundled rule databases are present and usable.
func Ready(dataDir string) bool {
	return fileReady(filepath.Join(dataDir, geoIPFileName), minGeoIPSize) &&
		fileReady(filepath.Join(dataDir, geositeFileName), minGeositeSize)
}

func fileReady(path string, minSize int64) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() >= minSize
}

func installFile(dataDir, name string, data []byte, minSize int64) error {
	if int64(len(data)) < minSize {
		return fmt.Errorf("bundled %s is too small (%d bytes)", name, len(data))
	}

	path := filepath.Join(dataDir, name)
	if info, err := os.Stat(path); err == nil && info.Size() >= minSize {
		return nil
	}

	tmp := path + ".download"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install %s: %w", name, err)
	}
	return nil
}
