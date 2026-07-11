package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/metacubex/mihomo/constant/features"
)

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
	path := filepath.Join(dataDir, "mihomo.log")
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func verifyTunStarted(dataDir string, offset int64) error {
	if !TunBuildReady() {
		return fmt.Errorf("%s", TunBuildHint())
	}

	time.Sleep(300 * time.Millisecond)

	path := filepath.Join(dataDir, "mihomo.log")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if int64(len(data)) <= offset {
		return nil
	}

	lines := strings.Split(string(data[offset:]), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if !strings.Contains(line, "Start TUN listening error") {
			continue
		}
		if strings.Contains(line, "operation not permitted") {
			return fmt.Errorf("VPN 需要管理员权限，请以 root / 管理员身份运行")
		}
		if strings.Contains(line, "gVisor is not included") {
			return fmt.Errorf("当前版本未包含 gVisor，请下载最新 Release 或重新编译")
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
