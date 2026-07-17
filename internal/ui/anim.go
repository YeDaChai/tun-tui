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
