package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-isatty"

	"tun-tui/internal/api"
	"tun-tui/internal/config"
	"tun-tui/internal/core"
)

const testURL = "https://www.gstatic.com/generate_204"

type screen int

const (
	screenMain screen = iota
	screenLinkList
)

// ═══════════════════════════════════════════════════════════════
// COLOR PALETTE — Cyberpunk / Synthwave
// ═══════════════════════════════════════════════════════════════
var (
	neonCyan   = lipgloss.Color("51")
	neonMag    = lipgloss.Color("201")
	neonGreen  = lipgloss.Color("46")
	neonYellow = lipgloss.Color("226")
	neonRed    = lipgloss.Color("196")
	neonOrange = lipgloss.Color("208")

	white    = lipgloss.Color("255")
	greyMed  = lipgloss.Color("245")
	greyLow  = lipgloss.Color("240")
	greyDark = lipgloss.Color("236")
	bgDark   = lipgloss.Color("233")

	hpGreen  = lipgloss.Color("40")
	hpYellow = lipgloss.Color("220")
	hpRed    = lipgloss.Color("196")
)

// ═══════════════════════════════════════════════════════════════
// STYLES
// ═══════════════════════════════════════════════════════════════
var (
	// ── frame borders ──
	frameBorderInactive = lipgloss.NewStyle().Foreground(greyLow)
	frameBorderActive   = lipgloss.NewStyle().Foreground(neonCyan)

	// ── status ──
	statusOnline  = lipgloss.NewStyle().Foreground(neonGreen).Bold(true)
	statusOffline = lipgloss.NewStyle().Foreground(neonRed)
	statusLoading = lipgloss.NewStyle().Foreground(neonYellow).Bold(true)

	// ── text ──
	textErr    = lipgloss.NewStyle().Foreground(neonRed)
	textMuted  = lipgloss.NewStyle().Foreground(greyMed)
	textSubtle = lipgloss.NewStyle().Foreground(greyLow)
	textBody   = lipgloss.NewStyle().Foreground(white)

	// ── input ──
	inputPanel = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(neonCyan).
			BorderTop(true).
			BorderBottom(true).
			BorderLeft(false).
			BorderRight(false).
			Padding(1, 0)

	// ── list items ──
	itemSelected = lipgloss.NewStyle().
			Foreground(neonCyan).
			Background(greyDark).
			Bold(true)
	itemCurrent = lipgloss.NewStyle().
			Foreground(neonGreen).
			Bold(true)
	itemNormal = lipgloss.NewStyle().
			Foreground(greyMed)

	// ── latency tiers ──
	pingFast  = lipgloss.NewStyle().Foreground(neonGreen)
	pingMid   = lipgloss.NewStyle().Foreground(neonYellow)
	pingSlow  = lipgloss.NewStyle().Foreground(neonRed)
	pingDead  = lipgloss.NewStyle().Foreground(greyMed)

	// ── traffic ──
	txColor = lipgloss.NewStyle().Foreground(neonCyan)
	rxColor = lipgloss.NewStyle().Foreground(neonGreen)

	// ── divider ──
	dividerStyle = lipgloss.NewStyle().Foreground(greyLow)

	// ── footer ──
	footerKey    = lipgloss.NewStyle().Foreground(neonMag).Bold(true)
	footerBracket = lipgloss.NewStyle().Foreground(greyLow)
	footerLabel   = lipgloss.NewStyle().Foreground(greyMed)
	footerSep     = lipgloss.NewStyle().Foreground(greyDark)

	// ── section headers ──
	sectionTitle = lipgloss.NewStyle().
			Foreground(neonMag).
			Bold(true)

	// ── data bars ──
	barFull    = lipgloss.NewStyle().Foreground(neonGreen)
	barWarning = lipgloss.NewStyle().Foreground(neonYellow)
	barDanger  = lipgloss.NewStyle().Foreground(neonRed)
	barEmpty   = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	// ── mode indicator ──
	modeActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("232")).
			Background(neonCyan).
			Bold(true).
			Padding(0, 1)
)

// ═══════════════════════════════════════════════════════════════
// MESSAGES
// ═══════════════════════════════════════════════════════════════

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

// ═══════════════════════════════════════════════════════════════
// MODEL
// ═══════════════════════════════════════════════════════════════

type Model struct {
	paths           config.Paths
	runner          *core.Runner
	api             *api.Client
	appVersion      string
	screen          screen
	linkInput       textinput.Model
	linkURLs        []string
	linkActive      int
	linkCursor      int
	linkRowOffset   int
	linkInputFocus  bool
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
	status := "按 [l] 管理订阅链接"
	if subURL != "" {
		status = "AUTO CONNECTING..."
	}

	ti := textinput.New()
	ti.Placeholder = "https://your-subscription-url"
	ti.CharLimit = 2048
	ti.Width = 64
	ti.Prompt = "链接: "
	ti.SetValue("")

	return Model{
		paths:           paths,
		runner:          runner,
		api:             client,
		appVersion:      appVersion,
		screen:          screenMain,
		linkInput:       ti,
		linkActive:      -1,
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
			status:  "SUBSCRIBED",
			refresh: m.running,
		}

		if m.running {
			if err := m.runner.Reload(); err != nil {
				msg.err = err
				msg.status = "RELOAD FAILED"
				msg.refresh = false
			} else {
				mode := config.LoadMode(m.paths.DataDir, "rule")
				if err := m.api.PatchMode(mode); err != nil {
					msg.err = err
					msg.status = "MODE SYNC FAILED"
				} else {
					msg.status = "SUBSCRIBED & RELOADED"
				}
			}
		}

		return msg
	}
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
		m.linkInput.Width = inputWidth
		m.clampListScroll()
		m.clampLinkScroll()
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
			m.status = "PING FAILED"
			m.err = msg.err.Error()
			return m, nil
		}
		m.delays = msg.delays
		m.status = "PING COMPLETE"
		m.err = ""
		return m, nil

	case linkMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.err = ""
		}
		if msg.added || msg.deleted {
			urls, active, _ := config.LoadSubscriptionLinks(m.paths.DataDir)
			m.linkURLs = urls
			m.linkActive = active
			if len(urls) == 0 {
				m.linkCursor = 0
				m.linkRowOffset = 0
				m.linkInputFocus = true
				m.linkInput.Focus()
			} else {
				if m.linkCursor >= len(urls) {
					m.linkCursor = len(urls) - 1
				}
				if msg.added {
					m.linkCursor = len(urls) - 1
					if active >= 0 {
						m.linkCursor = active
					}
				}
				m.clampLinkScroll()
			}
			subURL, _ := config.LoadSubscriptionURL(m.paths.DataDir)
			m.subscriptionURL = subURL
			m.hasSubscription = subURL != ""
			if msg.status != "" {
				m.status = msg.status
			} else if msg.added {
				m.status = "LINK ADDED"
			} else {
				m.status = "LINK DELETED"
			}
		}
		if msg.refresh {
			return m, refresh(m)
		}
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
		m.linkInput.SetValue("")
		if msg.refresh {
			return m, refresh(m)
		}
		return m, nil

	case startMsg:
		m.starting = false
		m.running = m.runner.Running()
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "CONNECT FAILED"
			return m, nil
		}
		m.running = true
		m.status = ""
		m.err = ""
		return m, refresh(m)

	case autoConnectMsg:
		return m.beginConnect()

	case tea.MouseMsg:
		if m.screen == screenLinkList {
			if !m.linkInputFocus {
				switch msg.Button {
				case tea.MouseButtonWheelUp:
					m.scrollLinkRows(-1)
				case tea.MouseButtonWheelDown:
					m.scrollLinkRows(1)
				}
			}
			return m, nil
		}
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
		if m.screen == screenLinkList {
			return m.updateLinkScreen(msg)
		}
		return m.updateMain(msg)
	}

	return m, nil
}

func (m Model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		_ = m.runner.Stop()
		return m, tea.Quit

	case "l":
		return m.openLinkScreen(), textinput.Blink

	case "s":
		if m.starting {
			return m, nil
		}
		if m.running {
			err := m.runner.Stop()
			m.running = false
			m.status = "DISCONNECTED"
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
			m.status = "CONNECT FIRST"
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
		m.status = "RELOADED"
		return m, refresh(m)

	case "u":
		if !m.running {
			m.status = "CONNECT FIRST"
			return m, nil
		}
		m.status = "UPDATING..."
		return m, func() tea.Msg {
			err := m.api.UpdateProvider(config.ProviderName)
			if err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{status: "UPDATED", refresh: true}
		}

	case "t":
		if !m.running {
			m.status = "CONNECT FIRST"
			return m, nil
		}
		m.status = "PINGING..."
		return m, testDelay(m)

	case "m":
		if !m.running {
			m.status = "CONNECT FIRST"
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
			return actionMsg{status: "MODE: " + modeLabel(next), refresh: true}
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
			return actionMsg{status: "SWITCHED → " + node, refresh: true}
		}
	}

	return m, nil
}

func (m Model) beginConnect() (Model, tea.Cmd) {
	if m.starting || m.running {
		return m, nil
	}
	if m.subscriptionURL == "" {
		m.status = "CONNECT FAILED"
		m.err = "按 [l] 添加订阅链接"
		return m, nil
	}
	if !core.TunBuildReady() {
		m.status = "CONNECT FAILED"
		m.err = core.TunBuildHint()
		return m, nil
	}
	m.starting = true
	m.status = "CONNECTING..."
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

// frameInner is the content width for a full-width row (no side borders).
func frameInner(frameW int) int {
	if frameW < 0 {
		return 0
	}
	return frameW
}

func visualWidth(s string) int {
	return lipgloss.Width(s)
}

func padVisual(s string, width int) string {
	w := visualWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func truncateVisual(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if visualWidth(s) <= max {
		return s
	}
	return ansi.Truncate(s, max, "…")
}

// frame draws horizontal rules with full-width content rows.
type frame struct {
	width  int
	border lipgloss.Style
}

func newFrame(width int) frame {
	return frame{width: width, border: dividerStyle}
}

func newPanelFrame(width int, active bool) frame {
	border := frameBorderInactive
	if active {
		border = frameBorderActive
	}
	return frame{width: width, border: border}
}

func (f frame) top() string {
	return f.border.Render(strings.Repeat("═", f.width))
}

func (f frame) bottom() string {
	return f.border.Render(strings.Repeat("═", f.width))
}

func (f frame) row(content string) string {
	content = truncateVisual(content, f.width)
	return padVisual(content, f.width)
}

func (f frame) rowSpaced(content string) string {
	return f.row(content)
}

func (f frame) rowSpacedLeft(left string) string {
	return f.row(left)
}

func (f frame) rowSpacedSplit(left, right string) string {
	inner := f.width
	leftW := visualWidth(left)
	rightW := visualWidth(right)
	if rightW > 0 {
		gap := inner - leftW - rightW
		if gap < 1 {
			gap = 1
			maxLeft := inner - rightW - gap
			if maxLeft < 0 {
				maxLeft = 0
			}
			left = truncateVisual(left, maxLeft)
		}
		content := left + strings.Repeat(" ", gap) + right
		return padVisual(content, inner)
	}
	return f.row(left)
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

// ═══════════════════════════════════════════════════════════════
// HUD — Heads-Up Display stats
// ═══════════════════════════════════════════════════════════════

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
		barLine := renderHPBar(used, info.Total, frameInner(w)-2)
		b.WriteString(f.rowSpacedLeft(" " + barLine))
		b.WriteString("\n")
	}

	if m.err != "" {
		errLine := textErr.Render("⚠ " + truncateVisual(m.err, frameInner(w)-2))
		b.WriteString(f.rowSpacedLeft(errLine))
		b.WriteString("\n")
	}

	b.WriteString(f.bottom())
	b.WriteString("\n")

	return b.String()
}

func (m Model) connectionLine() string {
	if m.running {
		return statusOnline.Render("● ONLINE")
	} else if m.starting {
		return statusLoading.Render("◉ CONNECTING")
	}
	return statusOffline.Render("○ OFFLINE")
}

func (m Model) metaLine() string {
	var parts []string
	if m.version != "" {
		parts = append(parts, textSubtle.Render("KERN")+textBody.Render(" "+m.version))
	}
	if m.mode != "" {
		label := modeLabel(m.mode)
		parts = append(parts, modeActive.Render(" "+label+" "))
	}
	return strings.Join(parts, "  ")
}

func (m Model) trafficGauge(width int) string {
	if !m.running {
		return textSubtle.Render("▲  --  ▼  --")
	}

	up := formatRate(m.traffic.Up)
	down := formatRate(m.traffic.Down)

	// build speed bars proportional to 1 MB/s
	upBar := miniSpeedBar(m.traffic.Up, 8)
	downBar := miniSpeedBar(m.traffic.Down, 8)

	return txColor.Render("▲") + " " + up + " " + upBar +
		textSubtle.Render("  ") +
		rxColor.Render("▼") + " " + down + " " + downBar
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
	return textSubtle.Render("> ") + textBody.Render(truncateVisual(m.status, width-2))
}

// ═══════════════════════════════════════════════════════════════
// PROXY PANEL — game inventory menu
// ═══════════════════════════════════════════════════════════════

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

	title := fmt.Sprintf(" %s ", m.activeGroup)
	if len(m.nodes) > 0 {
		title = fmt.Sprintf(" %s [%d/%d] ", m.activeGroup, m.cursor+1, len(m.nodes))
	}
	if m.group.Now != "" {
		title += fmt.Sprintf(" ◆ %s ", m.group.Now)
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
			hint := fmt.Sprintf("  △  %d more above", m.rowOffset)
			lines = append(lines, padVisual(textSubtle.Render(hint), innerW))
		}

		for i := m.rowOffset; i < vp.endIdx; i++ {
			lines = append(lines, padVisual(m.formatListItem(i, innerW), innerW))
		}

		if vp.showDownArrow {
			remaining := total - vp.endIdx
			hint := fmt.Sprintf("  ▽  %d more below", remaining)
			lines = append(lines, padVisual(textSubtle.Render(hint), innerW))
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) emptyHint() string {
	switch {
	case m.starting:
		return "... ESTABLISHING CONNECTION ..."
	case m.running:
		return "... LOADING NODES ..."
	case m.hasSubscription:
		return "... AWAITING CONNECTION ..."
	default:
		return "... PRESS [l] TO ADD SUBSCRIPTION ..."
	}
}

func (m Model) formatListItem(idx, width int) string {
	node := m.nodes[idx]
	active := idx == m.cursor
	current := node == m.group.Now

	mark := "  "
	if current {
		mark = "◆ "
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
			delayStr = "TIMEOUT"
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

// buildPlainRow assembles an unstyled row with exact visual width.
func buildPlainRow(width int, mark, name, delay string) string {
	markW := visualWidth(mark)
	delayW := visualWidth(delay)

	nameMax := width - markW
	if delay != "" {
		nameMax -= delayW
	}
	if nameMax < 1 {
		nameMax = 1
	}

	prefix := mark + truncateVisual(name, nameMax)
	if delay == "" {
		return padVisual(prefix, width)
	}

	gap := width - visualWidth(prefix) - delayW
	if gap < 0 {
		gap = 0
	}
	return padVisual(prefix+strings.Repeat(" ", gap)+delay, width)
}

// buildRow styles a fixed-width plain row.
func buildRow(width int, mark, name, delay string, rowStyle, delayStyle lipgloss.Style, fullRow bool) string {
	plain := buildPlainRow(width, mark, name, delay)
	if fullRow || delay == "" {
		return rowStyle.Render(plain)
	}
	prefix := strings.TrimSuffix(plain, delay)
	return rowStyle.Render(prefix) + delayStyle.Render(delay)
}

// ═══════════════════════════════════════════════════════════════
// FOOTER — game control bar
// ═══════════════════════════════════════════════════════════════

func (m Model) renderFooter() string {
	w := m.contentWidth()
	f := newFrame(w)
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(f.bottom())
	b.WriteString("\n ")

	keys := [][2]string{
		{"L", "LINK"},
		{"S", "CONN"},
		{"M", "MODE"},
		{"▲▼", "NAV"},
		{"↵", "SEL"},
		{"U", "UP"},
		{"T", "PING"},
		{"R", "REL"},
		{"Q", "QUIT"},
	}

	inner := frameInner(w)
	var parts []string
	for _, k := range keys {
		parts = append(parts, footerBracket.Render("[")+footerKey.Render(k[0])+footerBracket.Render("]")+footerLabel.Render(k[1]))
	}

	full := strings.Join(parts, footerSep.Render("·"))
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

// ═══════════════════════════════════════════════════════════════
// VIEWPORT
// ═══════════════════════════════════════════════════════════════

func (m Model) listBudget() int {
	if m.height <= 0 {
		return 8
	}
	used := lineCount(m.renderHUD()) +
		5 + // blank + double border top + title + divider + double border bot
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

// ═══════════════════════════════════════════════════════════════
// VIEW
// ═══════════════════════════════════════════════════════════════

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

	// fill rest
	if m.height > 0 {
		if total := lineCount(b.String()); total < m.height {
			b.WriteString(strings.Repeat("\n", m.height-total))
		}
	}

	return b.String()
}

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

func (m Model) fitLine(s string) string {
	return truncateVisual(s, m.contentWidth())
}

func padToWidth(s string, width int) string {
	return padVisual(s, width)
}

func truncateRunewidth(s string, max int) string {
	return truncateVisual(s, max)
}

// HP bar — RPG health bar style [████████░░] LABEL
func renderHPBar(used, total int64, width int) string {
	if total <= 0 || width < 8 {
		return ""
	}
	ratio := float64(used) / float64(total)
	if ratio > 1 {
		ratio = 1
	}
	barWidth := width - 16 // reserve space for label
	if barWidth < 4 {
		barWidth = 4
	}

	filled := int(ratio * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}

	var fillStyle lipgloss.Style
	switch {
	case ratio > 0.9:
		fillStyle = barDanger
	case ratio > 0.7:
		fillStyle = barWarning
	default:
		fillStyle = barFull
	}

	label := fmt.Sprintf(" %s/%s ", formatTraffic(used), formatTraffic(total))

	s := fillStyle.Render(strings.Repeat("█", filled))
	s += barEmpty.Render(strings.Repeat("█", barWidth-filled))
	s += textSubtle.Render(label)
	return s
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

func modeLabel(mode string) string {
	switch config.NormalizeMode(mode) {
	case "global":
		return "GLOBAL"
	case "direct":
		return "DIRECT"
	case "rule":
		return "RULE"
	default:
		if mode == "" {
			return "RULE"
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
