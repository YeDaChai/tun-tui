package ui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"tun-tui/internal/config"
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

// modalWidth: min(contentWidth-8, max); 窄屏退回 min(contentWidth, 36)，不小于 24。
func (m Model) modalWidth(maxW int) int {
	w := min(m.contentWidth()-8, maxW)
	if w < 36 {
		w = min(m.contentWidth(), 36)
	}
	return max(w, 24)
}

func (m Model) renderHUD() string {
	w := m.contentWidth()
	inner := w - 2
	if inner < 8 {
		inner = w
	}
	f := newFrame(w, m.running || m.work == workConnecting)
	var b strings.Builder
	b.WriteString(f.top() + "\n")
	b.WriteString(f.row(m.hudPrimaryLine(inner)) + "\n")
	if m.err != "" {
		b.WriteString(f.row(textErr.Render("! "+truncate(m.err, inner-2))) + "\n")
	}
	b.WriteString(f.bottom() + "\n")
	return b.String()
}

func (m Model) connectionLine() string {
	switch {
	case m.running:
		return modeActive.Render("*") + statusOnline.Render(" 已连接")
	case m.work == workConnecting:
		return statusLoading.Render(m.spinnerGlyph() + " 连接中")
	default:
		return statusOffline.Render("- 未连接")
	}
}

func (m Model) hudPrimaryLine(width int) string {
	sep := textSubtle.Render("  ")
	leftParts := []string{m.connectionLine()}
	if info := m.provider.SubscriptionInfo; info != nil && info.Total > 0 {
		leftParts = append(leftParts, usageBar(info.Upload+info.Download, info.Total))
	}
	leftParts = append(leftParts, m.trafficBars())
	left := strings.Join(leftParts, sep)

	if m.mode == "" {
		return truncate(left, width)
	}
	right := textSubtle.Render("模式 ") + modeActive.Render(modeLabel(m.mode))
	return splitRow(width, left, right)
}

func (m Model) trafficBars() string {
	up, down := int64(0), int64(0)
	if m.running {
		up, down = m.traffic.Up, m.traffic.Down
	}
	return txColor.Render("^") + trafficBar(up, txColor) +
		textSubtle.Render(" ") +
		rxColor.Render("v") + trafficBar(down, rxColor)
}

// trafficBarWidth：固定格数，避免速率跳动撑抖 HUD。
const trafficBarWidth = 4

func trafficBar(rate int64, fill lipgloss.Style) string {
	n := energyFill(rate, trafficBarWidth)
	var b strings.Builder
	b.WriteString(textSubtle.Render("["))
	if n > 0 {
		b.WriteString(fill.Render(strings.Repeat("#", n)))
	}
	if empty := trafficBarWidth - n; empty > 0 {
		b.WriteString(textSubtle.Render(strings.Repeat("-", empty)))
	}
	b.WriteString(textSubtle.Render("]"))
	return b.String()
}

func (m Model) renderProxyPanel() string {
	w := m.contentWidth()
	if w < 16 {
		w = 16
	}
	inner := w - 2
	f := newFrame(w, m.running)
	var b strings.Builder

	title := scrollPlaque("节点")
	switch {
	case len(m.nodes) > 0 && m.group.Now != "":
		title = scrollPlaque(fmt.Sprintf("节点 %d/%d | %s", m.cursor+1, len(m.nodes), layoutString(m.group.Now)))
	case len(m.nodes) > 0 && m.work == workTesting:
		title = scrollPlaque(fmt.Sprintf("节点 %d/%d | 测速中", m.cursor+1, len(m.nodes)))
	case len(m.nodes) > 0:
		title = scrollPlaque(fmt.Sprintf("节点 %d/%d", m.cursor+1, len(m.nodes)))
	case m.work == workTesting:
		title = scrollPlaque("节点 | 测速中")
	}
	b.WriteString(f.top() + "\n")
	b.WriteString(f.row(sectionTitle.Render(fitCells(title, inner))) + "\n")
	b.WriteString(f.row(mistSep(inner)) + "\n")

	for _, line := range strings.Split(m.renderNodeList(inner), "\n") {
		b.WriteString(f.row(line) + "\n")
	}
	b.WriteString(f.bottom())
	return b.String()
}

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
		return []string{textSubtle.Render(m.spinnerGlyph() + " 正在连接…")}
	case m.work == workLoadingNodes:
		return []string{textSubtle.Render(m.spinnerGlyph() + " 加载节点中…")}
	case m.running && m.err == "":
		return []string{textSubtle.Render(m.spinnerGlyph() + " 加载节点中…")}
	case m.running && m.err != "":
		return []string{
			textErr.Render("! 错误"),
			textSubtle.Render("代理加载失败"),
		}
	case m.hasSubscription:
		return []string{textSubtle.Render("按 S 连接")}
	default:
		return []string{textSubtle.Render("按 L 添加订阅链接")}
	}
}

func centerText(s string, width int) string {
	w := cellWidth(s)
	if w >= width {
		return truncate(s, width)
	}
	left := (width - w) / 2
	return fitCells(strings.Repeat(" ", left)+s, width)
}

func (m Model) formatListItem(idx, width int) string {
	node := m.nodes[idx]
	active, current := idx == m.cursor, node == m.group.Now
	burst := m.selectBursting(node)

	mark := m.cursorMark(active)
	switch {
	case burst:
		mark = m.selectBurstMark()
	case !active && current:
		mark = "* "
	}

	delayStr := ""
	delayStyle := itemNormal
	if burst {
		delayStr = selectBurstTail(m.selectFlash)
		delayStyle = itemCurrent
	} else if d, ok := m.delays[node]; ok {
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
			delayStr, delayStyle = "失败", pingSlow
		}
	} else if m.work == workTesting {
		delayStr = m.spinnerGlyph() + ".."
		delayStyle = textSubtle
	}

	style := itemNormal
	switch {
	case burst:
		style = itemCurrent
	case active:
		style = itemSelected
	case current:
		style = itemCurrent
	}
	return buildRow(width, mark, node, delayStr, style, delayStyle, active, current || burst, burst, m.selectFlash)
}

func (m Model) renderFooter() string {
	w := m.contentWidth()
	inner := w - 2
	if inner < 8 {
		inner = w
	}
	f := newFrame(w, false)
	sLabel := "连接"
	if m.running {
		sLabel = "关闭"
	}
	keys := [][2]string{
		{"S", sLabel}, {"JK", "选择"}, {"M", "模式"}, {"T", "测速"},
		{"L", "订阅"}, {"P", "设置"}, {"Q", "退出"},
	}
	items := make([]string, 0, len(keys))
	for _, k := range keys {
		items = append(items, antiqueButton(k[0], k[1]))
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(f.top() + "\n")
	b.WriteString(f.row(flexRow(items, inner)) + "\n")
	b.WriteString(f.bottom())
	return b.String()
}

// antiqueButton: [S 关闭] — 紧凑印章格，无两侧云纹。
func antiqueButton(key, label string) string {
	return btnBorder.Render("[") +
		footerKey.Render(key) +
		textSubtle.Render(" ") +
		btnLabel.Render(label) +
		btnBorder.Render("]")
}

// flexRow: justify-content: space-between — 首尾顶边，余宽均分到缝隙。
func flexRow(items []string, width int) string {
	if len(items) == 0 || width <= 0 {
		return ""
	}
	total := 0
	for _, it := range items {
		total += cellWidth(it)
	}
	if len(items) == 1 {
		return pad(truncate(items[0], width), width)
	}
	gaps := len(items) - 1
	if total+gaps > width {
		return pad(truncate(strings.Join(items, " "), width), width)
	}
	remain := width - total
	base, extra := remain/gaps, remain%gaps
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
	return b.String()
}

func modalBox(width int, body string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.Border{
			Top:         "=",
			Bottom:      "=",
			Left:        "|",
			Right:       "|",
			TopLeft:     ".",
			TopRight:    ".",
			BottomLeft:  "'",
			BottomRight: "'",
		}).
		BorderForeground(accent).
		Padding(1, 2).
		Width(width).
		Render(body)
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

func (f frame) top() string {
	if f.width < 4 {
		return f.border.Render(strings.Repeat("=", max(f.width, 0)))
	}
	return f.border.Render(".~" + strings.Repeat("=", f.width-4) + "~.")
}

func (f frame) bottom() string {
	if f.width < 4 {
		return f.border.Render(strings.Repeat("=", max(f.width, 0)))
	}
	return f.border.Render("'~" + strings.Repeat("=", f.width-4) + "~'")
}

func (f frame) row(s string) string {
	inner := f.width - 2
	if inner < 1 {
		return fitCells(s, f.width)
	}
	return f.border.Render("|") + fitCells(s, inner) + f.border.Render("|")
}

func splitRow(width int, left, right string) string {
	rw := cellWidth(right)
	if cellWidth(left)+rw >= width {
		left = truncate(left, width-rw-1)
	}
	return fitCells(left+strings.Repeat(" ", max(width-cellWidth(left)-rw, 0))+right, width)
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

func buildRow(width int, mark, name, delay string, rowStyle, delayStyle lipgloss.Style, fullRow, current, mist bool, mistPhase int) string {
	nameMax := width - cellWidth(mark)
	if delay != "" {
		nameMax -= cellWidth(delay) + 4
	}
	if nameMax < 1 {
		nameMax = 1
	}
	prefix := mark + truncate(name, nameMax)
	if delay == "" {
		return rowStyle.Render(fitCells(prefix, width))
	}
	gap := width - cellWidth(prefix) - cellWidth(delay)
	if gap < 0 {
		gap = 0
	}
	leader := dashedLeader(gap)
	if mist {
		leader = mistLeader(gap, mistPhase)
	}
	leaderStyle := leaderDim
	switch {
	case fullRow:
		leaderStyle = leaderActive
	case current:
		leaderStyle = leaderOn
	}
	rest := width - cellWidth(prefix) - cellWidth(leader) - cellWidth(delay)
	if rest < 0 {
		rest = 0
	}
	trail := strings.Repeat(" ", rest)
	if fullRow {
		return rowStyle.Render(prefix) +
			leaderStyle.Render(leader) +
			delayStyle.Background(selBg).Render(delay) +
			lipgloss.NewStyle().Background(selBg).Render(trail)
	}
	return rowStyle.Render(prefix) + leaderStyle.Render(leader) + delayStyle.Render(delay) + trail
}

func dashedLeader(gap int) string {
	switch {
	case gap <= 0:
		return ""
	case gap <= 2:
		return strings.Repeat(" ", gap)
	default:
		return " " + strings.Repeat(".", gap-2) + " "
	}
}

// mistLeader：选中成功时淡云带，宽度与 dashedLeader 一致。
func mistLeader(gap, phase int) string {
	switch {
	case gap <= 0:
		return ""
	case gap <= 2:
		return strings.Repeat(" ", gap)
	}
	inner := gap - 2
	var b strings.Builder
	b.Grow(gap)
	b.WriteByte(' ')
	for i := 0; i < inner; i++ {
		switch (i + phase) % 3 {
		case 0:
			b.WriteByte('~')
		case 1:
			b.WriteByte('.')
		default:
			b.WriteByte(' ')
		}
	}
	b.WriteByte(' ')
	return b.String()
}

// selectBurstTail：右侧成功尾焰，固定 3 格。
func selectBurstTail(flash int) string {
	switch {
	case flash >= 10:
		return "~~~"
	case flash >= 7:
		return "~.~"
	case flash >= 4:
		return " ~ "
	case flash >= 2:
		return " . "
	default:
		return " * "
	}
}

func usageBar(used, total int64) string {
	if total <= 0 {
		return ""
	}
	const width = 8
	filled := ratioFill(used, total, width)
	fill := barFull
	ratio := float64(used) / float64(total)
	switch {
	case ratio > 0.9:
		fill = barDanger
	case ratio > 0.7:
		fill = barWarning
	}
	label := fmt.Sprintf("%s/%s", formatBytes(used), formatBytes(total))

	// [####----]：括号与空档淡墨，填充按阈值；数字淡墨作说明。
	var b strings.Builder
	b.WriteString(textSubtle.Render("["))
	if filled > 0 {
		b.WriteString(fill.Render(strings.Repeat("#", filled)))
	}
	if empty := width - filled; empty > 0 {
		b.WriteString(textSubtle.Render(strings.Repeat("-", empty)))
	}
	b.WriteString(textSubtle.Render("] " + label))
	return b.String()
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
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
	used := 3 // HUD: top + status + bottom
	if m.err != "" {
		used++
	}
	// panel: top + title + mid + bottom; footer: top + buttons + bottom (+ leading gap)
	used += 4 + 4
	b := m.height - used
	if b < 1 {
		return 1
	}
	return b
}

func (m Model) linkListBudget() int {
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
