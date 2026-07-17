package ui

import "testing"

func TestEnergyFill(t *testing.T) {
	if energyFill(0, 8) != 0 {
		t.Fatalf("idle should be empty")
	}
	if energyFill(1, 8) < 1 {
		t.Fatalf("any traffic should light at least one cell")
	}
	full := energyFill(1<<20, 8)
	if full < 7 {
		t.Fatalf("1MiB/s should nearly fill, got %d", full)
	}
	if energyFill(1<<30, 8) != 8 {
		t.Fatalf("huge rate should clamp to width")
	}
}

func TestRatioFill(t *testing.T) {
	if ratioFill(0, 100, 10) != 0 {
		t.Fatalf("empty quota")
	}
	if ratioFill(50, 100, 10) != 5 {
		t.Fatalf("half quota")
	}
	if ratioFill(200, 100, 10) != 10 {
		t.Fatalf("over-quota clamps")
	}
}
