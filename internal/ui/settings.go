package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"tun-tui/internal/config"
	"tun-tui/internal/update"
	"tun-tui/internal/version"
)

const projectURL = "https://github.com/" + update.RepoOwner + "/" + update.RepoName

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
		if m.work.busy() {
			return m, nil
		}
		m.work = workActing
		m.settingsNote = ""
		m.err = ""
		return m, m.clearDataCmd()
	case "ctrl+c", "q":
		m.stopDelayTest()
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
	m.work = workIdle
	if msg.err != nil {
		m.err = msg.err.Error()
		m.settingsNote = ""
		return m
	}

	m.paths.APISecret = msg.secret
	m.api.SetSecret(msg.secret)
	m.runner.SetSecret(msg.secret)

	m = m.resetIdleState()
	m.subscriptionURL = ""
	m.hasSubscription = false
	m.linkURLs = nil
	m.linkActive = -1
	m.linkCursor, m.linkRowOffset = 0, 0
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

func (m Model) renderSettingsBox() string {
	modalW := m.modalWidth(60)
	innerW := modalW - 4
	if innerW < 20 {
		innerW = 20
	}

	var body strings.Builder
	body.WriteString(sectionTitle.Render(scrollPlaque("设置")) + "\n")
	body.WriteString(mistSep(innerW) + "\n\n")

	body.WriteString(textSubtle.Render("版本") + "\n")
	body.WriteString(modeActive.Render(truncate("当前 "+version.Version, innerW)) + "\n")
	body.WriteString(textSubtle.Render(truncate("更新: tun-tui update", innerW)) + "\n\n")

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
	body.WriteString(mistSep(innerW) + "\n")
	body.WriteString(antiqueButton("D", "清理数据"))
	body.WriteString(footerSep.Render(" "))
	body.WriteString(antiqueButton("ESC", "关闭"))

	return modalBox(modalW, strings.TrimRight(body.String(), "\n"))
}
