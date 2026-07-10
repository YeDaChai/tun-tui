package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"

	"tun-tui/internal/api"
	"tun-tui/internal/config"
	"tun-tui/internal/core"
)

const testURL = "https://www.gstatic.com/generate_204"

type screen int

const (
	screenMain screen = iota
	screenSubInput
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	statusOn    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	statusOff   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	inputStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	activeItem  = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("205"))
	currentItem = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	normalItem  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

type tickMsg struct{}
type refreshMsg struct {
	version         string
	mode            string
	traffic         api.Traffic
	group           api.Proxy
	provider        api.ProxyProvider
	hasSubscription bool
	err             error
}
type delayMsg struct {
	delays map[string]uint16
	err    error
}
type actionMsg struct {
	status  string
	err     error
	refresh bool
}
type startMsg struct {
	err error
}
type autoConnectMsg struct{}

type Model struct {
	paths           config.Paths
	runner          *core.Runner
	api             *api.Client
	appVersion      string
	screen          screen
	subInput        textinput.Model
	subscriptionURL string
	running         bool
	starting        bool
	version         string
	mode            string
	traffic         api.Traffic
	group           api.Proxy
	provider        api.ProxyProvider
	hasSubscription bool
	nodes           []string
	cursor          int
	rowOffset       int
	delays          map[string]uint16
	status          string
	err             string
	width           int
	height          int
}

func New(paths config.Paths, runner *core.Runner, client *api.Client, appVersion string) Model {
	subURL, _ := config.LoadSubscriptionURL(paths.DataDir)
	status := "按 i 输入订阅地址"
	if subURL != "" {
		status = "正在自动连接..."
	}

	ti := textinput.New()
	ti.Placeholder = "https://your-subscription-url"
	ti.CharLimit = 2048
	ti.Width = 64
	ti.Prompt = "订阅地址: "
	ti.SetValue(subURL)

	return Model{
		paths:           paths,
		runner:          runner,
		api:             client,
		appVersion:      appVersion,
		screen:          screenMain,
		subInput:        ti,
		subscriptionURL: subURL,
		nodes:           []string{},
		delays:          map[string]uint16{},
		status:          status,
		hasSubscription: subURL != "",
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick(), textinput.Blink}
	if m.hasSubscription {
		cmds = append(cmds, autoConnect())
	}
	return tea.Batch(cmds...)
}

func autoConnect() tea.Cmd {
	return func() tea.Msg { return autoConnectMsg{} }
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func refresh(m Model) tea.Cmd {
	return func() tea.Msg {
		if !m.running {
			return refreshMsg{err: fmt.Errorf("内核未运行")}
		}

		version, err := m.api.Version()
		if err != nil {
			return refreshMsg{err: err}
		}

		cfg, err := m.api.Configs()
		if err != nil {
			return refreshMsg{err: err}
		}

		traffic, err := m.api.Traffic()
		if err != nil {
			return refreshMsg{err: err}
		}

		proxies, err := m.api.Proxies()
		if err != nil {
			return refreshMsg{err: err}
		}

		group, ok := proxies.Proxies["PROXY"]
		if !ok {
			return refreshMsg{
				version: version.Version,
				mode:    config.NormalizeMode(cfg.Mode),
				traffic: traffic,
				err:     fmt.Errorf("配置中未找到 PROXY 策略组"),
			}
		}

		subURL, _ := config.LoadSubscriptionURL(m.paths.DataDir)
		msg := refreshMsg{
			version:         version.Version,
			mode:            config.NormalizeMode(cfg.Mode),
			traffic:         traffic,
			group:           group,
			hasSubscription: subURL != "",
		}

		providers, err := m.api.Providers()
		if err == nil {
			if provider, ok := providers.Providers[config.ProviderName]; ok {
				msg.provider = provider
			}
		}

		return msg
	}
}

func testDelay(m Model) tea.Cmd {
	return func() tea.Msg {
		delays, err := m.api.GroupDelay("PROXY", testURL)
		return delayMsg{delays: delays, err: err}
	}
}

func (m Model) saveSubscription(url string) tea.Cmd {
	return func() tea.Msg {
		if err := config.SaveSubscriptionURL(m.paths.DataDir, url); err != nil {
			return actionMsg{err: err}
		}

		msg := actionMsg{
			status:  "订阅地址已保存",
			refresh: m.running,
		}

		if m.running {
			if err := m.runner.Reload(); err != nil {
				msg.err = err
				msg.status = "订阅已保存，但重载失败"
				msg.refresh = false
			} else {
				mode := config.LoadMode(m.paths.DataDir, "rule")
				if err := m.api.PatchMode(mode); err != nil {
					msg.err = err
					msg.status = "订阅已保存，但模式同步失败"
				} else {
					msg.status = "订阅已保存并重载"
				}
			}
		}

		return msg
	}
}

func (m Model) openSubInput() Model {
	m.screen = screenSubInput
	m.subInput.SetValue(m.subscriptionURL)
	m.subInput.Focus()
	m.subInput.CursorEnd()
	m.err = ""
	m.status = "输入订阅地址"
	return m
}

func (m Model) closeSubInput() Model {
	m.screen = screenMain
	m.subInput.Blur()
	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		inputWidth := m.width - 8
		if inputWidth < 20 {
			inputWidth = 20
		}
		if inputWidth > 120 {
			inputWidth = 120
		}
		m.subInput.Width = inputWidth
		m.clampListScroll()
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{tick()}
		if m.running && m.screen == screenMain {
			cmds = append(cmds, refresh(m))
		}
		return m, tea.Batch(cmds...)

	case refreshMsg:
		if msg.err != nil {
			if m.running {
				m.err = msg.err.Error()
			}
			return m, nil
		}
		m.err = ""
		m.version = msg.version
		m.mode = msg.mode
		m.traffic = msg.traffic
		m.group = msg.group
		m.provider = msg.provider
		m.hasSubscription = msg.hasSubscription
		m.nodes = msg.group.All
		m.clampListScroll()
		return m, nil

	case delayMsg:
		if msg.err != nil {
			m.status = "测速失败"
			m.err = msg.err.Error()
			return m, nil
		}
		m.delays = msg.delays
		m.status = "测速完成"
		m.err = ""
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.err = ""
		}
		if msg.status != "" {
			m.status = msg.status
		}
		subURL, _ := config.LoadSubscriptionURL(m.paths.DataDir)
		m.subscriptionURL = subURL
		m.hasSubscription = subURL != ""
		m.subInput.SetValue(subURL)
		if msg.refresh {
			return m, refresh(m)
		}
		return m, nil

	case startMsg:
		m.starting = false
		m.running = m.runner.Running()
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "连接失败"
			return m, nil
		}
		m.running = true
		m.status = "VPN 已连接，流量经 utun 网卡转发"
		m.err = ""
		return m, refresh(m)

	case autoConnectMsg:
		return m.beginConnect()

	case tea.MouseMsg:
		if m.screen != screenMain || len(m.nodes) == 0 {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollRows(-1)
		case tea.MouseButtonWheelDown:
			m.scrollRows(1)
		}
		return m, nil

	case tea.KeyMsg:
		if m.screen == screenSubInput {
			return m.updateSubInput(msg)
		}
		return m.updateMain(msg)
	}

	return m, nil
}

func (m Model) updateSubInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m = m.closeSubInput()
		m.status = "已取消"
		return m, nil

	case "enter":
		url := strings.TrimSpace(m.subInput.Value())
		m = m.closeSubInput()
		return m, m.saveSubscription(url)

	case "ctrl+c":
		_ = m.runner.Stop()
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.subInput, cmd = m.subInput.Update(msg)
	return m, cmd
}

func (m Model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		_ = m.runner.Stop()
		return m, tea.Quit

	case "i":
		return m.openSubInput(), textinput.Blink

	case "s":
		if m.starting {
			return m, nil
		}
		if m.running {
			err := m.runner.Stop()
			m.running = false
			m.status = "VPN 已断开"
			m.version = ""
			m.nodes = nil
			m.provider = api.ProxyProvider{}
			m.delays = map[string]uint16{}
			m.rowOffset = 0
			m.cursor = 0
			if err != nil {
				m.err = err.Error()
			} else {
				m.err = ""
			}
			return m, nil
		}
		return m.beginConnect()

	case "r":
		if !m.running {
			m.status = "请先连接 VPN"
			return m, nil
		}
		if err := m.runner.Reload(); err != nil {
			m.err = err.Error()
			return m, nil
		}
		mode := config.LoadMode(m.paths.DataDir, "rule")
		if err := m.api.PatchMode(mode); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.status = "配置已重载"
		return m, refresh(m)

	case "u":
		if !m.running {
			m.status = "请先连接 VPN"
			return m, nil
		}
		m.status = "更新订阅中..."
		return m, func() tea.Msg {
			err := m.api.UpdateProvider(config.ProviderName)
			if err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{status: "订阅已更新", refresh: true}
		}

	case "t":
		if !m.running {
			m.status = "请先连接 VPN"
			return m, nil
		}
		m.status = "测速中..."
		return m, testDelay(m)

	case "m":
		if !m.running {
			m.status = "请先连接 VPN"
			return m, nil
		}
		next := nextMode(m.mode)
		dataDir := m.paths.DataDir
		return m, func() tea.Msg {
			if err := config.SaveMode(dataDir, next); err != nil {
				return actionMsg{err: err}
			}
			err := m.api.PatchMode(next)
			if err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{status: "模式: " + modeLabel(next), refresh: true}
		}

	case "k", "up":
		m.moveCursor(-1)
		return m, nil

	case "j", "down":
		m.moveCursor(1)
		return m, nil

	case "enter":
		if !m.running || len(m.nodes) == 0 {
			return m, nil
		}
		node := m.nodes[m.cursor]
		return m, func() tea.Msg {
			err := m.api.SelectProxy("PROXY", node)
			if err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{status: "已切换到 " + node, refresh: true}
		}
	}

	return m, nil
}

func (m Model) beginConnect() (Model, tea.Cmd) {
	if m.starting || m.running {
		return m, nil
	}
	if m.subscriptionURL == "" {
		m.status = "连接失败"
		m.err = "请先按 i 输入订阅地址"
		return m, nil
	}
	if !core.TunBuildReady() {
		m.status = "连接失败"
		m.err = core.TunBuildHint()
		return m, nil
	}
	m.starting = true
	m.status = "VPN 连接中..."
	m.err = ""
	return m, m.startRunnerCmd()
}

func (m Model) startRunnerCmd() tea.Cmd {
	return func() tea.Msg {
		if err := m.runner.Start(); err != nil {
			return startMsg{err: err}
		}
		return startMsg{}
	}
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

func (m *Model) scrollRows(delta int) {
	if len(m.nodes) == 0 {
		return
	}
	vp := m.listViewport()
	m.rowOffset += delta
	if m.rowOffset < 0 {
		m.rowOffset = 0
	}
	maxOffset := len(m.nodes) - vp.visibleRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.rowOffset > maxOffset {
		m.rowOffset = maxOffset
	}
}

func (m *Model) clampListScroll() {
	if len(m.nodes) == 0 {
		m.cursor = 0
		m.rowOffset = 0
		return
	}
	if m.cursor >= len(m.nodes) {
		m.cursor = len(m.nodes) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	vp := m.listViewport()
	if m.cursor < m.rowOffset {
		m.rowOffset = m.cursor
	}
	if m.cursor >= m.rowOffset+vp.visibleRows {
		m.rowOffset = m.cursor - vp.visibleRows + 1
	}
	maxOffset := len(m.nodes) - vp.visibleRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.rowOffset > maxOffset {
		m.rowOffset = maxOffset
	}
}

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
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

func (m Model) renderHeader() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(m.fitLine(fmt.Sprintf("VPN TUI %s", m.appVersion))))
	b.WriteString("\n\n")

	statusText := "○ 未连接"
	statusStyle := statusOff
	if m.running {
		statusText = "● VPN 已连接"
		statusStyle = statusOn
	} else if m.starting {
		statusText = "● 连接中"
		statusStyle = statusOn
	}
	rest := fmt.Sprintf("   内核: %s   路由: %s", emptyDash(m.version), modeLabel(m.mode))
	maxRest := m.contentWidth() - runewidth.StringWidth(statusText)
	if maxRest < 0 {
		maxRest = 0
	}
	b.WriteString(statusStyle.Render(statusText))
	b.WriteString(truncateRunewidth(rest, maxRest))
	b.WriteString("\n")
	b.WriteString(m.fitLine(fmt.Sprintf("上传: %s   下载: %s", formatRate(m.traffic.Up), formatRate(m.traffic.Down))))
	b.WriteString("\n")
	b.WriteString(m.fitLine(fmt.Sprintf("状态: %s", m.status)))
	b.WriteString("\n")

	if m.hasSubscription {
		subLine := "订阅: " + maskURL(m.subscriptionURL)
		if info := m.provider.SubscriptionInfo; info != nil && info.Total > 0 {
			used := info.Upload + info.Download
			subLine += fmt.Sprintf("   流量: %s / %s", formatTraffic(used), formatTraffic(info.Total))
		}
		b.WriteString(dimStyle.Render(m.fitLine(subLine)))
		b.WriteString("\n")
	}
	if m.err != "" {
		b.WriteString(errStyle.Render(m.fitLine("错误: " + m.err)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	listTitle := "策略组 PROXY"
	if len(m.nodes) > 0 {
		listTitle += fmt.Sprintf(" (%d/%d)", m.cursor+1, len(m.nodes))
	}
	listLine := listTitle
	if m.group.Now != "" {
		listLine += "   当前: " + m.group.Now
	}
	b.WriteString(titleStyle.Render(m.fitLine(listLine)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", minInt(m.contentWidth(), 40))))
	b.WriteString("\n")
	return b.String()
}

func (m Model) renderFooter() string {
	return "\n\n" + helpStyle.Render(m.fitLine("i 订阅  s 连接  m 模式  jk 移动  enter 选择  u 更新  t 测速  q 退出")) + "\n"
}

func (m Model) listBudget() int {
	if m.height <= 0 {
		return 8
	}
	budget := m.height - lineCount(m.renderHeader()) - lineCount(m.renderFooter())
	if budget < 1 {
		return 1
	}
	return budget
}

func (m Model) listVisibleRows() int {
	return m.listBudget()
}

func (m Model) listViewport() struct {
	visibleRows   int
	showUpArrow   bool
	showDownArrow bool
	endIdx        int
} {
	budget := m.listVisibleRows()
	showUp := m.rowOffset > 0
	arrows := 0
	if showUp {
		arrows++
	}
	maxVisible := budget - arrows
	if maxVisible < 1 {
		maxVisible = 1
	}
	showDown := m.rowOffset+maxVisible < len(m.nodes)
	if showDown {
		arrows++
	}
	visible := budget - arrows
	if visible < 1 {
		visible = 1
	}
	endIdx := m.rowOffset + visible
	if endIdx > len(m.nodes) {
		endIdx = len(m.nodes)
	}
	return struct {
		visibleRows   int
		showUpArrow   bool
		showDownArrow bool
		endIdx        int
	}{visible, showUp, showDown, endIdx}
}

func (m Model) fitLine(s string) string {
	return truncateRunewidth(s, m.contentWidth())
}

func (m Model) emptyListHint() string {
	switch {
	case m.starting:
		return dimStyle.Render("  (正在建立 VPN 连接...)")
	case m.running:
		return dimStyle.Render("  (正在加载节点...)")
	case m.hasSubscription:
		return dimStyle.Render("  (等待连接...)")
	default:
		return dimStyle.Render("  (按 i 输入订阅地址)")
	}
}

func (m Model) renderNodeList() string {
	budget := m.listBudget()
	var lines []string

	if len(m.nodes) == 0 {
		lines = append(lines, m.emptyListHint())
	} else {
		vp := m.listViewport()
		if vp.showUpArrow {
			lines = append(lines, dimStyle.Render("  ↑"))
		}
		for i := m.rowOffset; i < vp.endIdx; i++ {
			lines = append(lines, m.formatListItem(i))
		}
		if vp.showDownArrow {
			lines = append(lines, dimStyle.Render("  ↓"))
		}
	}

	for len(lines) < budget {
		lines = append(lines, "")
	}
	if len(lines) > budget {
		lines = lines[:budget]
	}
	return strings.Join(lines, "\n")
}

func (m Model) formatListItem(idx int) string {
	node := m.nodes[idx]
	active := idx == m.cursor
	current := node == m.group.Now

	prefix := "  "
	if current {
		prefix = "● "
	}

	delayStr := ""
	if d, ok := m.delays[node]; ok {
		if d > 0 {
			delayStr = fmt.Sprintf("%dms", d)
		} else {
			delayStr = "×"
		}
	}

	w := m.contentWidth()
	nameMax := w - runewidth.StringWidth(prefix)
	if delayStr != "" {
		nameMax -= runewidth.StringWidth(delayStr) + 2
	}
	if nameMax < 4 {
		nameMax = 4
	}

	line := prefix + truncateRunewidth(node, nameMax)
	if delayStr != "" {
		pad := w - runewidth.StringWidth(line) - runewidth.StringWidth(delayStr)
		if pad < 1 {
			pad = 1
		}
		line += strings.Repeat(" ", pad) + delayStr
	}

	switch {
	case active:
		return activeItem.Render(line)
	case current:
		return currentItem.Render(line)
	default:
		return normalItem.Render(line)
	}
}

func truncateRunewidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= max {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > max-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
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

func formatTraffic(n int64) string {
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

func maskURL(url string) string {
	if url == "" {
		return "-"
	}
	if len(url) <= 24 {
		return url
	}
	return url[:16] + "..." + url[len(url)-8:]
}

func (m Model) View() string {
	if m.screen == screenSubInput {
		return m.viewSubInput()
	}
	return m.viewMain()
}

func (m Model) viewSubInput() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("输入订阅地址"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("粘贴订阅链接，回车保存，Esc 取消"))
	b.WriteString("\n\n")
	b.WriteString(inputStyle.Render(m.subInput.View()))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Enter 保存   Esc 取消"))
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewMain() string {
	view := m.renderHeader() + m.renderNodeList() + m.renderFooter()
	if m.height <= 0 {
		return view
	}
	if total := lineCount(view); total < m.height {
		view += strings.Repeat("\n", m.height-total)
	}
	return view
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func modeLabel(mode string) string {
	switch config.NormalizeMode(mode) {
	case "global":
		return "全局"
	case "direct":
		return "直连"
	case "rule":
		return "分流"
	default:
		if mode == "" {
			return "分流"
		}
		return mode
	}
}

func nextMode(current string) string {
	switch config.NormalizeMode(current) {
	case "rule":
		return "global"
	case "global":
		return "direct"
	case "direct":
		return "rule"
	default:
		return "rule"
	}
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func Run(paths config.Paths, runner *core.Runner, client *api.Client, appVersion, binName string) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("需要在交互式终端中运行，不能从 IDE 的 Run/Debug 按钮直接启动\n请在终端执行:\n  %s", binName)
	}

	p := tea.NewProgram(New(paths, runner, client, appVersion), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
