package config

import (
	_ "embed"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

//go:embed default_config.yaml
var defaultConfig []byte

// APIControllerAddr is the local Mihomo external-controller bind address.
const APIControllerAddr = "127.0.0.1:9090"

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
		APIBase:    "http://" + APIControllerAddr,
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
	_ = ChownToSudoUser(dataDir)
	if err := os.MkdirAll(filepath.Join(dataDir, "providers"), 0o755); err != nil {
		return fmt.Errorf("create providers dir: %w", err)
	}
	_ = ChownToSudoUser(filepath.Join(dataDir, "providers"))

	cfgPath := filepath.Join(dataDir, "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.WriteFile(cfgPath, defaultConfig, 0o600); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
	}
	_ = ChownToSudoUser(cfgPath)
	// Geodata is installed on connect/reload via core.Runner.prepareConfig.
	return nil
}

// ClearAppData removes all files under the data directory and re-bootstraps defaults.
// Used when old local data is incompatible with a newer version.
func ClearAppData(dataDir string) (string, error) {
	if dataDir == "" {
		return "", fmt.Errorf("数据目录为空")
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("读取数据目录失败: %w", err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dataDir, entry.Name())); err != nil {
			return "", fmt.Errorf("清理 %s 失败: %w", entry.Name(), err)
		}
	}
	if err := bootstrap(dataDir); err != nil {
		return "", err
	}
	return LoadOrCreateAPISecret(dataDir)
}
