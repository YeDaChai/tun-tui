package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"tun-tui/internal/config"
)

type linkMsg struct {
	added   bool
	deleted bool
	err     error
	refresh bool
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
		m.busy = true
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
		m.busy = true
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
		return m, nil
	case "enter":
		url := strings.TrimSpace(m.linkInput.Value())
		if url == "" || m.busy {
			return m, nil
		}
		m.linkInput.SetValue("")
		m.linkInputFocus = false
		m.linkInput.Blur()
		m.busy = true
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
		return actionMsg{refresh: true}
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
		return linkMsg{added: true, refresh: true}
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
		return linkMsg{deleted: true, refresh: true}
	}
}

func (m Model) viewLinkScreen() string {
	w := m.contentWidth()
	if w < 40 {
		w = 40
	}
	f := newFrame(w, true)
	var b strings.Builder
	b.WriteString(f.top() + "\n")
	b.WriteString(f.row(sectionTitle.Render(" 订阅链接 ")) + "\n")
	b.WriteString(f.row(dividerStyle.Render(strings.Repeat("─", w))) + "\n")

	if len(m.linkURLs) == 0 {
		b.WriteString(f.row(textSubtle.Render("  暂无链接 — 按 i 添加")) + "\n")
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
			item := buildRow(w, mark, maskURL(m.linkURLs[i]), "", style, itemNormal, full)
			b.WriteString(f.row(pad(item, w)) + "\n")
		}
	}

	b.WriteString(f.bottom() + "\n\n")
	if m.linkInputFocus {
		b.WriteString(textSubtle.Render("  添加订阅链接:") + "\n")
	} else {
		b.WriteString(textSubtle.Render("  按 i 添加，按 d 删除选中") + "\n")
	}
	b.WriteString(inputPanel.Render(m.linkInput.View()) + "\n\n")
	if m.err != "" {
		b.WriteString(textErr.Render("! "+m.err) + "\n\n")
	}
	b.WriteString(footerKey.Render("↵") + footerLabel.Render(" 使用"))
	b.WriteString(footerSep.Render("  "))
	b.WriteString(footerKey.Render("i") + footerLabel.Render(" 添加"))
	b.WriteString(footerSep.Render("  "))
	b.WriteString(footerKey.Render("d") + footerLabel.Render(" 删除"))
	b.WriteString(footerSep.Render("  "))
	b.WriteString(footerKey.Render("esc") + footerLabel.Render(" 关闭") + "\n")
	return b.String()
}
