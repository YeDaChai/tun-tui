package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"go.uber.org/automaxprocs/maxprocs"

	"tun-tui/internal/api"
	"tun-tui/internal/config"
	"tun-tui/internal/core"
	"tun-tui/internal/privilege"
	"tun-tui/internal/ui"
	"tun-tui/internal/update"
	"tun-tui/internal/version"
)

func init() {
	_, _ = maxprocs.Set(maxprocs.Logger(nil))
}

func main() {
	binName := filepath.Base(os.Args[0])
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "update":
			if err := runUpdate(binName); err != nil {
				fmt.Fprintf(os.Stderr, "update: %v\n", err)
				os.Exit(1)
			}
			return
		case "help", "-help", "--help", "-h":
			printHelp(binName)
			return
		case "version", "-version", "--version":
			fmt.Println(version.Full())
			return
		}
	}

	var (
		showVersion = flag.Bool("version", false, "显示版本信息")
		showHelp    = flag.Bool("help", false, "显示帮助")
		cleanup     = flag.Bool("cleanup", false, "清理 TUN 残留路由（异常退出后修复网络）")
		dataDir     = flag.String("data-dir", "", "数据目录（也可用环境变量 TUN_TUI_DATA_DIR）")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Full())
		os.Exit(0)
	}
	if *showHelp {
		printHelp(binName)
		os.Exit(0)
	}
	if *cleanup {
		if !privilege.CanUseTUN() {
			fmt.Fprintf(os.Stderr, "清理路由需要管理员权限，请使用: sudo %s -cleanup\n", binName)
			os.Exit(1)
		}
		if err := core.CleanupTunRoutes(); err != nil {
			fmt.Fprintf(os.Stderr, "清理路由失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("TUN 残留路由已清理，网络应已恢复")
		os.Exit(0)
	}

	if !privilege.CanUseTUN() {
		fmt.Fprintf(os.Stderr, "VPN 模式需要管理员权限，请使用: sudo %s\n", binName)
		os.Exit(1)
	}

	paths, err := config.ResolvePaths(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve paths: %v\n", err)
		os.Exit(1)
	}

	runner := core.NewRunner(paths.DataDir, paths.ConfigFile, paths.APISecret)
	client := api.New(paths.APIBase, paths.APISecret)

	defer func() {
		if rec := recover(); rec != nil {
			fmt.Fprintf(os.Stderr, "fatal: %v\n", rec)
			_ = runner.Stop()
			os.Exit(1)
		}
		_ = runner.Stop()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	if err := ui.Run(ctx, paths, runner, client, binName); err != nil {
		fmt.Fprintf(os.Stderr, "ui error: %v\n", err)
		os.Exit(1)
	}
}

func runUpdate(binName string) error {
	fmt.Printf("当前版本: %s\n", version.Version)
	fmt.Println("正在检查更新…")
	info, err := update.Check(version.Version)
	if err != nil {
		return err
	}
	fmt.Printf("最新版本: %s\n", info.Latest)
	if !info.Newer {
		fmt.Println("已是最新版本")
		return nil
	}
	fmt.Printf("正在下载 %s …\n", info.AssetName)
	if err := update.Apply(info); err != nil {
		return fmt.Errorf("%w\n若权限不足，请尝试: sudo %s update", err, binName)
	}
	fmt.Printf("已更新到 v%s，请重新运行 %s\n", info.Latest, binName)
	return nil
}

func printHelp(binName string) {
	cleanupNote := "异常退出后修复网络（Linux/Windows 可重启网络）"
	if runtime.GOOS == "darwin" {
		cleanupNote = "清理 TUN 残留路由（异常退出后无法上网时使用）"
	}
	fmt.Printf(`%s — 基于 Mihomo 的 TUN 终端 VPN

用法:
  sudo %s [选项]
  %s update

命令:
  update             检查并安装最新版本

选项:
  -data-dir <路径>   数据目录（或环境变量 TUN_TUI_DATA_DIR）
  -cleanup           %s
  -version           版本信息
  -help              帮助

快捷键: S 连接/关闭 · JK 选择 · Enter 确认 · M 模式 · T 测速 · L 订阅 · P 设置 · Q 退出

示例:
  sudo %s
  sudo TUN_TUI_DATA_DIR=./data %s
  %s update
  sudo %s -cleanup

`, binName, binName, binName, cleanupNote, binName, binName, binName, binName)
}
