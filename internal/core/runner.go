package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	mihomocfg "github.com/metacubex/mihomo/config"
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/constant/features"
	"github.com/metacubex/mihomo/hub"
	"github.com/metacubex/mihomo/hub/executor"
	mihomolog "github.com/metacubex/mihomo/log"
	"github.com/metacubex/mihomo/tunnel"
	logrus "github.com/sirupsen/logrus"

	"tun-tui/internal/config"
	"tun-tui/internal/geodata"
)

type Runner struct {
	mu      sync.Mutex
	running bool
	dataDir string
	cfgPath string
	secret  string
}

func NewRunner(dataDir, cfgPath, secret string) *Runner {
	return &Runner{dataDir: dataDir, cfgPath: cfgPath, secret: secret}
}

func (r *Runner) SetSecret(secret string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secret = secret
}

func (r *Runner) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("mihomo is already running")
	}
	silenceLogging()

	cfgBytes, err := r.prepareConfig()
	if err != nil {
		return err
	}

	C.SetHomeDir(r.dataDir)
	C.SetConfig(r.cfgPath)
	if err := mihomocfg.Init(r.dataDir); err != nil {
		return fmt.Errorf("init config dir: %w", err)
	}

	if err := hub.Parse(cfgBytes); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	syncTunnelMode(r.dataDir)
	if err := r.verifyReady(); err != nil {
		executor.Shutdown()
		return err
	}

	r.running = true
	return nil
}

func (r *Runner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		CleanupTunRoutes()
		return nil
	}

	executor.Shutdown()
	CleanupTunRoutes()
	r.running = false
	return nil
}

func (r *Runner) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return fmt.Errorf("mihomo is not running")
	}
	cfgBytes, err := r.prepareConfig()
	if err != nil {
		return err
	}
	if err := hub.Parse(cfgBytes); err != nil {
		return fmt.Errorf("reload config: %w", err)
	}
	syncTunnelMode(r.dataDir)
	return nil
}

func (r *Runner) prepareConfig() ([]byte, error) {
	cleanupGeoFiles(r.dataDir)
	if err := geodata.Install(r.dataDir); err != nil {
		return nil, fmt.Errorf("install geodata: %w", err)
	}
	_ = config.ChownToSudoUser(filepath.Join(r.dataDir, "geoip.metadb"))
	_ = config.ChownToSudoUser(filepath.Join(r.dataDir, "geosite.dat"))
	return config.BuildConfigBytes(r.dataDir, r.cfgPath, r.secret)
}

func syncTunnelMode(dataDir string) {
	mode := config.LoadMode(dataDir, "rule")
	if m, ok := tunnel.ModeMapping[mode]; ok {
		tunnel.SetMode(m)
	}
}

func silenceLogging() {
	logrus.SetOutput(io.Discard)
	mihomolog.SetLevel(mihomolog.SILENT)
}

func TunBuildReady() bool {
	if runtime.GOOS == "linux" {
		return true
	}
	return features.WithGVisor
}

func TunBuildHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS TUN 需要 gVisor，请执行: make build"
	case "windows":
		return "Windows TUN 需要 gVisor，请使用官方 Release 或执行: make build GOOS=windows"
	default:
		return "TUN 不可用，请重新编译"
	}
}

func (r *Runner) verifyReady() error {
	if !TunBuildReady() {
		return fmt.Errorf("%s", TunBuildHint())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return DefaultReadyCheck(config.APIControllerAddr, r.secret)(ctx)
}

// cleanupGeoFiles removes stale GeoIP.dat (triggers download hang) and tiny/corrupt
// bundled databases so Install can restore them. Never delete geosite.dat by
// alternate casing — macOS FS is case-insensitive.
func cleanupGeoFiles(dataDir string) {
	if entries, err := os.ReadDir(dataDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.ToLower(e.Name()) == "geoip.dat" {
				path := filepath.Join(dataDir, e.Name())
				_ = os.Remove(path)
				_ = os.Remove(path + ".download")
			}
		}
	}
	for _, name := range []string{"geoip.metadb", "geosite.dat"} {
		path := filepath.Join(dataDir, name)
		info, err := os.Stat(path)
		if err != nil || info.Size() >= geodata.MinFileSize {
			continue
		}
		_ = os.Remove(path)
		_ = os.Remove(path + ".download")
	}
}
