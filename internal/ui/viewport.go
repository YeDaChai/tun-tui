package ui

import "strings"

type viewport struct {
	visibleRows   int
	showUpArrow   bool
	showDownArrow bool
	endIdx        int
}

func computeViewport(total, offset, budget int) viewport {
	if budget < 1 {
		budget = 1
	}
	showUp := offset > 0
	arrows := 0
	if showUp {
		arrows++
	}
	maxVisible := budget - arrows
	if maxVisible < 1 {
		maxVisible = 1
	}
	showDown := offset+maxVisible < total
	if showDown {
		arrows++
	}
	visible := budget - arrows
	if visible < 1 {
		visible = 1
	}
	endIdx := offset + visible
	if endIdx > total {
		endIdx = total
	}
	return viewport{
		visibleRows:   visible,
		showUpArrow:   showUp,
		showDownArrow: showDown,
		endIdx:        endIdx,
	}
}

func clampScroll(total, cursor, offset, visibleRows int) (newCursor, newOffset int) {
	if total == 0 {
		return 0, 0
	}
	if cursor >= total {
		cursor = total - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+visibleRows {
		offset = cursor - visibleRows + 1
	}
	maxOffset := total - visibleRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	return cursor, offset
}

func (m Model) listBudget() int {
	if m.height <= 0 {
		return 8
	}
	// Estimate without rendering: HUD + blank + panel chrome + footer.
	used := m.hudLineEstimate() + 5 + m.footerLineEstimate()
	budget := m.height - used
	if budget < 1 {
		return 1
	}
	return budget
}

func (m Model) hudLineEstimate() int {
	n := 4 // top + connection + traffic + bottom
	if strings.TrimSpace(m.status) != "" {
		n++
	}
	if info := m.provider.SubscriptionInfo; info != nil && info.Total > 0 {
		n++
	}
	if m.err != "" {
		n++
	}
	return n
}

func (m Model) footerLineEstimate() int {
	return 3 // blank + divider + keys
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

func (m Model) listViewport() viewport {
	return computeViewport(len(m.nodes), m.rowOffset, m.listBudget())
}

func (m Model) linkViewport() viewport {
	return computeViewport(len(m.linkURLs), m.linkRowOffset, m.linkListBudget())
}

func (m *Model) moveCursor(delta int) {
	if len(m.nodes) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.nodes) {
		m.cursor = len(m.nodes) - 1
	}
	m.clampListScroll()
}

func (m *Model) clampListScroll() {
	vp := m.listViewport()
	m.cursor, m.rowOffset = clampScroll(len(m.nodes), m.cursor, m.rowOffset, vp.visibleRows)
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

func (m *Model) clampLinkScroll() {
	vp := m.linkViewport()
	m.linkCursor, m.linkRowOffset = clampScroll(len(m.linkURLs), m.linkCursor, m.linkRowOffset, vp.visibleRows)
}
