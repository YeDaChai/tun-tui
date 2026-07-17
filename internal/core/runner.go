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

	"github.com/metacubex/mihomo/component/updater"
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
	mu         sync.Mutex
	running    bool
	dataDir    string
	cfgPath    string
	secret     string
	logFile    *os.File
	logReady   bool
	readyCheck ReadyFunc // nil → DefaultReadyCheck
}

func NewRunner(dataDir, cfgPath, secret string) *Runner {
	return &Runner{dataDir: dataDir, cfgPath: cfgPath, secret: secret}
}

func (r *Runner) SetSecret(secret string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secret = secret
}

// SetReadyCheck injects a custom readiness probe (tests). Pass nil to restore default.
func (r *Runner) SetReadyCheck(fn ReadyFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.readyCheck = fn
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

	cfgBytes, err := r.prepareConfig()
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
	if err := r.verifyReady(logPos); err != nil {
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

// ponytail: 2MiB ceiling — enough for TUN error scans; rotate to .old instead of unbounded growth.
const maxMihomoLogBytes = 2 << 20

func (r *Runner) setupLogging() error {
	if r.logReady {
		return nil
	}
	path := filepath.Join(r.dataDir, "mihomo.log")
	rotateLogIfNeeded(path, maxMihomoLogBytes)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	_ = config.ChownToSudoUser(path)
	logrus.SetOutput(f)
	mihomolog.SetLevel(mihomolog.WARNING)
	r.logFile = f
	r.logReady = true
	return nil
}

func rotateLogIfNeeded(path string, maxBytes int64) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxBytes {
		return
	}
	_ = os.Rename(path, path+".old")
}

func (r *Runner) closeLogging() {
	if r.logFile != nil {
		_ = r.logFile.Close()
		r.logFile = nil
	}
	logrus.SetOutput(io.Discard)
	r.logReady = false
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

func logOffset(dataDir string) int64 {
	info, err := os.Stat(filepath.Join(dataDir, "mihomo.log"))
	if err != nil {
		return 0
	}
	return info.Size()
}

func (r *Runner) verifyReady(logPos int64) error {
	if !TunBuildReady() {
		return fmt.Errorf("%s", TunBuildHint())
	}
	check := r.readyCheck
	if check == nil {
		dataDir := r.dataDir
		check = DefaultReadyCheck(config.APIControllerAddr, r.secret, func() error {
			return scanTunStartErrors(dataDir, logPos)
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return check(ctx)
}

func scanTunStartErrors(dataDir string, offset int64) error {
	data, err := os.ReadFile(filepath.Join(dataDir, "mihomo.log"))
	if err != nil || int64(len(data)) <= offset {
		return nil
	}
	lines := strings.Split(string(data[offset:]), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.Contains(line, "External controller listen error") && strings.Contains(line, "address already in use") {
			return fmt.Errorf("控制端口 %s 被占用（常见于 Clash / ClashLite 的 Tunnel 进程仍在运行），请完全退出后再试", config.APIControllerAddr)
		}
		if strings.Contains(line, "can't initial GeoSite") || strings.Contains(line, "can't initial GeoIP") {
			return fmt.Errorf("地理数据初始化失败，请重试或检查网络；详见 mihomo.log")
		}
		if !strings.Contains(line, "Start TUN listening error") {
			continue
		}
		switch {
		case strings.Contains(line, "operation not permitted"):
			return fmt.Errorf("VPN 需要管理员权限，请以 root / 管理员身份运行")
		case strings.Contains(line, "gVisor is not included"):
			return fmt.Errorf("当前版本未包含 gVisor，请下载最新 Release 或重新编译")
		case strings.Contains(line, "file exists"):
			return fmt.Errorf("TUN 路由残留，请先执行: sudo tun-tui -cleanup")
		}
		if idx := strings.Index(line, "msg=\""); idx >= 0 {
			msg := line[idx+5:]
			if end := strings.Index(msg, "\""); end >= 0 {
				return fmt.Errorf("VPN 启动失败: %s", msg[:end])
			}
		}
		return fmt.Errorf("VPN 启动失败，请查看数据目录中的 mihomo.log")
	}
	return nil
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
