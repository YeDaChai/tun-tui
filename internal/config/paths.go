package config

import (
	_ "embed"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"tun-tui/internal/geodata"
)

//go:embed default_config.yaml
var defaultConfig []byte

type Paths struct {
	DataDir    string
	ConfigFile string
	APIBase    string
	APISecret  string
}

func ResolvePaths(dataDirFlag string) (Paths, error) {
	dataDir, err := resolveDataDir(dataDirFlag)
	if err != nil {
		return Paths{}, err
	}
	if err := bootstrap(dataDir); err != nil {
		return Paths{}, err
	}
	secret, err := LoadOrCreateAPISecret(dataDir)
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		DataDir:    dataDir,
		ConfigFile: filepath.Join(dataDir, "config.yaml"),
		APIBase:    "http://127.0.0.1:9090",
		APISecret:  secret,
	}, nil
}

func resolveDataDir(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
	}
	if env := os.Getenv("TUN_TUI_DATA_DIR"); env != "" {
		return filepath.Abs(env)
	}
	if cwd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "data", "config.yaml")); err == nil {
			return filepath.Join(cwd, "data"), nil
		}
	}
	return defaultUserDataDir()
}

func defaultUserDataDir() (string, error) {
	home, err := effectiveHome()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "tun-tui"), nil
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "tun-tui"), nil
		}
		return filepath.Join(home, "AppData", "Roaming", "tun-tui"), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "tun-tui"), nil
		}
		return filepath.Join(home, ".local", "share", "tun-tui"), nil
	}
}

func effectiveHome() (string, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		if u, err := user.Lookup(sudoUser); err == nil && u.HomeDir != "" {
			return u.HomeDir, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return home, nil
}

func bootstrap(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	_ = chownToSudoUser(dataDir)
	if err := os.MkdirAll(filepath.Join(dataDir, "providers"), 0o755); err != nil {
		return fmt.Errorf("create providers dir: %w", err)
	}
	_ = chownToSudoUser(filepath.Join(dataDir, "providers"))

	cfgPath := filepath.Join(dataDir, "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.WriteFile(cfgPath, defaultConfig, 0o600); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
	}
	_ = chownToSudoUser(cfgPath)

	if err := geodata.Install(dataDir); err != nil {
		return fmt.Errorf("install bundled geodata: %w", err)
	}
	_ = chownToSudoUser(filepath.Join(dataDir, "geoip.metadb"))
	_ = chownToSudoUser(filepath.Join(dataDir, "geosite.dat"))
	return nil
}
