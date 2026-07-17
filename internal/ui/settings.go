package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tun-tui/internal/config"
	"tun-tui/internal/update"
	"tun-tui/internal/version"
)

const projectURL = "https://github.com/" + update.RepoOwner + "/" + update.RepoName

type clearDataMsg struct {
	secret string
	err    error
}

type checkUpdateMsg struct {
	info update.Info
	err  error
}

type applyUpdateMsg struct {
	version string
	err     error
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
	case "c":
		if m.work.busy() {
			return m, nil
		}
		m.work = workActing
		m.err = ""
		m.settingsNote = "正在检查更新…"
		return m, checkUpdateCmd()
	case "r":
		if m.work.busy() {
			return m, nil
		}
		if !m.updateInfo.Newer || m.updateInfo.DownloadURL == "" {
			m.settingsNote = "请先按 C 检查更新"
			m.err = ""
			return m, nil
		}
		m.work = workActing
		m.err = ""
		m.settingsNote = "正在下载并安装 v" + m.updateInfo.Latest + "…"
		info := m.updateInfo
		return m, m.applyUpdateCmd(info)
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

func checkUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		info, err := update.Check(version.Version)
		return checkUpdateMsg{info: info, err: err}
	}
}

func (m Model) applyUpdateCmd(info update.Info) tea.Cmd {
	return func() tea.Msg {
		if m.running {
			_ = m.runner.Stop()
		}
		if err := update.Apply(info); err != nil {
			return applyUpdateMsg{err: err}
		}
		return applyUpdateMsg{version: info.Latest}
	}
}

func (m Model) applyCheckUpdate(msg checkUpdateMsg) Model {
	m.work = workIdle
	if msg.err != nil {
		m.err = msg.err.Error()
		m.settingsNote = ""
		return m
	}
	m.err = ""
	m.updateInfo = msg.info
	if msg.info.Newer {
		m.settingsNote = fmt.Sprintf("发现新版本 v%s，按 R 更新", msg.info.Latest)
	} else {
		m.settingsNote = fmt.Sprintf("已是最新版本 v%s", msg.info.Latest)
	}
	return m
}

func (m Model) applyAppUpdate(msg applyUpdateMsg) Model {
	m.work = workIdle
	// Apply stops the kernel before replacing the binary.
	if m.running {
		m = m.resetIdleState()
	}
	if msg.err != nil {
		m.err = msg.err.Error()
		m.settingsNote = ""
		return m
	}
	m.updateInfo = update.Info{}
	m.err = ""
	m.settingsNote = fmt.Sprintf("已更新到 v%s，请重启程序", msg.version)
	return m
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
	m.updateInfo = update.Info{}
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
	body.WriteString(sectionTitle.Render("设置") + "\n")
	body.WriteString(dividerStyle.Render(strings.Repeat("─", innerW)) + "\n\n")

	body.WriteString(textSubtle.Render("版本") + "\n")
	body.WriteString(modeActive.Render(truncate("当前 "+version.Version, innerW)) + "\n")
	if m.updateInfo.Latest != "" {
		line := "最新 " + m.updateInfo.Latest
		if m.updateInfo.Newer {
			line += " · 可更新"
		}
		body.WriteString(textSubtle.Render(truncate(line, innerW)) + "\n")
	}
	body.WriteString("\n")

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
	body.WriteString(footerKey.Render("C") + footerLabel.Render(" 检查更新"))
	body.WriteString(footerSep.Render("  "))
	body.WriteString(footerKey.Render("R") + footerLabel.Render(" 更新"))
	body.WriteString(footerSep.Render("  "))
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
