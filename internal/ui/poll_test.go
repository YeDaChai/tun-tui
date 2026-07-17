package ui

import (
	"testing"

	"tun-tui/internal/api"
)

func TestRefreshMsg_DropsStalePollGen(t *testing.T) {
	m := Model{running: true, mode: "rule", pollGen: 2, work: workIdle}
	next, _ := m.Update(refreshMsg{gen: 1, mode: "direct"})
	got := next.(Model)
	if got.mode != "rule" {
		t.Fatalf("stale refresh overwrote mode: got %q", got.mode)
	}
}

func TestRefreshMsg_DropsWhileBusy(t *testing.T) {
	m := Model{running: true, mode: "rule", pollGen: 1, work: workActing}
	next, _ := m.Update(refreshMsg{gen: 1, mode: "direct"})
	got := next.(Model)
	if got.mode != "rule" {
		t.Fatalf("busy refresh overwrote mode: got %q", got.mode)
	}
}

func TestTrafficMsg_AppliesMatchingGen(t *testing.T) {
	m := Model{running: true, pollGen: 3}
	next, _ := m.Update(trafficMsg{gen: 3, traffic: api.Traffic{Up: 10, Down: 20}})
	got := next.(Model)
	if got.traffic.Up != 10 || got.traffic.Down != 20 {
		t.Fatalf("traffic not applied: %+v", got.traffic)
	}
	next, _ = got.Update(trafficMsg{gen: 2, traffic: api.Traffic{Up: 99}})
	got = next.(Model)
	if got.traffic.Up != 10 {
		t.Fatalf("stale traffic applied: %+v", got.traffic)
	}
}
