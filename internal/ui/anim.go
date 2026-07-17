package ui

import (
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type animTickMsg struct{}

const animInterval = 80 * time.Millisecond

// Classic ASCII spinner — width-1 glyphs keep box edges aligned.
var spinnerFrames = []string{"|", "/", "-", "\\"}

func animTick() tea.Cmd {
	return tea.Tick(animInterval, func(time.Time) tea.Msg { return animTickMsg{} })
}

func (m *Model) tickAnim() { m.anim++ }

func (m Model) spinnerGlyph() string {
	return spinnerFrames[m.anim%len(spinnerFrames)]
}

func (m Model) cursorMark(active bool) string {
	if !active {
		return "  "
	}
	if m.anim%2 == 0 {
		return "> "
	}
	return "* "
}

// selectFlashFrames ≈ 1s @ 80ms tick：回车确认选节点的爆发时长。
const selectFlashFrames = 12

func (m Model) selectBursting(node string) bool {
	return m.selectFlash > 0 && node != "" && node == m.selectFlashNode
}

// selectBurstMark：虚影放大再收束到落定星标。
func (m Model) selectBurstMark() string {
	switch {
	case m.selectFlash >= 10:
		return ">>"
	case m.selectFlash >= 7:
		return "~>"
	case m.selectFlash >= 4:
		return "*>"
	case m.selectFlash >= 2:
		return "*~"
	default:
		return "* "
	}
}

// energyFill：约 1 MiB/s 满格（测试 / 预留进度条用）。
func energyFill(rate int64, width int) int {
	if width <= 0 || rate <= 0 {
		return 0
	}
	level := math.Log10(float64(rate)+1) / math.Log10(float64(1<<20)+1)
	if level > 1 {
		level = 1
	}
	n := int(math.Round(level * float64(width)))
	if n < 1 && rate > 0 {
		n = 1
	}
	if n > width {
		n = width
	}
	return n
}

func ratioFill(used, total int64, width int) int {
	if width <= 0 || total <= 0 {
		return 0
	}
	n := int(math.Round(float64(used) / float64(total) * float64(width)))
	if n < 0 {
		n = 0
	}
	if n > width {
		n = width
	}
	return n
}
