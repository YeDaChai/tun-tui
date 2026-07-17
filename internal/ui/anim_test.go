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

func TestSelectBurstMarkWidth(t *testing.T) {
	m := Model{}
	for flash := 1; flash <= selectFlashFrames; flash++ {
		m.selectFlash = flash
		got := m.selectBurstMark()
		if cellWidth(got) != 2 {
			t.Fatalf("flash=%d mark %q width=%d want 2", flash, got, cellWidth(got))
		}
		tail := selectBurstTail(flash)
		if cellWidth(tail) != 3 {
			t.Fatalf("flash=%d tail %q width=%d want 3", flash, tail, cellWidth(tail))
		}
	}
}

func TestMistLeaderWidth(t *testing.T) {
	for _, gap := range []int{0, 1, 2, 5, 20} {
		a := dashedLeader(gap)
		b := mistLeader(gap, 3)
		if cellWidth(a) != gap || cellWidth(b) != gap {
			t.Fatalf("gap=%d a=%d b=%d", gap, cellWidth(a), cellWidth(b))
		}
	}
}

func TestTrafficBarWidth(t *testing.T) {
	idle := trafficBar(0, txColor)
	busy := trafficBar(1<<20, rxColor)
	if cellWidth(idle) != trafficBarWidth+2 { // [####]
		t.Fatalf("idle width=%d", cellWidth(idle))
	}
	if cellWidth(busy) != trafficBarWidth+2 {
		t.Fatalf("busy width=%d", cellWidth(busy))
	}
	if energyFill(0, trafficBarWidth) != 0 {
		t.Fatalf("idle fill")
	}
	if energyFill(1, trafficBarWidth) < 1 {
		t.Fatalf("any traffic lights a cell")
	}
}
