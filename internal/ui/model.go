package ui

import (
	"github.com/charmbracelet/bubbles/textinput"

	"tun-tui/internal/api"
	"tun-tui/internal/config"
	"tun-tui/internal/core"
)

const (
	testURL          = "https://www.gstatic.com/generate_204"
	heavyRefreshEvery = 3 // full state refresh every N ticks
)

type screen int

const (
	screenMain screen = iota
	screenLinkList
)

type tickMsg struct{}
type trafficMsg struct {
	traffic api.Traffic
	err     error
}
type refreshMsg struct {
	version         string
	mode            string
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
	tickCount       int
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
	ti.SetValue("")

	m := Model{
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
	if err != nil {
		m.err = err.Error()
	}
	return m
}
