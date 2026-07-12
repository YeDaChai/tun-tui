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
	"tun-tui/internal/version"
)

func init() {
	_, _ = maxprocs.Set(maxprocs.Logger(nil))
}

func main() {
	var (
		showVersion = flag.Bool("version", false, "显示版本信息")
		showHelp    = flag.Bool("help", false, "显示帮助")
		cleanup     = flag.Bool("cleanup", false, "清理 TUN 残留路由（异常退出后修复网络）")
		dataDir     = flag.String("data-dir", "", "数据目录（也可用环境变量 TUN_TUI_DATA_DIR）")
	)
	flag.Parse()

	binName := filepath.Base(os.Args[0])

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

func printHelp(binName string) {
	cleanupNote := "macOS 上可自动清理；Linux/Windows 请重启网络"
	if runtime.GOOS == "darwin" {
		cleanupNote = "清理 TUN 残留路由（异常退出后无法上网时使用）"
	}
	fmt.Printf(`%s — 终端 VPN 管理工具（基于 Mihomo utun）

通过虚拟网卡（utun）在网络层转发流量，无需应用层代理设置。

用法:
  sudo %s [选项]

选项:
  -data-dir <路径>   指定数据目录
  -cleanup           %s
  -version           显示版本信息
  -help              显示此帮助

环境变量:
  TUN_TUI_DATA_DIR  数据目录（未指定时默认使用用户目录）

数据目录:
  macOS   ~/Library/Application Support/tun-tui
  Linux   ~/.local/share/tun-tui
  Windows %%APPDATA%%\tun-tui

退出与清理:
  按 q / Ctrl+C     正常退出，自动关闭 TUN 并清理路由
  按 s 断开         仅断开 VPN，不退出程序
  kill <pid>        收到终止信号后会尝试清理
  kill -9 <pid>     无法清理，请执行: sudo %s -cleanup

TUN 会临时修改系统路由表（非系统代理、非系统 DNS 设置）。

快捷操作:
  S         连接 / 断开
  J/K       选择节点
  Enter     确认节点
  M         切换模式（分流 / 全局 / 直连）
  T         测速
  U         更新订阅
  L         管理订阅链接
  P         设置（数据目录 / Git）
  Q         退出

链接管理（按 L 进入）:
  i / a     添加链接
  Enter     使用选中链接
  d         删除选中链接
  Esc       返回主界面

设置（按 P 进入）:
  Esc       返回主界面

示例:
  sudo %s                          # 使用默认数据目录
  sudo TUN_TUI_DATA_DIR=./data %s  # 开发模式
  sudo %s -cleanup                 # 异常退出后修复网络

`, binName, binName, cleanupNote, binName, binName, binName, binName)
}
