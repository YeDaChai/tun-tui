package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/metacubex/mihomo/component/updater"
	mihomocfg "github.com/metacubex/mihomo/config"
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/hub"
	"github.com/metacubex/mihomo/hub/executor"
	mihomolog "github.com/metacubex/mihomo/log"
	"github.com/metacubex/mihomo/tunnel"
	logrus "github.com/sirupsen/logrus"

	"tun-tui/internal/config"
	"tun-tui/internal/geodata"
)

type Runner struct {
	mu       sync.Mutex
	running  bool
	dataDir  string
	cfgPath  string
	logFile  *os.File
	logReady bool
}

func NewRunner(dataDir, cfgPath string) *Runner {
	return &Runner{dataDir: dataDir, cfgPath: cfgPath}
}

func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("mihomo is already running")
	}
	if err := r.setupLogging(); err != nil {
		return err
	}

	CleanupGeoFiles(r.dataDir)
	if err := geodata.Install(r.dataDir); err != nil {
		return fmt.Errorf("install geodata: %w", err)
	}

	cfgBytes, err := config.BuildConfigBytes(r.dataDir, r.cfgPath)
	if err != nil {
		return err
	}

	C.SetHomeDir(r.dataDir)
	C.SetConfig(r.cfgPath)
	if err := mihomocfg.Init(r.dataDir); err != nil {
		return fmt.Errorf("init config dir: %w", err)
	}

	logPos := logOffset(r.dataDir)
	if err := hub.Parse(cfgBytes); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	syncTunnelMode(r.dataDir)
	if err := verifyTunStarted(r.dataDir, logPos); err != nil {
		executor.Shutdown()
		return err
	}

	if updater.GeoAutoUpdate() {
		updater.RegisterGeoUpdater()
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
	r.closeLogging()
	return nil
}

func (r *Runner) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return fmt.Errorf("mihomo is not running")
	}

	CleanupGeoFiles(r.dataDir)
	if err := geodata.Install(r.dataDir); err != nil {
		return fmt.Errorf("install geodata: %w", err)
	}

	cfgBytes, err := config.BuildConfigBytes(r.dataDir, r.cfgPath)
	if err != nil {
		return err
	}
	if err := hub.Parse(cfgBytes); err != nil {
		return fmt.Errorf("reload config: %w", err)
	}
	syncTunnelMode(r.dataDir)
	return nil
}

func syncTunnelMode(dataDir string) {
	mode := config.LoadMode(dataDir, "rule")
	if m, ok := tunnel.ModeMapping[mode]; ok {
		tunnel.SetMode(m)
	}
}

func (r *Runner) setupLogging() error {
	if r.logReady {
		return nil
	}

	path := filepath.Join(r.dataDir, "mihomo.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	logrus.SetOutput(f)
	mihomolog.SetLevel(mihomolog.WARNING)
	r.logFile = f
	r.logReady = true
	return nil
}

func (r *Runner) closeLogging() {
	if r.logFile != nil {
		_ = r.logFile.Close()
		r.logFile = nil
	}
	logrus.SetOutput(io.Discard)
	r.logReady = false
}
