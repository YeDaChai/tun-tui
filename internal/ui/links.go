package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tun-tui/internal/config"
)

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
		if len(m.linkURLs) == 0 || m.work.busy() {
			return m, nil
		}
		cmds := []tea.Cmd{m.deleteLink(m.linkCursor)}
		if m.running && m.linkCursor == m.linkActive {
			m = m.beginNodesLoad()
		} else {
			m.work = workActing
		}
		return m, tea.Batch(cmds...)
	case "enter":
		if len(m.linkURLs) == 0 {
			m.linkInputFocus = true
			m.linkInput.Focus()
			return m, textinput.Blink
		}
		if m.work.busy() {
			return m, nil
		}
		cmds := []tea.Cmd{m.selectLink(m.linkCursor)}
		if m.running {
			m = m.beginNodesLoad()
		} else {
			m.work = workActing
		}
		return m.closeLinkScreen(), tea.Batch(cmds...)
	case "ctrl+c":
		m.stopDelayTest()
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
		if url == "" || m.work.busy() {
			return m, nil
		}
		m.linkInput.SetValue("")
		m.linkInputFocus = false
		m.linkInput.Blur()
		cmds := []tea.Cmd{m.addLink(url)}
		if m.running {
			m = m.beginNodesLoad()
		} else {
			m.work = workActing
		}
		return m, tea.Batch(cmds...)
	case "ctrl+c":
		m.stopDelayTest()
		_ = m.runner.Stop()
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.linkInput, cmd = m.linkInput.Update(msg)
	return m, cmd
}

// reloadActiveSubscription reloads the kernel after the active subscription changes.
func (m Model) reloadActiveSubscription(base actionMsg) tea.Msg {
	if !m.running {
		return base
	}
	if err := config.ClearProviderCache(m.paths.DataDir); err != nil {
		base.err = err
		return base
	}
	if err := reloadAndSyncMode(m.runner, m.api, m.paths.DataDir); err != nil {
		base.err = err
		return base
	}
	if err := m.api.UpdateProvider(config.ProviderName); err != nil {
		base.err = err
		return base
	}
	base.refresh, base.autoTest = true, true
	return base
}

func (m Model) selectLink(index int) tea.Cmd {
	return func() tea.Msg {
		if err := config.SetActiveSubscriptionLink(m.paths.DataDir, index); err != nil {
			return actionMsg{err: err}
		}
		return m.reloadActiveSubscription(actionMsg{})
	}
}

func (m Model) addLink(url string) tea.Cmd {
	return func() tea.Msg {
		base := actionMsg{added: true}
		if err := config.AddSubscriptionLink(m.paths.DataDir, url); err != nil {
			return actionMsg{err: err}
		}
		return m.reloadActiveSubscription(base)
	}
}

func (m Model) deleteLink(index int) tea.Cmd {
	return func() tea.Msg {
		base := actionMsg{deleted: true}
		wasActive := index == m.linkActive
		if err := config.DeleteSubscriptionLink(m.paths.DataDir, index); err != nil {
			return actionMsg{err: err}
		}
		urls, _, err := config.LoadSubscriptionLinks(m.paths.DataDir)
		if err != nil {
			base.err = err
			return base
		}
		if !wasActive || len(urls) == 0 {
			return base
		}
		return m.reloadActiveSubscription(base)
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

func (m Model) renderLinkListBox() string {
	modalW := m.modalWidth(64)
	innerW := modalW - 4
	if innerW < 20 {
		innerW = 20
	}

	var body strings.Builder
	body.WriteString(sectionTitle.Render(scrollPlaque("订阅")) + "\n")
	body.WriteString(mistSep(innerW) + "\n")

	if len(m.linkURLs) == 0 {
		body.WriteString(textSubtle.Render("暂无链接 — 按 I 添加") + "\n")
	} else {
		vp := m.linkViewport()
		for i := m.linkRowOffset; i < vp.end; i++ {
			mark := "  "
			if i == m.linkCursor {
				mark = "> "
			} else if i == m.linkActive {
				mark = "* "
			}
			style, full := itemNormal, false
			if i == m.linkCursor {
				style, full = itemSelected, true
			}
			item := buildRow(innerW, mark, maskURL(m.linkURLs[i]), "", style, itemNormal, full, i == m.linkActive, false, 0)
			body.WriteString(item + "\n")
		}
	}

	body.WriteString(mistSep(innerW) + "\n")
	if m.err != "" {
		body.WriteString(textErr.Render("! "+truncate(m.err, innerW-2)) + "\n")
	}
	body.WriteString(antiqueButton("↵", "使用"))
	body.WriteString(footerSep.Render(" "))
	body.WriteString(antiqueButton("I", "添加"))
	body.WriteString(footerSep.Render(" "))
	body.WriteString(antiqueButton("D", "删除"))
	body.WriteString(footerSep.Render(" "))
	body.WriteString(antiqueButton("ESC", "关闭"))

	return modalBox(modalW, strings.TrimRight(body.String(), "\n"))
}

func (m Model) renderAddLinkBox() string {
	modalW := m.modalWidth(64)
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
	body.WriteString(sectionTitle.Render(scrollPlaque("添加订阅")) + "\n\n")
	body.WriteString(ti.View() + "\n")
	if m.err != "" {
		body.WriteString("\n" + textErr.Render("! "+truncate(m.err, innerW-2)) + "\n")
	}
	body.WriteString("\n")
	body.WriteString(antiqueButton("↵", "确认"))
	body.WriteString(footerSep.Render(" "))
	body.WriteString(antiqueButton("ESC", "取消"))

	return modalBox(modalW, strings.TrimRight(body.String(), "\n"))
}
