package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const projectURL = "https://github.com/YeDaChai/tun-tui"

func (m Model) openSettingsScreen() Model {
	m.screen = screenSettings
	m.err = ""
	return m
}

func (m Model) closeSettingsScreen() Model {
	m.screen = screenMain
	return m
}

func (m Model) updateSettingsScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "p":
		return m.closeSettingsScreen(), nil
	case "ctrl+c", "q":
		_ = m.runner.Stop()
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewSettingsScreen() string {
	w := m.contentWidth()
	if w < 40 {
		w = 40
	}
	f := newFrame(w, true)
	var b strings.Builder
	b.WriteString(f.top() + "\n")
	b.WriteString(f.row(sectionTitle.Render(" 设置 ")) + "\n")
	b.WriteString(f.row(dividerStyle.Render(strings.Repeat("─", w))) + "\n")
	b.WriteString(f.row("") + "\n")

	b.WriteString(f.row(textSubtle.Render(" 当前数据目录")) + "\n")
	b.WriteString(f.row(modeActive.Render("  "+m.paths.DataDir)) + "\n")
	b.WriteString(f.row("") + "\n")

	b.WriteString(f.row(textSubtle.Render(" 默认数据目录")) + "\n")
	b.WriteString(f.row(textSubtle.Render("  macOS    ")+modeActive.Render("~/Library/Application Support/tun-tui")) + "\n")
	b.WriteString(f.row(textSubtle.Render("  Windows  ")+modeActive.Render(`%APPDATA%\tun-tui`)) + "\n")
	b.WriteString(f.row(textSubtle.Render("  Linux    ")+modeActive.Render("~/.local/share/tun-tui")) + "\n")
	b.WriteString(f.row("") + "\n")

	b.WriteString(f.row(textSubtle.Render(" Git")) + "\n")
	b.WriteString(f.row(modeActive.Render("  "+projectURL)) + "\n")
	b.WriteString(f.row("") + "\n")
	b.WriteString(f.bottom() + "\n\n")
	b.WriteString(footerKey.Render("esc") + footerLabel.Render(" 关闭") + "\n")
	return b.String()
}
