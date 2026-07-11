package ui

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"tun-tui/internal/api"
	"tun-tui/internal/config"
	"tun-tui/internal/core"
)

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func autoConnect() tea.Cmd {
	return func() tea.Msg { return autoConnectMsg{} }
}

func fetchTraffic(m Model) tea.Cmd {
	return func() tea.Msg {
		if !m.running {
			return trafficMsg{err: fmt.Errorf("内核未运行")}
		}
		traffic, err := m.api.Traffic()
		return trafficMsg{traffic: traffic, err: err}
	}
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
		sort.Strings(groups)

		group, ok := proxies.Proxies[m.activeGroup]
		if !ok {
			if len(groups) > 0 {
				group = proxies.Proxies[groups[0]]
			} else {
				return refreshMsg{
					version: version.Version,
					mode:    config.NormalizeMode(cfg.Mode),
					err:     fmt.Errorf("no proxy groups found"),
				}
			}
		}

		subURL, _ := config.LoadSubscriptionURL(m.paths.DataDir)
		msg := refreshMsg{
			version:         version.Version,
			mode:            config.NormalizeMode(cfg.Mode),
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

// reloadAndSyncMode reloads mihomo config and restores the persisted mode.
func reloadAndSyncMode(runner *core.Runner, client *api.Client, dataDir string) (status string, err error) {
	if err := runner.Reload(); err != nil {
		return "重载失败", err
	}
	mode := config.LoadMode(dataDir, "rule")
	if err := client.PatchMode(mode); err != nil {
		return "模式同步失败", err
	}
	return "", nil
}

func (m Model) startRunnerCmd() tea.Cmd {
	return func() tea.Msg {
		if err := m.runner.Start(); err != nil {
			return startMsg{err: err}
		}
		return startMsg{}
	}
}

func syncGlobalQuiet(m Model) tea.Cmd {
	return func() tea.Msg {
		if config.NormalizeMode(config.LoadMode(m.paths.DataDir, "rule")) != "global" &&
			config.NormalizeMode(m.mode) != "global" {
			return nil
		}
		_ = m.api.SyncGlobalFromProxy()
		return nil
	}
}
