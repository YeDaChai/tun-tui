package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"tun-tui/internal/api"
	"tun-tui/internal/config"
	"tun-tui/internal/core"
)

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick(), textinput.Blink}
	if m.hasSubscription {
		cmds = append(cmds, autoConnect())
	}
	return tea.Batch(cmds...)
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
			m.tickCount++
			cmds = append(cmds, fetchTraffic(m))
			if m.tickCount%heavyRefreshEvery == 0 {
				cmds = append(cmds, refresh(m))
			}
		}
		return m, tea.Batch(cmds...)

	case trafficMsg:
		if msg.err != nil {
			return m, nil
		}
		m.traffic = msg.traffic
		return m, nil

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
		m.group = msg.group
		m.proxyGroups = msg.groups
		m.provider = msg.provider
		m.hasSubscription = msg.hasSubscription
		m.nodes = msg.group.All
		m.clampListScroll()
		if config.NormalizeMode(m.mode) == "global" {
			return m, syncGlobalQuiet(m)
		}
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
			subURL, subErr := config.LoadSubscriptionURL(m.paths.DataDir)
			if subErr != nil && m.err == "" {
				m.err = subErr.Error()
			}
			m.subscriptionURL = subURL
			m.hasSubscription = subURL != ""
			if msg.status != "" {
				m.status = msg.status
			} else if msg.added {
				m.status = "已添加链接"
			} else {
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
		subURL, subErr := config.LoadSubscriptionURL(m.paths.DataDir)
		if subErr != nil && m.err == "" {
			m.err = subErr.Error()
		}
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
			m.status = "连接失败"
			return m, nil
		}
		m.running = true
		m.status = ""
		m.err = ""
		m.tickCount = 0
		return m, tea.Batch(refresh(m), fetchTraffic(m), syncGlobalQuiet(m))

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
			m.rowOffset = 0
			m.cursor = 0
			m.tickCount = 0
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
			err := m.api.UpdateProvider(config.ProviderName)
			if err != nil {
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
		return m, testDelay(m)

	case "m":
		if !m.running {
			m.status = "请先连接"
			return m, nil
		}
		next := nextMode(m.mode)
		m.mode = next
		m.status = ""
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
			err := m.api.SelectProxy(m.activeGroup, node)
			if err != nil {
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
		m.status = "连接失败"
		m.err = "按 l 添加订阅链接"
		return m, nil
	}
	if !core.TunBuildReady() {
		m.status = "连接失败"
		m.err = core.TunBuildHint()
		return m, nil
	}
	m.starting = true
	m.status = "连接中…"
	m.err = ""
	return m, m.startRunnerCmd()
}
