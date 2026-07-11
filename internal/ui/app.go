package ui

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
type startMsg struct{ err error }
type autoConnectMsg struct{}

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

func Run(ctx context.Context, paths config.Paths, runner *core.Runner, client *api.Client, appVersion, binName string) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("需要在交互式终端中运行，不能从 IDE 的 Run/Debug 按钮直接启动\n请在终端执行:\n  %s", binName)
	}
	_, err := tea.NewProgram(
		New(paths, runner, client, appVersion),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
	).Run()
	return err
}

func New(paths config.Paths, runner *core.Runner, client *api.Client, appVersion string) Model {
	subURL, err := config.LoadSubscriptionURL(paths.DataDir)
	status := "按 l 管理订阅链接"
	if err != nil {
		status = "读取订阅失败"
	} else if subURL != "" {
		status = "自动连接中…"
	}

	ti := textinput.New()
	ti.Placeholder = "https://your-subscription-url"
	ti.CharLimit = 2048
	ti.Width = 64
	ti.Prompt = "链接: "

	m := Model{
		paths:           paths,
		runner:          runner,
		api:             client,
		appVersion:      appVersion,
		linkInput:       ti,
		linkActive:      -1,
		subscriptionURL: subURL,
		activeGroup:     api.DefaultProxyGroup,
		nodes:           []string{},
		delays:          map[string]uint16{},
		status:          status,
		hasSubscription: subURL != "",
	}
	if err != nil {
		m.err = err.Error()
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick(), textinput.Blink}
	if m.hasSubscription {
		cmds = append(cmds, func() tea.Msg { return autoConnectMsg{} })
	}
	return tea.Batch(cmds...)
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
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
		traffic, _ := m.api.Traffic()
		proxies, err := m.api.Proxies()
		if err != nil {
			return refreshMsg{err: err}
		}

		group, ok := proxies.Proxies[m.activeGroup]
		if !ok {
			group, ok = proxies.Proxies[api.DefaultProxyGroup]
		}
		if !ok {
			return refreshMsg{
				version: version.Version,
				mode:    config.NormalizeMode(cfg.Mode),
				traffic: traffic,
				err:     fmt.Errorf("找不到代理组"),
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
		if providers, err := m.api.Providers(); err == nil {
			if p, ok := providers.Providers[config.ProviderName]; ok {
				msg.provider = p
			}
		}
		return msg
	}
}

func reloadAndSyncMode(runner *core.Runner, client *api.Client, dataDir string) (string, error) {
	if err := runner.Reload(); err != nil {
		return "重载失败", err
	}
	if err := client.PatchMode(config.LoadMode(dataDir, "rule")); err != nil {
		return "模式同步失败", err
	}
	return "", nil
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
		if config.NormalizeMode(m.mode) == "global" {
			return m, func() tea.Msg {
				_ = m.api.SyncGlobalFromProxy()
				return nil
			}
		}
		return m, nil

	case delayMsg:
		if msg.err != nil {
			m.status, m.err = "测速失败", msg.err.Error()
			return m, nil
		}
		m.delays, m.status, m.err = msg.delays, "测速完成", ""
		return m, nil

	case linkMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.err = ""
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
			m.syncSubscription()
			switch {
			case msg.status != "":
				m.status = msg.status
			case msg.added:
				m.status = "已添加链接"
			default:
				m.status = "已删除链接"
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
		m.syncSubscription()
		m.linkInput.SetValue("")
		if msg.refresh {
			return m, refresh(m)
		}
		return m, nil

	case startMsg:
		m.starting = false
		m.running = m.runner.Running()
		if msg.err != nil {
			m.err, m.status = msg.err.Error(), "连接失败"
			return m, nil
		}
		m.running, m.status, m.err = true, "", ""
		return m, tea.Batch(refresh(m), func() tea.Msg {
			if config.NormalizeMode(config.LoadMode(m.paths.DataDir, "rule")) == "global" ||
				config.NormalizeMode(m.mode) == "global" {
				_ = m.api.SyncGlobalFromProxy()
			}
			return nil
		})

	case autoConnectMsg:
		return m.beginConnect()

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
			m.status = "已断开"
			m.version = ""
			m.nodes = nil
			m.provider = api.ProxyProvider{}
			m.delays = map[string]uint16{}
			m.rowOffset, m.cursor = 0, 0
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
			m.status = "请先连接"
			return m, nil
		}
		m.status = "重载中…"
		return m, func() tea.Msg {
			status, err := reloadAndSyncMode(m.runner, m.api, m.paths.DataDir)
			if err != nil {
				return actionMsg{status: status, err: err}
			}
			return actionMsg{status: "已重载", refresh: true}
		}
	case "u":
		if !m.running {
			m.status = "请先连接"
			return m, nil
		}
		m.status = "更新中…"
		return m, func() tea.Msg {
			if err := m.api.UpdateProvider(config.ProviderName); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{status: "订阅已更新", refresh: true}
		}
	case "t":
		if !m.running {
			m.status = "请先连接"
			return m, nil
		}
		m.status = "测速中…"
		return m, func() tea.Msg {
			delays, err := m.api.GroupDelay(m.activeGroup, testURL)
			return delayMsg{delays: delays, err: err}
		}
	case "m":
		if !m.running {
			m.status = "请先连接"
			return m, nil
		}
		next := nextMode(m.mode)
		m.mode, m.status = next, ""
		dataDir := m.paths.DataDir
		return m, func() tea.Msg {
			if err := config.SaveMode(dataDir, next); err != nil {
				return actionMsg{err: err}
			}
			if err := m.api.PatchMode(next); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{refresh: true}
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
			if err := m.api.SelectProxy(m.activeGroup, node); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{status: "已切换 → " + node, refresh: true}
		}
	}
	return m, nil
}

func (m Model) beginConnect() (Model, tea.Cmd) {
	if m.starting || m.running {
		return m, nil
	}
	if m.subscriptionURL == "" {
		m.status, m.err = "连接失败", "按 l 添加订阅链接"
		return m, nil
	}
	if !core.TunBuildReady() {
		m.status, m.err = "连接失败", core.TunBuildHint()
		return m, nil
	}
	m.starting, m.status, m.err = true, "连接中…", ""
	return m, func() tea.Msg {
		if err := m.runner.Start(); err != nil {
			return startMsg{err: err}
		}
		return startMsg{}
	}
}
