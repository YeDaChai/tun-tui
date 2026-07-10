package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"tun-tui/internal/config"
)

func (m Model) openLinkScreen() Model {
	urls, active, _ := config.LoadSubscriptionLinks(m.paths.DataDir)
	m.screen = screenLinkList
	m.linkURLs = urls
	m.linkActive = active
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
	m.err = ""
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
		if len(m.linkURLs) == 0 {
			return m, nil
		}
		return m, m.deleteLink(m.linkCursor)

	case "enter":
		if len(m.linkURLs) == 0 {
			m.linkInputFocus = true
			m.linkInput.Focus()
			return m, textinput.Blink
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
		return m, nil

	case "enter":
		url := strings.TrimSpace(m.linkInput.Value())
		if url == "" {
			return m, nil
		}
		m.linkInput.SetValue("")
		m.linkInputFocus = false
		m.linkInput.Blur()
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

		msg := actionMsg{status: "LINK APPLIED", refresh: m.running}
		if !m.running {
			msg.status = "LINK SELECTED"
			msg.refresh = false
			return msg
		}

		if err := m.runner.Reload(); err != nil {
			msg.err = err
			msg.status = "RELOAD FAILED"
			msg.refresh = false
			return msg
		}
		mode := config.LoadMode(m.paths.DataDir, "rule")
		if err := m.api.PatchMode(mode); err != nil {
			msg.err = err
			msg.status = "MODE SYNC FAILED"
		}
		return msg
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

		if err := m.runner.Reload(); err != nil {
			return linkMsg{added: true, err: err, status: "RELOAD FAILED"}
		}
		mode := config.LoadMode(m.paths.DataDir, "rule")
		if err := m.api.PatchMode(mode); err != nil {
			return linkMsg{added: true, err: err, status: "MODE SYNC FAILED"}
		}
		return linkMsg{added: true, status: "LINK ADDED & APPLIED", refresh: true}
	}
}

func (m Model) deleteLink(index int) tea.Cmd {
	return func() tea.Msg {
		wasActive := index == m.linkActive
		if err := config.DeleteSubscriptionLink(m.paths.DataDir, index); err != nil {
			return linkMsg{err: err}
		}

		urls, _, _ := config.LoadSubscriptionLinks(m.paths.DataDir)
		if !m.running || !wasActive || len(urls) == 0 {
			return linkMsg{deleted: true, status: "LINK DELETED"}
		}

		if err := m.runner.Reload(); err != nil {
			return linkMsg{deleted: true, err: err, status: "RELOAD FAILED"}
		}
		mode := config.LoadMode(m.paths.DataDir, "rule")
		if err := m.api.PatchMode(mode); err != nil {
			return linkMsg{deleted: true, err: err, status: "MODE SYNC FAILED"}
		}
		return linkMsg{deleted: true, status: "LINK DELETED & APPLIED", refresh: true}
	}
}

type linkMsg struct {
	added   bool
	deleted bool
	err     error
	status  string
	refresh bool
}

func (m *Model) moveLinkCursor(delta int) {
	if len(m.linkURLs) == 0 {
		return
	}
	m.linkCursor += delta
	if m.linkCursor < 0 {
		m.linkCursor = 0
	}
	if m.linkCursor >= len(m.linkURLs) {
		m.linkCursor = len(m.linkURLs) - 1
	}
	m.clampLinkScroll()
}

func (m *Model) scrollLinkRows(delta int) {
	if len(m.linkURLs) == 0 {
		return
	}
	vp := m.linkViewport()
	m.linkRowOffset += delta
	if m.linkRowOffset < 0 {
		m.linkRowOffset = 0
	}
	maxOffset := len(m.linkURLs) - vp.visibleRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.linkRowOffset > maxOffset {
		m.linkRowOffset = maxOffset
	}
}

func (m *Model) clampLinkScroll() {
	if len(m.linkURLs) == 0 {
		m.linkCursor = 0
		m.linkRowOffset = 0
		return
	}
	if m.linkCursor >= len(m.linkURLs) {
		m.linkCursor = len(m.linkURLs) - 1
	}
	if m.linkCursor < 0 {
		m.linkCursor = 0
	}
	vp := m.linkViewport()
	if m.linkCursor < m.linkRowOffset {
		m.linkRowOffset = m.linkCursor
	}
	if m.linkCursor >= m.linkRowOffset+vp.visibleRows {
		m.linkRowOffset = m.linkCursor - vp.visibleRows + 1
	}
	maxOffset := len(m.linkURLs) - vp.visibleRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.linkRowOffset > maxOffset {
		m.linkRowOffset = maxOffset
	}
}

func (m Model) linkListBudget() int {
	if m.height <= 0 {
		return 6
	}
	// title(1) + divider(1) + blank(2) + input box(3) + hints(2) + keys(1)
	used := 10
	budget := m.height - used
	if budget < 2 {
		return 2
	}
	return budget
}

func (m Model) linkViewport() struct {
	visibleRows   int
	showUpArrow   bool
	showDownArrow bool
	endIdx        int
} {
	budget := m.linkListBudget()
	showUp := m.linkRowOffset > 0
	arrows := 0
	if showUp {
		arrows++
	}
	maxVisible := budget - arrows
	if maxVisible < 1 {
		maxVisible = 1
	}
	showDown := m.linkRowOffset+maxVisible < len(m.linkURLs)
	if showDown {
		arrows++
	}
	visible := budget - arrows
	if visible < 1 {
		visible = 1
	}
	endIdx := m.linkRowOffset + visible
	if endIdx > len(m.linkURLs) {
		endIdx = len(m.linkURLs)
	}
	return struct {
		visibleRows   int
		showUpArrow   bool
		showDownArrow bool
		endIdx        int
	}{visible, showUp, showDown, endIdx}
}

func (m Model) viewLinkScreen() string {
	w := m.contentWidth()
	if w < 40 {
		w = 40
	}
	f := newPanelFrame(w, true)
	inner := frameInner(w)

	var b strings.Builder
	b.WriteString(f.top())
	b.WriteString("\n")
	b.WriteString(f.rowSpacedLeft(sectionTitle.Render(" SUBSCRIPTION LINKS ")))
	b.WriteString("\n")
	b.WriteString(f.rowSpacedLeft(dividerStyle.Render(strings.Repeat("─", inner))))
	b.WriteString("\n")

	if len(m.linkURLs) == 0 {
		b.WriteString(f.rowSpacedLeft(textSubtle.Render("  No links — press [i] to add")))
		b.WriteString("\n")
	} else {
		vp := m.linkViewport()

		if vp.showUpArrow {
			hint := fmt.Sprintf("  △  %d more above", m.linkRowOffset)
			b.WriteString(f.rowSpacedLeft(padVisual(textSubtle.Render(hint), inner)))
			b.WriteString("\n")
		}

		for i := m.linkRowOffset; i < vp.endIdx; i++ {
			mark := "  "
			if i == m.linkActive {
				mark = "◆ "
			}
			rowStyle := itemNormal
			fullRow := false
			if i == m.linkCursor {
				rowStyle = itemSelected
				fullRow = true
			}
			item := buildRow(inner, mark, maskURL(m.linkURLs[i]), "", rowStyle, itemNormal, fullRow)
			b.WriteString(f.rowSpacedLeft(padVisual(item, inner)))
			b.WriteString("\n")
		}

		if vp.showDownArrow {
			remaining := len(m.linkURLs) - vp.endIdx
			hint := fmt.Sprintf("  ▽  %d more below", remaining)
			b.WriteString(f.rowSpacedLeft(padVisual(textSubtle.Render(hint), inner)))
			b.WriteString("\n")
		}
	}

	b.WriteString(f.bottom())
	b.WriteString("\n\n")

	if m.linkInputFocus {
		b.WriteString(textSubtle.Render("  Add subscription link:"))
	} else {
		b.WriteString(textSubtle.Render("  Press [i] to add, [d] to delete selected"))
	}
	b.WriteString("\n")
	b.WriteString(inputPanel.Render(m.linkInput.View()))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(textErr.Render("⚠ " + m.err))
		b.WriteString("\n\n")
	}

	b.WriteString(footerBracket.Render("[") + footerKey.Render("↵") + footerBracket.Render("]") + footerLabel.Render(" USE"))
	b.WriteString(footerSep.Render("  ·  "))
	b.WriteString(footerBracket.Render("[") + footerKey.Render("I") + footerBracket.Render("]") + footerLabel.Render(" ADD"))
	b.WriteString(footerSep.Render("  ·  "))
	b.WriteString(footerBracket.Render("[") + footerKey.Render("D") + footerBracket.Render("]") + footerLabel.Render(" DEL"))
	b.WriteString(footerSep.Render("  ·  "))
	b.WriteString(footerBracket.Render("[") + footerKey.Render("ESC") + footerBracket.Render("]") + footerLabel.Render(" CLOSE"))
	b.WriteString("\n")

	return b.String()
}
