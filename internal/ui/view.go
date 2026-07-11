package ui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"tun-tui/internal/config"
	"tun-tui/internal/geodata"
)

var (
	accent = lipgloss.Color("39")
	ok     = lipgloss.Color("71")
	warn   = lipgloss.Color("178")
	danger = lipgloss.Color("167")
	fg     = lipgloss.Color("252")
	muted  = lipgloss.Color("245")
	subtle = lipgloss.Color("240")
	selBg  = lipgloss.Color("236")

	frameBorderActive = lipgloss.NewStyle().Foreground(accent)
	statusOnline        = lipgloss.NewStyle().Foreground(ok).Bold(true)
	statusOffline       = lipgloss.NewStyle().Foreground(muted)
	statusLoading       = lipgloss.NewStyle().Foreground(warn).Bold(true)
	textErr             = lipgloss.NewStyle().Foreground(danger)
	textMuted           = lipgloss.NewStyle().Foreground(muted)
	textSubtle          = lipgloss.NewStyle().Foreground(subtle)
	textBody            = lipgloss.NewStyle().Foreground(fg)
	inputPanel          = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(accent).BorderTop(true).BorderBottom(true).BorderLeft(false).BorderRight(false).Padding(1, 0)
	itemSelected        = lipgloss.NewStyle().Foreground(accent).Background(selBg).Bold(true)
	itemCurrent         = lipgloss.NewStyle().Foreground(ok).Bold(true)
	itemNormal          = lipgloss.NewStyle().Foreground(muted)
	pingFast            = lipgloss.NewStyle().Foreground(ok)
	pingMid             = lipgloss.NewStyle().Foreground(warn)
	pingSlow            = lipgloss.NewStyle().Foreground(danger)
	pingDead            = lipgloss.NewStyle().Foreground(muted)
	txColor             = lipgloss.NewStyle().Foreground(accent)
	rxColor             = lipgloss.NewStyle().Foreground(ok)
	dividerStyle        = lipgloss.NewStyle().Foreground(subtle)
	footerKey           = lipgloss.NewStyle().Foreground(accent).Bold(true)
	footerLabel         = lipgloss.NewStyle().Foreground(muted)
	footerSep           = lipgloss.NewStyle().Foreground(subtle)
	sectionTitle        = lipgloss.NewStyle().Foreground(accent).Bold(true)
	barFull             = lipgloss.NewStyle().Foreground(ok)
	barWarning          = lipgloss.NewStyle().Foreground(warn)
	barDanger           = lipgloss.NewStyle().Foreground(danger)
	barEmpty            = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	modeActive          = lipgloss.NewStyle().Foreground(accent).Bold(true)
	rulesOn             = lipgloss.NewStyle().Foreground(ok).Bold(true)
	rulesOff            = lipgloss.NewStyle().Foreground(muted)
	rulesBad            = lipgloss.NewStyle().Foreground(warn).Bold(true)
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
	b.WriteString(m.renderProxyPanel())
	b.WriteString(m.renderFooter())
	if m.height > 0 {
		if n := lineCount(b.String()); n < m.height {
			b.WriteString(strings.Repeat("\n", m.height-n))
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
	f := newFrame(w, false)
	var b strings.Builder
	b.WriteString(f.top() + "\n")
	b.WriteString(f.split(m.connectionLine(), m.metaLine()) + "\n")
	b.WriteString(f.row(m.trafficLine()) + "\n")
	if strings.TrimSpace(m.status) != "" {
		b.WriteString(f.row(textMuted.Render(truncate(m.status, w))) + "\n")
	}
	if info := m.provider.SubscriptionInfo; info != nil && info.Total > 0 {
		b.WriteString(f.row(" "+usageBar(info.Upload+info.Download, info.Total, w-2)) + "\n")
	}
	if m.err != "" {
		b.WriteString(f.row(textErr.Render("! "+truncate(m.err, w-2))) + "\n")
	}
	b.WriteString(f.bottom() + "\n")
	return b.String()
}

func (m Model) connectionLine() string {
	switch {
	case m.running:
		return statusOnline.Render("● 已连接")
	case m.starting:
		return statusLoading.Render("… 连接中")
	default:
		return statusOffline.Render("○ 未连接")
	}
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
	if m.nodeCrypto != "" {
		parts = append(parts, textSubtle.Render("协议: ")+modeActive.Render(m.nodeCrypto))
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

func (m Model) trafficLine() string {
	if !m.running {
		return textSubtle.Render("↑  --  ↓  --")
	}
	return txColor.Render("↑") + " " + formatRate(m.traffic.Up) +
		textSubtle.Render("  ") +
		rxColor.Render("↓") + " " + formatRate(m.traffic.Down)
}

func (m Model) renderProxyPanel() string {
	w := m.contentWidth()
	if w < 16 {
		w = 16
	}
	f := newFrame(w, m.running && m.activeGroup != "")
	var b strings.Builder
	b.WriteString(f.top() + "\n")

	title := " 节点 "
	if len(m.nodes) > 0 {
		title = fmt.Sprintf(" 节点 [%d/%d] ", m.cursor+1, len(m.nodes))
	}
	if m.group.Now != "" {
		title += "· " + m.group.Now + " "
	}
	b.WriteString(f.row(sectionTitle.Render(truncate(title, w))) + "\n")
	b.WriteString(f.row(dividerStyle.Render(strings.Repeat("─", w))) + "\n")

	for _, line := range strings.Split(m.renderNodeList(w), "\n") {
		if line != "" {
			b.WriteString(f.row(line) + "\n")
		}
	}
	b.WriteString(f.bottom())
	return b.String()
}

func (m Model) renderNodeList(innerW int) string {
	vp := m.listViewport()
	if len(m.nodes) == 0 {
		return pad(textSubtle.Render("  "+m.emptyHint()), innerW)
	}
	var lines []string
	if vp.showUp {
		lines = append(lines, pad(textSubtle.Render(fmt.Sprintf("  △  上方还有 %d 个", m.rowOffset)), innerW))
	}
	for i := m.rowOffset; i < vp.end; i++ {
		lines = append(lines, pad(m.formatListItem(i, innerW), innerW))
	}
	if vp.showDown {
		lines = append(lines, pad(textSubtle.Render(fmt.Sprintf("  ▽  下方还有 %d 个", len(m.nodes)-vp.end)), innerW))
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
	active, current := idx == m.cursor, node == m.group.Now
	mark := "  "
	if active {
		mark = "› "
	} else if current {
		mark = "● "
	}

	delayStr := ""
	delayStyle := itemNormal
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
			delayStr, delayStyle = "超时", pingDead
		}
	}

	style := itemNormal
	switch {
	case active:
		style = itemSelected
	case current:
		style = itemCurrent
	}
	return buildRow(width, mark, node, delayStr, style, delayStyle, active)
}

func (m Model) renderFooter() string {
	w := m.contentWidth()
	f := newFrame(w, false)
	keys := [][2]string{
		{"l", "链接"}, {"s", "连接"}, {"m", "模式"}, {"↑↓", "选择"},
		{"↵", "确认"}, {"u", "更新"}, {"t", "测速"}, {"r", "重载"}, {"q", "退出"},
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, footerKey.Render(k[0])+footerLabel.Render(" "+k[1]))
	}
	full := strings.Join(parts, footerSep.Render("  "))
	if lipgloss.Width(full) > w {
		parts = parts[:0]
		for _, k := range keys {
			parts = append(parts, footerKey.Render(k[0])+footerLabel.Render(k[1]))
		}
		full = strings.Join(parts, footerSep.Render(" "))
	}
	return "\n" + f.bottom() + "\n " + truncate(full, w)
}

// --- layout helpers ---

type frame struct {
	width  int
	border lipgloss.Style
}

func newFrame(width int, active bool) frame {
	b := dividerStyle
	if active {
		b = frameBorderActive
	}
	return frame{width: width, border: b}
}

func (f frame) top() string    { return f.border.Render(strings.Repeat("─", f.width)) }
func (f frame) bottom() string { return f.border.Render(strings.Repeat("─", f.width)) }
func (f frame) row(s string) string {
	return pad(truncate(s, f.width), f.width)
}
func (f frame) split(left, right string) string {
	lw, rw := lipgloss.Width(left), lipgloss.Width(right)
	gap := f.width - lw - rw
	if gap < 1 {
		gap = 1
		left = truncate(left, f.width-rw-gap)
	}
	return pad(left+strings.Repeat(" ", gap)+right, f.width)
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	return ansi.Truncate(s, max, "…")
}

func pad(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

func buildRow(width int, mark, name, delay string, rowStyle, delayStyle lipgloss.Style, fullRow bool) string {
	nameMax := width - lipgloss.Width(mark)
	if delay != "" {
		nameMax -= lipgloss.Width(delay)
	}
	if nameMax < 1 {
		nameMax = 1
	}
	prefix := mark + truncate(name, nameMax)
	plain := prefix
	if delay != "" {
		gap := width - lipgloss.Width(prefix) - lipgloss.Width(delay)
		if gap < 0 {
			gap = 0
		}
		plain = prefix + strings.Repeat(" ", gap) + delay
	}
	plain = pad(plain, width)
	if fullRow || delay == "" {
		return rowStyle.Render(plain)
	}
	return rowStyle.Render(strings.TrimSuffix(plain, delay)) + delayStyle.Render(delay)
}

func usageBar(used, total int64, width int) string {
	if total <= 0 || width < 8 {
		return ""
	}
	ratio := float64(used) / float64(total)
	if ratio > 1 {
		ratio = 1
	}
	label := fmt.Sprintf(" %s/%s ", formatBytes(used), formatBytes(total))
	barW := width - lipgloss.Width(label) - 2
	if barW < 4 {
		barW = 4
	}
	filled := int(ratio * float64(barW))
	if filled > barW {
		filled = barW
	}
	style := barFull
	switch {
	case ratio > 0.9:
		style = barDanger
	case ratio > 0.7:
		style = barWarning
	}
	return style.Render(strings.Repeat("█", filled)) +
		barEmpty.Render(strings.Repeat("░", barW-filled)) +
		textSubtle.Render(label)
}

func formatRate(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB/s", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB/s", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B/s", n)
	}
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func maskURL(raw string) string {
	if raw == "" {
		return "-"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "(无效地址)"
	}
	return u.Scheme + "://" + u.Host + "/…"
}

func modeLabel(mode string) string {
	switch config.NormalizeMode(mode) {
	case "global":
		return "全局"
	case "direct":
		return "直连"
	default:
		return "分流"
	}
}

func nextMode(current string) string {
	switch config.NormalizeMode(current) {
	case "rule":
		return "global"
	case "global":
		return "direct"
	default:
		return "rule"
	}
}

// --- viewport ---

type viewport struct {
	visible  int
	showUp   bool
	showDown bool
	end      int
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
	maxVis := budget - arrows
	if maxVis < 1 {
		maxVis = 1
	}
	showDown := offset+maxVis < total
	if showDown {
		arrows++
	}
	vis := budget - arrows
	if vis < 1 {
		vis = 1
	}
	end := offset + vis
	if end > total {
		end = total
	}
	return viewport{visible: vis, showUp: showUp, showDown: showDown, end: end}
}

func (m Model) listBudget() int {
	if m.height <= 0 {
		return 8
	}
	used := 4 // HUD base
	if strings.TrimSpace(m.status) != "" {
		used++
	}
	if info := m.provider.SubscriptionInfo; info != nil && info.Total > 0 {
		used++
	}
	if m.err != "" {
		used++
	}
	used += 4 + 3 // panel chrome + footer
	b := m.height - used
	if b < 1 {
		return 1
	}
	return b
}

func (m Model) linkListBudget() int {
	if m.height <= 0 {
		return 6
	}
	b := m.height - 10
	if b < 2 {
		return 2
	}
	return b
}

func (m Model) listViewport() viewport {
	return computeViewport(len(m.nodes), m.rowOffset, m.listBudget())
}

func (m Model) linkViewport() viewport {
	return computeViewport(len(m.linkURLs), m.linkRowOffset, m.linkListBudget())
}

func clampScroll(total, cursor, offset, visible int) (int, int) {
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
	if cursor >= offset+visible {
		offset = cursor - visible + 1
	}
	maxOff := total - visible
	if maxOff < 0 {
		maxOff = 0
	}
	if offset > maxOff {
		offset = maxOff
	}
	if offset < 0 {
		offset = 0
	}
	return cursor, offset
}

func (m *Model) moveCursor(delta int) {
	if len(m.nodes) == 0 {
		return
	}
	m.cursor += delta
	m.clampListScroll()
}

func (m *Model) clampListScroll() {
	vp := m.listViewport()
	m.cursor, m.rowOffset = clampScroll(len(m.nodes), m.cursor, m.rowOffset, vp.visible)
}

func (m *Model) moveLinkCursor(delta int) {
	if len(m.linkURLs) == 0 {
		return
	}
	m.linkCursor += delta
	m.clampLinkScroll()
}

func (m *Model) clampLinkScroll() {
	vp := m.linkViewport()
	m.linkCursor, m.linkRowOffset = clampScroll(len(m.linkURLs), m.linkCursor, m.linkRowOffset, vp.visible)
}
