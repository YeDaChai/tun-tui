package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"tun-tui/internal/config"
	"tun-tui/internal/geodata"
)

func (m Model) View() string {
	if m.screen == screenLinkList {
		return m.viewLinkScreen()
	}
	return m.viewMain()
}

func (m Model) viewMain() string {
	var b strings.Builder

	b.WriteString(m.renderHUD())
	b.WriteString("\n")
	b.WriteString(m.renderProxyPanel())
	b.WriteString(m.renderFooter())

	if m.height > 0 {
		if total := lineCount(b.String()); total < m.height {
			b.WriteString(strings.Repeat("\n", m.height-total))
		}
	}

	return b.String()
}

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

func (m Model) renderHUD() string {
	w := m.contentWidth()
	f := newFrame(w)
	var b strings.Builder

	b.WriteString(f.top())
	b.WriteString("\n")
	b.WriteString(f.rowSpacedSplit(m.connectionLine(), m.metaLine()))
	b.WriteString("\n")
	b.WriteString(f.rowSpacedLeft(m.trafficGauge(frameInner(w))))
	b.WriteString("\n")

	if strings.TrimSpace(m.status) != "" {
		b.WriteString(f.rowSpacedLeft(m.statusLine(frameInner(w))))
		b.WriteString("\n")
	}

	if info := m.provider.SubscriptionInfo; info != nil && info.Total > 0 {
		used := info.Upload + info.Download
		barLine := renderUsageBar(used, info.Total, frameInner(w)-2)
		b.WriteString(f.rowSpacedLeft(" " + barLine))
		b.WriteString("\n")
	}

	if m.err != "" {
		errLine := textErr.Render("! " + truncateVisual(m.err, frameInner(w)-2))
		b.WriteString(f.rowSpacedLeft(errLine))
		b.WriteString("\n")
	}

	b.WriteString(f.bottom())
	b.WriteString("\n")

	return b.String()
}

func (m Model) connectionLine() string {
	if m.running {
		return statusOnline.Render("● 已连接")
	} else if m.starting {
		return statusLoading.Render("… 连接中")
	}
	return statusOffline.Render("○ 未连接")
}

func (m Model) metaLine() string {
	var parts []string
	if m.appVersion != "" {
		parts = append(parts, textSubtle.Render("tun-tui")+textBody.Render(" "+m.appVersion))
	}
	if m.version != "" {
		parts = append(parts, textSubtle.Render("mihomo")+textBody.Render(" "+m.version))
	}
	if m.mode != "" {
		parts = append(parts, textSubtle.Render("模式: ")+modeActive.Render(modeLabel(m.mode)))
	}
	parts = append(parts, m.rulesStatus())
	return strings.Join(parts, "  ")
}

func (m Model) rulesStatus() string {
	if !m.running {
		return rulesOff.Render("规则 --")
	}
	ready := geodata.Ready(m.paths.DataDir)
	switch config.NormalizeMode(m.mode) {
	case "rule":
		if ready {
			return rulesOn.Render("规则 开")
		}
		return rulesBad.Render("规则缺数据")
	case "global", "direct":
		return rulesOff.Render("规则 关")
	default:
		if ready {
			return rulesOn.Render("规则 开")
		}
		return rulesOff.Render("规则 --")
	}
}

func (m Model) trafficGauge(width int) string {
	if !m.running {
		return textSubtle.Render("↑  --  ↓  --")
	}

	up := formatRate(m.traffic.Up)
	down := formatRate(m.traffic.Down)

	upBar := miniSpeedBar(m.traffic.Up, 6)
	downBar := miniSpeedBar(m.traffic.Down, 6)

	return txColor.Render("↑") + " " + up + " " + upBar +
		textSubtle.Render("  ") +
		rxColor.Render("↓") + " " + down + " " + downBar
}

func miniSpeedBar(rate int64, width int) string {
	const max = 1 << 20 // 1 MB/s
	ratio := float64(rate) / float64(max)
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	if filled < 0 {
		filled = 0
	}

	var style lipgloss.Style
	switch {
	case ratio > 0.8:
		style = barDanger
	case ratio > 0.4:
		style = barWarning
	default:
		style = barFull
	}

	s := style.Render(strings.Repeat("▆", filled))
	s += barEmpty.Render(strings.Repeat("▆", width-filled))
	return s
}

func (m Model) statusLine(width int) string {
	return textMuted.Render(truncateVisual(m.status, width))
}

func (m Model) renderProxyPanel() string {
	w := m.contentWidth()
	active := m.running && m.activeGroup != ""
	f := newPanelFrame(w, active)
	inner := frameInner(w)
	if inner < 16 {
		inner = 16
	}

	var b strings.Builder
	b.WriteString(f.top())
	b.WriteString("\n")

	title := " 节点 "
	if len(m.nodes) > 0 {
		title = fmt.Sprintf(" 节点 [%d/%d] ", m.cursor+1, len(m.nodes))
	}
	if m.group.Now != "" {
		title += "· " + m.group.Now + " "
	}
	b.WriteString(f.rowSpacedLeft(sectionTitle.Render(truncateVisual(title, inner))))
	b.WriteString("\n")
	b.WriteString(f.rowSpacedLeft(dividerStyle.Render(strings.Repeat("─", inner))))
	b.WriteString("\n")

	for _, line := range strings.Split(m.renderNodeList(inner), "\n") {
		if line == "" {
			continue
		}
		b.WriteString(f.rowSpacedLeft(line))
		b.WriteString("\n")
	}

	b.WriteString(f.bottom())
	return b.String()
}

func (m Model) renderNodeList(innerW int) string {
	vp := m.listViewport()
	total := len(m.nodes)

	var lines []string

	if total == 0 {
		lines = append(lines, padVisual(textSubtle.Render("  "+m.emptyHint()), innerW))
	} else {
		if vp.showUpArrow {
			hint := fmt.Sprintf("  △  上方还有 %d 个", m.rowOffset)
			lines = append(lines, padVisual(textSubtle.Render(hint), innerW))
		}

		for i := m.rowOffset; i < vp.endIdx; i++ {
			lines = append(lines, padVisual(m.formatListItem(i, innerW), innerW))
		}

		if vp.showDownArrow {
			remaining := total - vp.endIdx
			hint := fmt.Sprintf("  ▽  下方还有 %d 个", remaining)
			lines = append(lines, padVisual(textSubtle.Render(hint), innerW))
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) emptyHint() string {
	switch {
	case m.starting:
		return "正在建立连接…"
	case m.running:
		return "加载节点中…"
	case m.hasSubscription:
		return "等待连接…"
	default:
		return "按 l 添加订阅链接"
	}
}

func (m Model) formatListItem(idx, width int) string {
	node := m.nodes[idx]
	active := idx == m.cursor
	current := node == m.group.Now

	mark := "  "
	if active {
		mark = "› "
	} else if current {
		mark = "● "
	}

	delayStr := ""
	var delayStyle lipgloss.Style
	if d, ok := m.delays[node]; ok {
		if d > 0 {
			delayStr = fmt.Sprintf("%dms", d)
			switch {
			case d < 150:
				delayStyle = pingFast
			case d < 400:
				delayStyle = pingMid
			default:
				delayStyle = pingSlow
			}
		} else {
			delayStr = "超时"
			delayStyle = pingDead
		}
	}

	var itemStyle lipgloss.Style
	switch {
	case active:
		itemStyle = itemSelected
	case current:
		itemStyle = itemCurrent
	default:
		itemStyle = itemNormal
	}

	return buildRow(width, mark, node, delayStr, itemStyle, delayStyle, active)
}

func (m Model) renderFooter() string {
	w := m.contentWidth()
	f := newFrame(w)
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(f.bottom())
	b.WriteString("\n ")

	keys := [][2]string{
		{"l", "链接"},
		{"s", "连接"},
		{"m", "模式"},
		{"↑↓", "选择"},
		{"↵", "确认"},
		{"u", "更新"},
		{"t", "测速"},
		{"r", "重载"},
		{"q", "退出"},
	}

	inner := frameInner(w)
	var parts []string
	for _, k := range keys {
		parts = append(parts, footerKey.Render(k[0])+footerLabel.Render(" "+k[1]))
	}

	full := strings.Join(parts, footerSep.Render("  "))
	if visualWidth(full) > inner {
		parts = nil
		for _, k := range keys {
			parts = append(parts, footerKey.Render(k[0])+footerLabel.Render(k[1]))
		}
		full = strings.Join(parts, footerSep.Render(" "))
	}

	b.WriteString(truncateVisual(full, inner))

	return b.String()
}
