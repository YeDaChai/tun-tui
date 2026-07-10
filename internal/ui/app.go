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

// ─── color palette ────────────────────────────────────────────
var (
	accent    = lipgloss.Color("51")
	accentDim = lipgloss.Color("36")
	green     = lipgloss.Color("42")
	red       = lipgloss.Color("196")
	yellow    = lipgloss.Color("220")
	muted     = lipgloss.Color("241")
	subtle    = lipgloss.Color("238")
	bright    = lipgloss.Color("255")
	fg        = lipgloss.Color("252")
)

// ─── base styles ──────────────────────────────────────────────
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent)

	panelBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1)

	panelBorderActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentDim).
				Padding(0, 1)

	statusOn  = lipgloss.NewStyle().Foreground(green).Bold(true)
	statusOff = lipgloss.NewStyle().Foreground(red)

	errStyle = lipgloss.NewStyle().Foreground(red)
	dimStyle = lipgloss.NewStyle().Foreground(muted)

	// input
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1, 2)

	// list items
	selectedItem = lipgloss.NewStyle().
			Foreground(bright).
			Background(lipgloss.Color("236"))
	currentItem = lipgloss.NewStyle().
			Foreground(green)
	normalItem = lipgloss.NewStyle().
			Foreground(fg)

	// scrollbar
	scrollTrack = lipgloss.NewStyle().Foreground(subtle)
	scrollThumb = lipgloss.NewStyle().Foreground(muted)

	// latency coloring
	latencyFast = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	latencyMid  = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	latencySlow = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	// traffic arrows
	upArrow   = lipgloss.NewStyle().Foreground(accent)
	downArrow = lipgloss.NewStyle().Foreground(green)

	// usage bar
	barFilledLow    = lipgloss.NewStyle().Foreground(accent)
	barFilledMid    = lipgloss.NewStyle().Foreground(yellow)
	barFilledHigh   = lipgloss.NewStyle().Foreground(red)
	barEmpty        = lipgloss.NewStyle().Foreground(subtle)

	// keybinding chips
	keyStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)
	keySep = lipgloss.NewStyle().
			Foreground(subtle)

	// section label
	sectionLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentDim)
)

type tickMsg struct{}
type refreshMsg struct {
	version         string
	mode            string
	traffic         api.Traffic
	group           api.Proxy
	groups          []string
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
	proxyGroups     []string
	activeGroup     string
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
		activeGroup:     "PROXY",
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

		var groups []string
		for name, p := range proxies.Proxies {
			if len(p.All) > 0 {
				groups = append(groups, name)
			}
		}

		group, ok := proxies.Proxies[m.activeGroup]
		if !ok {
			if len(groups) > 0 {
				group = proxies.Proxies[groups[0]]
			} else {
				return refreshMsg{
					version: version.Version,
					mode:    config.NormalizeMode(cfg.Mode),
					traffic: traffic,
					err:     fmt.Errorf("no proxy groups found"),
				}
			}
		}

		subURL, _ := config.LoadSubscriptionURL(m.paths.DataDir)
		msg := refreshMsg{
			version:         version.Version,
			mode:            config.NormalizeMode(cfg.Mode),
			traffic:         traffic,
			group:           group,
			groups:          groups,
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
		delays, err := m.api.GroupDelay(m.activeGroup, testURL)
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
		m.proxyGroups = msg.groups
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
			err := m.api.SelectProxy(m.activeGroup, node)
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

// ─── header ───────────────────────────────────────────────────

func (m Model) renderHeader() string {
	var b strings.Builder

	// title bar with decorative line
	title := fmt.Sprintf("VPN TUI %s", m.appVersion)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")

	// separator
	sep := sepStyle.Render(strings.Repeat("─", m.contentWidth()))
	b.WriteString(sep)
	b.WriteString("\n\n")

	return b.String()
}

// ─── status section ───────────────────────────────────────────

func (m Model) renderStatus() string {
	var b strings.Builder

	// row 1: connection status + kernel + mode
	statusIcon := "○"
	statusLabel := "未连接"
	statusStyle := statusOff
	if m.running {
		statusIcon = "●"
		statusLabel = "已连接"
		statusStyle = statusOn
	} else if m.starting {
		statusIcon = "◉"
		statusLabel = "连接中"
		statusStyle = statusOn
	}

	b.WriteString(statusStyle.Render(fmt.Sprintf("%s %s", statusIcon, statusLabel)))

	metaParts := []string{}
	if m.version != "" {
		metaParts = append(metaParts, fmt.Sprintf("内核 %s", m.version))
	}
	if m.mode != "" {
		metaParts = append(metaParts, fmt.Sprintf("路由 %s", modeLabel(m.mode)))
	}
	if len(metaParts) > 0 {
		meta := "  " + strings.Join(metaParts, "  ")
		b.WriteString(dimStyle.Render(meta))
	}
	b.WriteString("\n")

	// row 2: traffic
	if m.running {
		up := formatRate(m.traffic.Up)
		down := formatRate(m.traffic.Down)
		b.WriteString(upArrow.Render("↑ ") + up)
		b.WriteString(dimStyle.Render("  "))
		b.WriteString(downArrow.Render("↓ ") + down)
	} else {
		b.WriteString(dimStyle.Render("↑ -  ↓ -"))
	}
	b.WriteString("\n")

	// row 3: status text
	b.WriteString(dimStyle.Render(m.fitLine(m.status)))
	b.WriteString("\n")

	// row 4: subscription
	if m.hasSubscription {
		b.WriteString(dimStyle.Render(m.fitLine(maskURL(m.subscriptionURL))))
		b.WriteString("\n")
		if info := m.provider.SubscriptionInfo; info != nil && info.Total > 0 {
			used := info.Upload + info.Download
			bar := renderUsageBar(used, info.Total, 20)
			b.WriteString(dimStyle.Render(fmt.Sprintf("  %s  %s / %s", bar, formatTraffic(used), formatTraffic(info.Total))))
			b.WriteString("\n")
		}
	}

	// error
	if m.err != "" {
		b.WriteString(errStyle.Render(m.fitLine("✗ " + m.err)))
		b.WriteString("\n")
	}

	return b.String()
}

// ─── proxy list panel ─────────────────────────────────────────

func (m Model) renderProxyPanel() string {
	var b strings.Builder

	// panel title
	title := fmt.Sprintf(" %s ", m.activeGroup)
	if len(m.nodes) > 0 {
		title = fmt.Sprintf(" %s (%d/%d) ", m.activeGroup, m.cursor+1, len(m.nodes))
	}
	if m.group.Now != "" {
		title += fmt.Sprintf("◆ %s ", m.group.Now)
	}

	panelWidth := m.contentWidth() - 4
	if panelWidth < 20 {
		panelWidth = 20
	}

	titleLine := sectionLabel.Render(truncateRunewidth(title, panelWidth))
	b.WriteString(titleLine)
	b.WriteString("\n")

	// separator line
	b.WriteString(sepStyle.Render(strings.Repeat("─", panelWidth)))
	b.WriteString("\n")

	// node list
	b.WriteString(m.renderNodeList())
	return b.String()
}

func (m Model) renderNodeList() string {
	vp := m.listViewport()
	panelWidth := m.contentWidth() - 4
	if panelWidth < 20 {
		panelWidth = 20
	}

	var lines []string

	if len(m.nodes) == 0 {
		lines = append(lines, dimStyle.Render("  "+m.emptyListHintText()))
	} else {
		if vp.showUpArrow {
			lines = append(lines, dimStyle.Render("  ↑"))
		}
		for i := m.rowOffset; i < vp.endIdx; i++ {
			itemLine := m.formatListItem(i, panelWidth)
			scrollChar := m.scrollbarChar(i-m.rowOffset, vp)
			lines = append(lines, itemLine+scrollChar)
		}
		if vp.showDownArrow {
			lines = append(lines, dimStyle.Render("  ↓"))
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) scrollbarChar(row int, vp struct {
	visibleRows   int
	showUpArrow   bool
	showDownArrow bool
	endIdx        int
}) string {
	total := len(m.nodes)
	visible := vp.visibleRows
	if total <= visible {
		return ""
	}

	thumbSize := visible * visible / total
	if thumbSize < 1 {
		thumbSize = 1
	}
	thumbStart := m.rowOffset * visible / total
	thumbEnd := thumbStart + thumbSize
	if thumbEnd > visible {
		thumbEnd = visible
	}

	if row >= thumbStart && row < thumbEnd {
		return scrollThumb.Render("▐")
	}
	return scrollTrack.Render("▐")
}

func (m Model) emptyListHintText() string {
	switch {
	case m.starting:
		return "(正在建立 VPN 连接...)"
	case m.running:
		return "(正在加载节点...)"
	case m.hasSubscription:
		return "(等待连接...)"
	default:
		return "(按 i 输入订阅地址)"
	}
}

func (m Model) formatListItem(idx int, width int) string {
	node := m.nodes[idx]
	active := idx == m.cursor
	current := node == m.group.Now

	prefix := "  "
	if current {
		prefix = "● "
	}

	delayStr := ""
	var delayStyle lipgloss.Style
	if d, ok := m.delays[node]; ok {
		if d > 0 {
			delayStr = fmt.Sprintf("%dms", d)
			switch {
			case d < 200:
				delayStyle = latencyFast
			case d < 500:
				delayStyle = latencyMid
			default:
				delayStyle = latencySlow
			}
		} else {
			delayStr = "×"
			delayStyle = latencySlow
		}
	}

	// width already accounts for border+padding, -1 for scrollbar gutter
	nameMax := width - runewidth.StringWidth(prefix) - 1
	if delayStr != "" {
		nameMax -= runewidth.StringWidth(delayStr) + 2
	}
	if nameMax < 4 {
		nameMax = 4
	}

	line := prefix + truncateRunewidth(node, nameMax)
	itemStyle := normalItem
	switch {
	case active:
		itemStyle = selectedItem
	case current:
		itemStyle = currentItem
	}

	if delayStr != "" {
		pad := width - 1 - runewidth.StringWidth(line) - runewidth.StringWidth(delayStr)
		if pad < 1 {
			pad = 1
		}
		return itemStyle.Render(line+strings.Repeat(" ", pad)) + delayStyle.Render(delayStr)
	}

	remaining := width - 1 - runewidth.StringWidth(line)
	if remaining > 0 {
		line += strings.Repeat(" ", remaining)
	}
	return itemStyle.Render(line)
}

// ─── footer / keybindings ─────────────────────────────────────

func (m Model) renderFooter() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(sepStyle.Render(strings.Repeat("─", m.contentWidth())))
	b.WriteString("\n")

	keys := [][2]string{
		{"i", "订阅"},
		{"s", "连接"},
		{"m", "模式"},
		{"j/k", "移动"},
		{"↵", "选择"},
		{"u", "更新"},
		{"t", "测速"},
		{"r", "重载"},
		{"q", "退出"},
	}

	width := m.contentWidth()
	sep := "  "
	sepW := runewidth.StringWidth(sep)

	var line strings.Builder
	lineW := 0
	first := true

	flush := func() {
		if line.Len() == 0 {
			return
		}
		b.WriteString(line.String())
		b.WriteString("\n")
		line.Reset()
		lineW = 0
		first = true
	}

	for _, k := range keys {
		chip := keyStyle.Render(k[0]) + dimStyle.Render(" "+k[1])
		chipW := runewidth.StringWidth(k[0]) + 1 + runewidth.StringWidth(k[1])
		need := chipW
		if !first {
			need += sepW
		}
		if !first && lineW+need > width {
			flush()
		}
		if !first {
			line.WriteString(sep)
			lineW += sepW
		}
		line.WriteString(chip)
		lineW += chipW
		first = false
	}
	flush()

	return b.String()
}

// ─── viewport helpers ─────────────────────────────────────────

func (m Model) listBudget() int {
	if m.height <= 0 {
		return 8
	}
	used := lineCount(m.renderHeader()) +
		lineCount(m.renderStatus()) +
		5 + // blank + border top + title + separator + border bottom
		lineCount(m.renderFooter())
	budget := m.height - used
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

// ─── main view ────────────────────────────────────────────────

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

	inputView := inputStyle.Render(m.subInput.View())
	b.WriteString(inputView)
	b.WriteString("\n\n")

	// styled hints
	b.WriteString(keyStyle.Render("Enter") + dimStyle.Render(" 保存"))
	b.WriteString(keySep.Render("  │  "))
	b.WriteString(keyStyle.Render("Esc") + dimStyle.Render(" 取消"))
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewMain() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString(m.renderStatus())
	b.WriteString("\n")

	// proxy panel with border
	panelContent := m.renderProxyPanel()

	if m.running && m.activeGroup != "" {
		b.WriteString(panelBorderActive.Width(m.contentWidth()).Render(panelContent))
	} else {
		b.WriteString(panelBorder.Width(m.contentWidth()).Render(panelContent))
	}

	b.WriteString(m.renderFooter())

	// fill remaining height
	if m.height > 0 {
		if total := lineCount(b.String()); total < m.height {
			b.WriteString(strings.Repeat("\n", m.height-total))
		}
	}

	return b.String()
}

// ─── helpers ──────────────────────────────────────────────────

var sepStyle = lipgloss.NewStyle().Foreground(subtle)

func (m Model) fitLine(s string) string {
	return truncateRunewidth(s, m.contentWidth())
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

func renderUsageBar(used, total int64, width int) string {
	if total <= 0 || width < 3 {
		return ""
	}
	ratio := float64(used) / float64(total)
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	var b strings.Builder
	filledStyle := barFilledLow
	if ratio > 0.9 {
		filledStyle = barFilledHigh
	} else if ratio > 0.7 {
		filledStyle = barFilledMid
	}

	b.WriteString(filledStyle.Render(strings.Repeat("█", filled)))
	b.WriteString(barEmpty.Render(strings.Repeat("░", width-filled)))
	return b.String()
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

func Run(paths config.Paths, runner *core.Runner, client *api.Client, appVersion, binName string) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("需要在交互式终端中运行，不能从 IDE 的 Run/Debug 按钮直接启动\n请在终端执行:\n  %s", binName)
	}

	p := tea.NewProgram(New(paths, runner, client, appVersion), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
