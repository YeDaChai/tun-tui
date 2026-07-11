package ui

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"tun-tui/internal/api"
	"tun-tui/internal/config"
	"tun-tui/internal/core"
)

func Run(ctx context.Context, paths config.Paths, runner *core.Runner, client *api.Client, appVersion, binName string) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("需要在交互式终端中运行，不能从 IDE 的 Run/Debug 按钮直接启动\n请在终端执行:\n  %s", binName)
	}

	p := tea.NewProgram(
		New(paths, runner, client, appVersion),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
