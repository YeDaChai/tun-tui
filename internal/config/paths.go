package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

type Paths struct {
	DataDir    string
	ConfigFile string
	APIBase    string
}

func ResolvePaths(dataDirFlag string) (Paths, error) {
	dataDir, err := resolveDataDir(dataDirFlag)
	if err != nil {
		return Paths{}, err
	}

	if err := Bootstrap(dataDir); err != nil {
		return Paths{}, err
	}

	return Paths{
		DataDir:    dataDir,
		ConfigFile: filepath.Join(dataDir, "config.yaml"),
		APIBase:    "http://127.0.0.1:9090",
	}, nil
}

func resolveDataDir(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
	}
	if env := os.Getenv("TUN_TUI_DATA_DIR"); env != "" {
		return filepath.Abs(env)
	}

	if devDir := detectDevDataDir(); devDir != "" {
		return devDir, nil
	}

	return defaultUserDataDir()
}

func detectDevDataDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(cwd, "data", "config.yaml")
	if _, err := os.Stat(candidate); err == nil {
		return filepath.Join(cwd, "data")
	}
	return ""
}

func defaultUserDataDir() (string, error) {
	home, err := effectiveHome()
	if err != nil {
		return "", err
	}

	var dir string
	switch runtime.GOOS {
	case "darwin":
		dir = filepath.Join(home, "Library", "Application Support", "tun-tui")
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			dir = filepath.Join(appdata, "tun-tui")
		} else {
			dir = filepath.Join(home, "AppData", "Roaming", "tun-tui")
		}
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			dir = filepath.Join(xdg, "tun-tui")
		} else {
			dir = filepath.Join(home, ".local", "share", "tun-tui")
		}
	}
	return dir, nil
}

func effectiveHome() (string, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		u, err := user.Lookup(sudoUser)
		if err == nil && u.HomeDir != "" {
			return u.HomeDir, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return home, nil
}
