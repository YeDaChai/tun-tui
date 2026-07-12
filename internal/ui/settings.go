package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tun-tui/internal/api"
	"tun-tui/internal/config"
)

const projectURL = "https://github.com/YeDaChai/tun-tui"

type clearDataMsg struct {
	secret string
	err    error
}

func (m Model) openSettingsScreen() Model {
	m.screen = screenSettings
	m.err = ""
	m.settingsNote = ""
	return m
}

func (m Model) closeSettingsScreen() Model {
	m.screen = screenMain
	m.settingsNote = ""
	return m
}

func (m Model) updateSettingsScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "p":
		return m.closeSettingsScreen(), nil
	case "d":
		if m.busy || m.starting || m.loadingNodes {
			return m, nil
		}
		m.busy = true
		m.settingsNote = ""
		m.err = ""
		return m, m.clearDataCmd()
	case "ctrl+c", "q":
		_ = m.runner.Stop()
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) clearDataCmd() tea.Cmd {
	return func() tea.Msg {
		if m.running {
			_ = m.runner.Stop()
		}
		secret, err := config.ClearAppData(m.paths.DataDir)
		return clearDataMsg{secret: secret, err: err}
	}
}

func (m Model) applyClearedData(msg clearDataMsg) Model {
	m.busy = false
	if msg.err != nil {
		m.err = msg.err.Error()
		m.settingsNote = ""
		return m
	}

	m.paths.APISecret = msg.secret
	m.api.SetSecret(msg.secret)
	m.runner.SetSecret(msg.secret)

	m.running = false
	m.starting = false
	m.loadingNodes = false
	m.nodes = nil
	m.delays = map[string]uint16{}
	m.cursor, m.rowOffset = 0, 0
	m.mode = ""
	m.traffic = api.Traffic{}
	m.group = api.Proxy{}
	m.provider = api.ProxyProvider{}
	m.subscriptionURL = ""
	m.hasSubscription = false
	m.linkURLs = nil
	m.linkActive = -1
	m.linkCursor, m.linkRowOffset = 0, 0
	m.err = ""
	m.settingsNote = "已清理本地数据，请重新添加订阅"
	return m
}

func (m Model) viewSettingsScreen() string {
	w := m.width
	if w <= 0 {
		w = m.contentWidth()
	}
	h := m.height
	if h <= 0 {
		h = 24
	}
	return overlayCenter(m.viewMain(), m.renderSettingsBox(), w, h)
}

func (m Model) settingsModalWidth() int {
	w := m.contentWidth() - 8
	if w > 56 {
		w = 56
	}
	if w < 36 {
		w = m.contentWidth()
		if w > 36 {
			w = 36
		}
	}
	if w < 28 {
		w = 28
	}
	return w
}

func (m Model) renderSettingsBox() string {
	modalW := m.settingsModalWidth()
	innerW := modalW - 4
	if innerW < 20 {
		innerW = 20
	}

	var body strings.Builder
	body.WriteString(sectionTitle.Render("设置") + "\n")
	body.WriteString(dividerStyle.Render(strings.Repeat("─", innerW)) + "\n\n")

	body.WriteString(textSubtle.Render("数据目录") + "\n")
	body.WriteString(modeActive.Render(truncate(m.paths.DataDir, innerW)) + "\n\n")

	body.WriteString(textSubtle.Render("Git") + "\n")
	body.WriteString(modeActive.Render(truncate(projectURL, innerW)) + "\n")

	if m.settingsNote != "" {
		body.WriteString("\n" + textSubtle.Render(truncate(m.settingsNote, innerW)) + "\n")
	}
	if m.err != "" {
		body.WriteString("\n" + textErr.Render("! "+truncate(m.err, innerW-2)) + "\n")
	}

	body.WriteString("\n")
	body.WriteString(dividerStyle.Render(strings.Repeat("─", innerW)) + "\n")
	body.WriteString(footerKey.Render("D") + footerLabel.Render(" 清理数据"))
	body.WriteString(footerSep.Render("  "))
	body.WriteString(footerKey.Render("ESC") + footerLabel.Render(" 关闭"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalW).
		Render(strings.TrimRight(body.String(), "\n"))
}
