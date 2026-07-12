package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tun-tui/internal/api"
	"tun-tui/internal/config"
)

type linkMsg struct {
	added    bool
	deleted  bool
	err      error
	refresh  bool
	autoTest bool
}

func fetchProviderNodes(client *api.Client) error {
	return client.UpdateProvider(config.ProviderName)
}

func (m Model) openLinkScreen() Model {
	urls, active, err := config.LoadSubscriptionLinks(m.paths.DataDir)
	m.screen = screenLinkList
	m.linkURLs, m.linkActive = urls, active
	m.linkCursor = 0
	if active >= 0 && active < len(urls) {
		m.linkCursor = active
	}
	m.linkRowOffset = 0
	m.linkInput.SetValue("")
	m.linkInputFocus = len(urls) == 0
	if m.linkInputFocus {
		m.linkInput.Focus()
	} else {
		m.linkInput.Blur()
	}
	m.clampLinkScroll()
	if err != nil {
		m.err = err.Error()
	} else {
		m.err = ""
	}
	return m
}

func (m Model) closeLinkScreen() Model {
	m.screen = screenMain
	m.linkInput.Blur()
	m.linkInputFocus = false
	return m
}

func (m Model) updateLinkScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.linkInputFocus {
		return m.updateLinkInput(msg)
	}
	return m.updateLinkList(msg)
}

func (m Model) updateLinkList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "l":
		return m.closeLinkScreen(), nil
	case "i", "a":
		m.linkInputFocus = true
		m.linkInput.Focus()
		m.err = ""
		return m, textinput.Blink
	case "k", "up":
		m.moveLinkCursor(-1)
		return m, nil
	case "j", "down":
		m.moveLinkCursor(1)
		return m, nil
	case "d":
		if len(m.linkURLs) == 0 || m.busy {
			return m, nil
		}
		if m.running && m.linkCursor == m.linkActive {
			m = m.beginNodesLoad()
		} else {
			m.busy = true
		}
		return m, m.deleteLink(m.linkCursor)
	case "enter":
		if len(m.linkURLs) == 0 {
			m.linkInputFocus = true
			m.linkInput.Focus()
			return m, textinput.Blink
		}
		if m.busy {
			return m, nil
		}
		if m.running {
			m = m.beginNodesLoad()
		} else {
			m.busy = true
		}
		return m.closeLinkScreen(), m.selectLink(m.linkCursor)
	case "ctrl+c":
		_ = m.runner.Stop()
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateLinkInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if len(m.linkURLs) == 0 {
			return m.closeLinkScreen(), nil
		}
		m.linkInputFocus = false
		m.linkInput.Blur()
		m.linkInput.SetValue("")
		m.err = ""
		return m, nil
	case "enter":
		url := strings.TrimSpace(m.linkInput.Value())
		if url == "" || m.busy {
			return m, nil
		}
		m.linkInput.SetValue("")
		m.linkInputFocus = false
		m.linkInput.Blur()
		if m.running {
			m = m.beginNodesLoad()
		} else {
			m.busy = true
		}
		return m, m.addLink(url)
	case "ctrl+c":
		_ = m.runner.Stop()
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.linkInput, cmd = m.linkInput.Update(msg)
	return m, cmd
}

func (m Model) selectLink(index int) tea.Cmd {
	return func() tea.Msg {
		if err := config.SetActiveSubscriptionLink(m.paths.DataDir, index); err != nil {
			return actionMsg{err: err}
		}
		if !m.running {
			return actionMsg{}
		}
		if err := reloadAndSyncMode(m.runner, m.api, m.paths.DataDir); err != nil {
			return actionMsg{err: err}
		}
		if err := fetchProviderNodes(m.api); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{refresh: true, autoTest: true}
	}
}

func (m Model) addLink(url string) tea.Cmd {
	return func() tea.Msg {
		if err := config.AddSubscriptionLink(m.paths.DataDir, url); err != nil {
			return linkMsg{err: err}
		}
		if !m.running {
			return linkMsg{added: true}
		}
		if err := reloadAndSyncMode(m.runner, m.api, m.paths.DataDir); err != nil {
			return linkMsg{added: true, err: err}
		}
		if err := fetchProviderNodes(m.api); err != nil {
			return linkMsg{added: true, err: err}
		}
		return linkMsg{added: true, refresh: true, autoTest: true}
	}
}

func (m Model) deleteLink(index int) tea.Cmd {
	return func() tea.Msg {
		wasActive := index == m.linkActive
		if err := config.DeleteSubscriptionLink(m.paths.DataDir, index); err != nil {
			return linkMsg{err: err}
		}
		urls, _, err := config.LoadSubscriptionLinks(m.paths.DataDir)
		if err != nil {
			return linkMsg{deleted: true, err: err}
		}
		if !m.running || !wasActive || len(urls) == 0 {
			return linkMsg{deleted: true}
		}
		if err := reloadAndSyncMode(m.runner, m.api, m.paths.DataDir); err != nil {
			return linkMsg{deleted: true, err: err}
		}
		if err := fetchProviderNodes(m.api); err != nil {
			return linkMsg{deleted: true, err: err}
		}
		return linkMsg{deleted: true, refresh: true, autoTest: true}
	}
}

func (m Model) viewLinkScreen() string {
	w := m.width
	if w <= 0 {
		w = m.contentWidth()
	}
	h := m.height
	if h <= 0 {
		h = 24
	}
	bg := m.viewMain()
	if m.linkInputFocus {
		return overlayCenter(bg, m.renderAddLinkBox(), w, h)
	}
	return overlayCenter(bg, m.renderLinkListBox(), w, h)
}

func (m Model) linkModalWidth() int {
	w := m.contentWidth() - 8
	if w > 64 {
		w = 64
	}
	if w < 36 {
		w = m.contentWidth()
		if w > 36 {
			w = 36
		}
	}
	if w < 24 {
		w = 24
	}
	return w
}

func (m Model) renderLinkListBox() string {
	modalW := m.linkModalWidth()
	innerW := modalW - 4
	if innerW < 20 {
		innerW = 20
	}

	var body strings.Builder
	body.WriteString(sectionTitle.Render("订阅链接") + "\n")
	body.WriteString(dividerStyle.Render(strings.Repeat("─", innerW)) + "\n")

	if len(m.linkURLs) == 0 {
		body.WriteString(textSubtle.Render("暂无链接 — 按 I 添加") + "\n")
	} else {
		vp := m.linkViewport()
		for i := m.linkRowOffset; i < vp.end; i++ {
			mark := "  "
			if i == m.linkCursor {
				mark = "› "
			} else if i == m.linkActive {
				mark = "● "
			}
			style, full := itemNormal, false
			if i == m.linkCursor {
				style, full = itemSelected, true
			}
			item := buildRow(innerW, mark, maskURL(m.linkURLs[i]), "", style, itemNormal, full)
			body.WriteString(item + "\n")
		}
	}

	body.WriteString(dividerStyle.Render(strings.Repeat("─", innerW)) + "\n")
	if m.err != "" {
		body.WriteString(textErr.Render("! "+truncate(m.err, innerW-2)) + "\n")
	}
	body.WriteString(footerKey.Render("↵") + footerLabel.Render(" 使用"))
	body.WriteString(footerSep.Render("  "))
	body.WriteString(footerKey.Render("I") + footerLabel.Render(" 添加"))
	body.WriteString(footerSep.Render("  "))
	body.WriteString(footerKey.Render("D") + footerLabel.Render(" 删除"))
	body.WriteString(footerSep.Render("  "))
	body.WriteString(footerKey.Render("ESC") + footerLabel.Render(" 关闭"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalW).
		Render(strings.TrimRight(body.String(), "\n"))
}

func (m Model) renderAddLinkBox() string {
	modalW := m.linkModalWidth()
	innerW := modalW - 4
	if innerW < 20 {
		innerW = 20
	}

	ti := m.linkInput
	ti.Width = innerW - lipgloss.Width(ti.Prompt)
	if ti.Width < 12 {
		ti.Width = 12
	}

	var body strings.Builder
	body.WriteString(sectionTitle.Render("添加订阅链接") + "\n\n")
	body.WriteString(ti.View() + "\n")
	if m.err != "" {
		body.WriteString("\n" + textErr.Render("! "+truncate(m.err, innerW-2)) + "\n")
	}
	body.WriteString("\n")
	body.WriteString(footerKey.Render("↵") + footerLabel.Render(" 确认"))
	body.WriteString(footerSep.Render("  "))
	body.WriteString(footerKey.Render("ESC") + footerLabel.Render(" 取消"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalW).
		Render(strings.TrimRight(body.String(), "\n"))
}
