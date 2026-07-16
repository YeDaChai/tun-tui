package ui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"tun-tui/internal/config"
)

// Dark palette aligned with Claude Code defaults (warm coral accent, readable grays).
var (
	accent  = lipgloss.Color("#D77757") // claude
	suggest = lipgloss.Color("#B1B9F9") // suggestion / permission
	ok      = lipgloss.Color("#4EBA65") // success
	warn    = lipgloss.Color("#FFC107") // warning
	danger  = lipgloss.Color("#FF6B80") // error
	muted   = lipgloss.Color("#999999") // inactive — secondary labels
	subtle  = lipgloss.Color("#666666") // chrome / borders (brighter than CC #505050)
	selBg   = lipgloss.Color("#2A2420") // warm selection wash

	frameBorderActive = lipgloss.NewStyle().Foreground(accent)
	statusOnline      = lipgloss.NewStyle().Foreground(ok).Bold(true)
	statusOffline     = lipgloss.NewStyle().Foreground(muted)
	statusLoading     = lipgloss.NewStyle().Foreground(warn).Bold(true)
	textErr           = lipgloss.NewStyle().Foreground(danger)
	textSubtle        = lipgloss.NewStyle().Foreground(muted)
	itemSelected      = lipgloss.NewStyle().
				Foreground(suggest).
				Background(selBg).
				Bold(true)
	itemCurrent  = lipgloss.NewStyle().Foreground(ok).Bold(true)
	itemNormal   = lipgloss.NewStyle().Foreground(muted)
	pingFast     = lipgloss.NewStyle().Foreground(ok)
	pingMid      = lipgloss.NewStyle().Foreground(warn)
	pingSlow     = lipgloss.NewStyle().Foreground(danger)
	txColor      = lipgloss.NewStyle().Foreground(accent)
	rxColor      = lipgloss.NewStyle().Foreground(ok)
	dividerStyle = lipgloss.NewStyle().Foreground(subtle)
	footerKey    = lipgloss.NewStyle().Foreground(accent).Bold(true)
	footerLabel  = lipgloss.NewStyle().Foreground(muted)
	footerSep    = lipgloss.NewStyle().Foreground(subtle)
	sectionTitle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	barFull      = lipgloss.NewStyle().Foreground(ok)
	barWarning   = lipgloss.NewStyle().Foreground(warn)
	barDanger    = lipgloss.NewStyle().Foreground(danger)
	modeActive   = lipgloss.NewStyle().Foreground(accent).Bold(true)
)

func (m Model) View() string {
	switch m.screen {
	case screenLinkList:
		return m.viewLinkScreen()
	case screenSettings:
		return m.viewSettingsScreen()
	default:
		return m.viewMain()
	}
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

func (m Model) modalWidth(max int) int {
	w := m.contentWidth() - 8
	if w > max {
		w = max
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

func (m Model) renderHUD() string {
	w := m.contentWidth()
	f := newFrame(w, false)
	var b strings.Builder
	b.WriteString(f.top() + "\n")
	b.WriteString(f.row(m.hudPrimaryLine(w)) + "\n")
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
	case m.work == workConnecting:
		return statusLoading.Render("… 连接中")
	default:
		return statusOffline.Render("○ 未连接")
	}
}

func (m Model) hudPrimaryLine(width int) string {
	sep := textSubtle.Render("  ·  ")
	leftParts := []string{m.connectionLine()}
	if info := m.provider.SubscriptionInfo; info != nil && info.Total > 0 {
		leftParts = append(leftParts, usageCircle(info.Upload+info.Download, info.Total))
	}
	leftParts = append(leftParts, m.trafficLine())
	left := strings.Join(leftParts, sep)

	if m.mode == "" {
		return truncate(left, width)
	}
	right := textSubtle.Render("模式 ") + modeActive.Render(modeLabel(m.mode))
	return newFrame(width, false).split(left, right)
}

func (m Model) trafficLine() string {
	up, down := "--", "--"
	if m.running {
		up, down = formatRate(m.traffic.Up), formatRate(m.traffic.Down)
	}
	return txColor.Render("↑") + " " + up +
		textSubtle.Render("  ") +
		rxColor.Render("↓") + " " + down
}

func (m Model) renderProxyPanel() string {
	w := m.contentWidth()
	if w < 16 {
		w = 16
	}
	f := newFrame(w, m.running)
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
		b.WriteString(f.row(line) + "\n")
	}
	b.WriteString(f.bottom())
	return b.String()
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m Model) renderNodeList(innerW int) string {
	budget := m.listBudget()
	if budget < 1 {
		budget = 1
	}
	if m.work.spinning() || len(m.nodes) == 0 {
		return strings.Join(m.emptyNodeLines(innerW, budget), "\n")
	}
	vp := m.listViewport()
	lines := make([]string, 0, budget)
	for i := m.rowOffset; i < vp.end; i++ {
		lines = append(lines, pad(m.formatListItem(i, innerW), innerW))
	}
	for len(lines) < budget {
		lines = append(lines, pad("", innerW))
	}
	return strings.Join(lines, "\n")
}

func (m Model) emptyNodeLines(width, height int) []string {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = pad("", width)
	}
	content := m.emptyPlaceholder()
	if len(content) == 0 || height == 0 {
		return lines
	}
	start := (height - len(content)) / 2
	if start < 0 {
		start = 0
	}
	for i, line := range content {
		y := start + i
		if y >= height {
			break
		}
		lines[y] = centerText(line, width)
	}
	return lines
}

func (m Model) emptyPlaceholder() []string {
	switch {
	case m.work == workConnecting:
		frame := spinnerFrames[m.spinner%len(spinnerFrames)]
		return []string{textSubtle.Render(frame + " 正在连接…")}
	case m.work == workLoadingNodes:
		frame := spinnerFrames[m.spinner%len(spinnerFrames)]
		return []string{textSubtle.Render(frame + " 加载节点中…")}
	case m.running && m.err == "":
		return []string{textSubtle.Render("加载节点中…")}
	case m.running && m.err != "":
		return []string{
			textErr.Render("404"),
			textSubtle.Render("代理加载失败"),
		}
	case m.hasSubscription:
		return []string{textSubtle.Render("等待连接…")}
	default:
		return []string{textSubtle.Render("按 L 添加订阅链接")}
	}
}

func centerText(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return truncate(s, width)
	}
	left := (width - w) / 2
	return pad(strings.Repeat(" ", left)+s, width)
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
			delayStr, delayStyle = "连接失败", pingSlow
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
		{"S", "连接"}, {"JK", "选择"}, {"M", "模式"}, {"T", "测速"},
		{"U", "更新"}, {"L", "链接"}, {"P", "设置"}, {"Q", "退出"},
	}
	items := make([]string, 0, len(keys))
	total := 0
	for _, k := range keys {
		item := footerKey.Render(k[0]) + footerLabel.Render(" "+k[1])
		items = append(items, item)
		total += lipgloss.Width(item)
	}
	return "\n" + f.bottom() + "\n" + distributeItems(items, total, w)
}

// distributeItems spreads items across width with even gaps between them.
func distributeItems(items []string, total, width int) string {
	if len(items) == 0 || width <= 0 {
		return ""
	}
	if len(items) == 1 {
		return pad(truncate(items[0], width), width)
	}
	gaps := len(items) - 1
	if total+gaps > width {
		compact := make([]string, len(items))
		copy(compact, items)
		line := strings.Join(compact, footerSep.Render(" "))
		return pad(truncate(line, width), width)
	}
	remain := width - total
	base := remain / gaps
	extra := remain % gaps
	var b strings.Builder
	for i, item := range items {
		b.WriteString(item)
		if i < gaps {
			g := base
			if i < extra {
				g++
			}
			b.WriteString(strings.Repeat(" ", g))
		}
	}
	return pad(b.String(), width)
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
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	switch {
	case w == width:
		return s
	case w > width:
		// Flag emoji / CJK can under-measure; clip so resize ghosts don't accumulate.
		return ansi.Truncate(s, width, "")
	default:
		return s + strings.Repeat(" ", width-w)
	}
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

func usageCircle(used, total int64) string {
	if total <= 0 {
		return ""
	}
	ratio := float64(used) / float64(total)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	style := barFull
	switch {
	case ratio > 0.9:
		style = barDanger
	case ratio > 0.7:
		style = barWarning
	}
	glyph := "○"
	switch {
	case ratio >= 0.875:
		glyph = "●"
	case ratio >= 0.625:
		glyph = "◕"
	case ratio >= 0.375:
		glyph = "◑"
	case ratio >= 0.125:
		glyph = "◔"
	}
	label := fmt.Sprintf("%s / %s", formatBytes(used), formatBytes(total))
	return style.Render(glyph) + " " + textSubtle.Render(label)
}

func formatRate(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB/s", float64(n)/(1<<30))
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
	visible int
	end     int
}

func computeViewport(total, offset, budget int) viewport {
	if budget < 1 {
		budget = 1
	}
	end := offset + budget
	if end > total {
		end = total
	}
	vis := end - offset
	if vis < 0 {
		vis = 0
	}
	return viewport{visible: vis, end: end}
}

func (m Model) listBudget() int {
	if m.height <= 0 {
		return 8
	}
	used := 3 // HUD base (top + status/traffic + bottom)
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
	// Keep the modal compact so it stays centered over the main screen.
	b := 8
	if m.height > 0 {
		b = m.height/2 - 6
		if b > 12 {
			b = 12
		}
	}
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
