package ui

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"tun-tui/internal/api"
	"tun-tui/internal/config"
	"tun-tui/internal/core"
)

type screen int

const (
	screenMain screen = iota
	screenLinkList
	screenSettings
)

type tickMsg struct{}
type trafficMsg struct {
	gen     uint64
	traffic api.Traffic
}
type refreshMsg struct {
	gen             uint64
	mode            string
	group           api.Proxy
	provider        api.ProxyProvider
	autoTest        bool
	err             error
}
type delayOneMsg struct {
	name  string
	delay uint16
	more  <-chan delayOneMsg
}
type delayDoneMsg struct{}
type actionMsg struct {
	err      error
	refresh  bool
	autoTest bool
	added    bool
	deleted  bool
}
type startMsg struct{ err error }
type autoConnectMsg struct{}

type Model struct {
	paths           config.Paths
	runner          *core.Runner
	api             *api.Client
	screen          screen
	linkInput       textinput.Model
	linkURLs        []string
	linkActive      int
	linkCursor      int
	linkRowOffset   int
	linkInputFocus  bool
	subscriptionURL string
	running         bool
	work            workState
	mode            string
	traffic         api.Traffic
	group           api.Proxy
	provider        api.ProxyProvider
	hasSubscription bool
	nodes           []string
	cursor          int
	rowOffset       int
	delays          map[string]uint16
	delayCancel     context.CancelFunc
	err             string
	width           int
	height          int
	anim            int
	selectFlash     int    // 回车选节点成功爆发剩余帧
	selectFlashNode string // 爆发落在哪条节点
	settingsNote    string
	pollGen         uint64 // drops stale traffic/refresh after mode switch etc.
	tickN           uint64
}

func Run(ctx context.Context, paths config.Paths, runner *core.Runner, client *api.Client, binName string) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("需要在交互式终端中运行，不能从 IDE 的 Run/Debug 按钮直接启动\n请在终端执行:\n  %s", binName)
	}
	_, err := tea.NewProgram(
		New(paths, runner, client),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
	).Run()
	return err
}

func New(paths config.Paths, runner *core.Runner, client *api.Client) Model {
	subURL, err := config.LoadSubscriptionURL(paths.DataDir)

	ti := textinput.New()
	ti.Placeholder = "https://your-subscription-url"
	ti.CharLimit = 2048
	ti.Width = 64
	ti.Prompt = "链接: "

	m := Model{
		paths:           paths,
		runner:          runner,
		api:             client,
		linkInput:       ti,
		linkActive:      -1,
		subscriptionURL: subURL,
		nodes:           []string{},
		delays:          map[string]uint16{},
		hasSubscription: subURL != "",
	}
	if err != nil {
		m.err = err.Error()
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick(), animTick(), textinput.Blink}
	if m.hasSubscription {
		cmds = append(cmds, func() tea.Msg { return autoConnectMsg{} })
	}
	return tea.Batch(cmds...)
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

const delayConcurrency = 8

func (m *Model) stopDelayTest() {
	if m.delayCancel != nil {
		m.delayCancel()
		m.delayCancel = nil
	}
}

func (m Model) beginDelayTest() (Model, tea.Cmd) {
	m.stopDelayTest()
	ctx, cancel := context.WithCancel(context.Background())
	m.delayCancel = cancel
	m.work = workTesting
	m.err = ""
	m.delays = map[string]uint16{}
	return m, startDelayTest(ctx, m.api, m.nodes)
}

func startDelayTest(ctx context.Context, client *api.Client, nodes []string) tea.Cmd {
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n == "" || n == "DIRECT" || n == "REJECT" {
			continue
		}
		names = append(names, n)
	}
	if len(names) == 0 {
		return func() tea.Msg { return delayDoneMsg{} }
	}

	ch := make(chan delayOneMsg, delayConcurrency)
	go func() {
		defer close(ch)
		sem := make(chan struct{}, delayConcurrency)
		var wg sync.WaitGroup
		for _, name := range names {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				select {
				case <-ctx.Done():
					return
				case sem <- struct{}{}:
				}
				defer func() { <-sem }()

				d, err := client.ProxyDelay(ctx, name)
				if err != nil {
					d = 0
				}
				select {
				case <-ctx.Done():
				case ch <- delayOneMsg{name: name, delay: d}:
				}
			}(name)
		}
		wg.Wait()
	}()
	return waitDelayResult(ch)
}

func waitDelayResult(ch <-chan delayOneMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return delayDoneMsg{}
		}
		msg.more = ch
		return msg
	}
}

func (m *Model) invalidatePolls() {
	m.pollGen++
}

func refresh(m Model, autoTest bool) tea.Cmd {
	gen := m.pollGen
	return func() tea.Msg {
		if !m.running {
			return refreshMsg{gen: gen, err: fmt.Errorf("内核未运行")}
		}
		cfg, err := m.api.Configs()
		if err != nil {
			return refreshMsg{gen: gen, err: err}
		}
		proxies, err := m.api.Proxies()
		if err != nil {
			return refreshMsg{gen: gen, err: err}
		}

		group, ok := proxies.Proxies[api.DefaultProxyGroup]
		if !ok {
			return refreshMsg{
				gen:  gen,
				mode: config.NormalizeMode(cfg.Mode),
				err:  fmt.Errorf("找不到代理组"),
			}
		}

		msg := refreshMsg{
			gen:      gen,
			mode:     config.NormalizeMode(cfg.Mode),
			group:    group,
			autoTest: autoTest,
		}
		if providers, err := m.api.Providers(); err == nil {
			if p, ok := providers.Providers[config.ProviderName]; ok {
				msg.provider = p
			}
		}
		return msg
	}
}

func pollTraffic(m Model) tea.Cmd {
	gen := m.pollGen
	return func() tea.Msg {
		t, err := m.api.Traffic()
		if err != nil {
			return trafficMsg{gen: gen}
		}
		return trafficMsg{gen: gen, traffic: t}
	}
}

func reloadAndSyncMode(runner *core.Runner, client *api.Client, dataDir string) error {
	if err := runner.Reload(); err != nil {
		return fmt.Errorf("重载失败: %w", err)
	}
	if err := client.PatchMode(config.LoadMode(dataDir, "rule")); err != nil {
		return fmt.Errorf("模式同步失败: %w", err)
	}
	return nil
}

func (m *Model) syncSubscription() {
	subURL, err := config.LoadSubscriptionURL(m.paths.DataDir)
	if err != nil && m.err == "" {
		m.err = err.Error()
	}
	m.subscriptionURL = subURL
	m.hasSubscription = subURL != ""
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		w := m.width - 8
		if w < 20 {
			w = 20
		}
		if w > 120 {
			w = 120
		}
		m.linkInput.Width = w
		m.clampListScroll()
		m.clampLinkScroll()
		// Differential redraw leaves ghost cells when the terminal shrinks.
		return m, tea.ClearScreen

	case tickMsg:
		cmds := []tea.Cmd{tick()}
		if m.running && m.screen == screenMain && !m.work.busy() {
			m.tickN++
			cmds = append(cmds, pollTraffic(m))
			// Full node/mode/provider poll every ~6s; traffic stays at 2s.
			if m.tickN%3 == 1 {
				cmds = append(cmds, refresh(m, false))
			}
		}
		return m, tea.Batch(cmds...)

	case animTickMsg:
		m.tickAnim()
		if m.selectFlash > 0 {
			m.selectFlash--
			if m.selectFlash == 0 {
				m.selectFlashNode = ""
			}
		}
		return m, animTick()

	case trafficMsg:
		if msg.gen != m.pollGen || !m.running {
			return m, nil
		}
		m.traffic = msg.traffic
		return m, nil

	case refreshMsg:
		// Drop stale polls (e.g. in-flight before M mode switch) so optimistic UI sticks.
		if msg.gen != m.pollGen || !m.running || m.work.busy() {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.mode = msg.mode
		m.group = msg.group
		m.provider = msg.provider
		m.nodes = msg.group.All
		m.clampListScroll()

		var cmds []tea.Cmd
		if msg.autoTest && len(m.nodes) > 0 {
			var delayCmd tea.Cmd
			m, delayCmd = m.beginDelayTest()
			cmds = append(cmds, delayCmd)
		}
		return m, tea.Batch(cmds...)

	case delayOneMsg:
		if !m.running {
			return m, waitDelayResult(msg.more)
		}
		if m.delays == nil {
			m.delays = map[string]uint16{}
		}
		m.delays[msg.name] = msg.delay
		m.err = ""
		return m, waitDelayResult(msg.more)

	case delayDoneMsg:
		// Ignore stale completions after disconnect / a newer load started.
		if m.running && m.work == workTesting {
			m.work = workIdle
		}
		return m, nil

	case actionMsg:
		m.work = workIdle
		if msg.err != nil {
			m.err = msg.err.Error()
			m.selectFlash = 0
			m.selectFlashNode = ""
		} else {
			m.err = ""
			if m.selectFlashNode != "" {
				// API 成功：再爆一波确认。
				m.selectFlash = selectFlashFrames
			}
		}
		if msg.err != nil && !msg.added && !msg.deleted && m.screen == screenLinkList {
			m.linkInputFocus = true
			m.linkInput.Focus()
			return m, textinput.Blink
		}
		if msg.added || msg.deleted {
			urls, active, err := config.LoadSubscriptionLinks(m.paths.DataDir)
			if err != nil {
				m.err = err.Error()
			}
			m.linkURLs, m.linkActive = urls, active
			if len(urls) == 0 {
				m.linkCursor, m.linkRowOffset = 0, 0
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
		} else {
			m.linkInput.SetValue("")
		}
		m.syncSubscription()
		if msg.refresh && m.running {
			return m, refresh(m, msg.autoTest)
		}
		return m, nil

	case startMsg:
		if msg.err != nil {
			m.work = workIdle
			m.err = msg.err.Error()
			return m, nil
		}
		m.running, m.err = true, ""
		m = m.beginNodesLoad()
		return m, m.fetchNodesCmd()

	case clearDataMsg:
		return m.applyClearedData(msg), nil

	case autoConnectMsg:
		return m.beginConnect()

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m.quit()
		}
		if m.screen == screenLinkList {
			return m.updateLinkScreen(msg)
		}
		if m.screen == screenSettings {
			return m.updateSettingsScreen(msg)
		}
		return m.updateMain(msg)
	}
	return m, nil
}

// quit stops background work and the kernel, then exits the program.
func (m Model) quit() (tea.Model, tea.Cmd) {
	m.stopDelayTest()
	_ = m.runner.Stop()
	return m, tea.Quit
}

func (m Model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.quit()
	case "l":
		return m.openLinkScreen(), textinput.Blink
	case "p":
		return m.openSettingsScreen(), nil
	case "s":
		if m.work.busy() {
			return m, nil
		}
		if m.running {
			m.stopDelayTest()
			m.invalidatePolls()
			err := m.runner.Stop()
			m = m.resetIdleState()
			if err != nil {
				m.err = err.Error()
			}
			return m, nil
		}
		return m.beginConnect()
	case "t":
		if !m.running {
			m.err = "请先连接"
			return m, nil
		}
		if m.work.busy() {
			return m, nil
		}
		if len(m.nodes) == 0 {
			m.err = "暂无节点"
			return m, nil
		}
		return m.beginDelayTest()
	case "m":
		if !m.running {
			m.err = "请先连接"
			return m, nil
		}
		if m.work.busy() {
			return m, nil
		}
		m.invalidatePolls()
		m.work = workActing
		next := nextMode(m.mode)
		m.mode = next
		dataDir := m.paths.DataDir
		return m, func() tea.Msg {
			if err := config.SaveMode(dataDir, next); err != nil {
				return actionMsg{err: err}
			}
			if err := m.api.PatchMode(next); err != nil {
				return actionMsg{err: err}
			}
			// Mode already painted optimistically; skip refresh so a slow Configs
			// reply cannot flash the previous label (直连↔分流).
			return actionMsg{}
		}
	case "k", "up":
		m.moveCursor(-1)
		return m, nil
	case "j", "down":
		m.moveCursor(1)
		return m, nil
	case "enter":
		if !m.running || len(m.nodes) == 0 || m.work.busy() {
			return m, nil
		}
		node := m.nodes[m.cursor]
		m.work = workActing
		m.selectFlashNode = node
		m.selectFlash = selectFlashFrames
		return m, func() tea.Msg {
			if err := m.api.SelectProxy(api.DefaultProxyGroup, node); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{refresh: true}
		}
	}
	return m, nil
}

func (m Model) beginNodesLoad() Model {
	m.stopDelayTest()
	m.invalidatePolls()
	m.work = workLoadingNodes
	m.err = ""
	m.nodes = nil
	m.delays = map[string]uint16{}
	m.cursor, m.rowOffset = 0, 0
	return m
}

// resetIdleState restores the pre-connect UI, same as a fresh launch before Start.
func (m Model) resetIdleState() Model {
	m.stopDelayTest()
	m.invalidatePolls()
	m.running = false
	m.work = workIdle
	m.nodes = nil
	m.delays = map[string]uint16{}
	m.cursor, m.rowOffset = 0, 0
	m.mode = ""
	m.traffic = api.Traffic{}
	m.group = api.Proxy{}
	m.provider = api.ProxyProvider{}
	m.err = ""
	m.syncSubscription()
	return m
}

func (m Model) fetchNodesCmd() tea.Cmd {
	return func() tea.Msg {
		if err := m.api.UpdateProvider(config.ProviderName); err != nil {
			return actionMsg{err: err}
		}
		// GLOBAL may lag during provider warm-up; best-effort sync for global mode.
		_ = m.api.SyncGlobalFromProxyRetry()
		return actionMsg{refresh: true, autoTest: true}
	}
}

func (m Model) beginConnect() (Model, tea.Cmd) {
	if m.work.busy() || m.running {
		return m, nil
	}
	if m.subscriptionURL == "" {
		m.err = "按 l 添加订阅链接"
		return m, nil
	}
	if !core.TunBuildReady() {
		m.err = core.TunBuildHint()
		return m, nil
	}
	m.stopDelayTest()
	m.invalidatePolls()
	m.work = workConnecting
	m.err = ""
	m.nodes = nil
	m.delays = map[string]uint16{}
	m.cursor, m.rowOffset = 0, 0
	return m, func() tea.Msg {
		if err := m.runner.Start(); err != nil {
			return startMsg{err: err}
		}
		return startMsg{}
	}
}
