package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// frame draws horizontal rules with full-width content rows.
type frame struct {
	width  int
	border lipgloss.Style
}

func newFrame(width int) frame {
	return frame{width: width, border: dividerStyle}
}

func newPanelFrame(width int, active bool) frame {
	border := frameBorderInactive
	if active {
		border = frameBorderActive
	}
	return frame{width: width, border: border}
}

func (f frame) top() string {
	return f.border.Render(strings.Repeat("─", f.width))
}

func (f frame) bottom() string {
	return f.border.Render(strings.Repeat("─", f.width))
}

func (f frame) row(content string) string {
	content = truncateVisual(content, f.width)
	return padVisual(content, f.width)
}

func (f frame) rowSpacedLeft(left string) string {
	return f.row(left)
}

func (f frame) rowSpacedSplit(left, right string) string {
	inner := f.width
	leftW := visualWidth(left)
	rightW := visualWidth(right)
	if rightW > 0 {
		gap := inner - leftW - rightW
		if gap < 1 {
			gap = 1
			maxLeft := inner - rightW - gap
			if maxLeft < 0 {
				maxLeft = 0
			}
			left = truncateVisual(left, maxLeft)
		}
		content := left + strings.Repeat(" ", gap) + right
		return padVisual(content, inner)
	}
	return f.row(left)
}

func visualWidth(s string) int {
	return lipgloss.Width(s)
}

func padVisual(s string, width int) string {
	w := visualWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func truncateVisual(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if visualWidth(s) <= max {
		return s
	}
	return ansi.Truncate(s, max, "…")
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

func frameInner(frameW int) int {
	if frameW < 0 {
		return 0
	}
	return frameW
}

func buildPlainRow(width int, mark, name, delay string) string {
	markW := visualWidth(mark)
	delayW := visualWidth(delay)

	nameMax := width - markW
	if delay != "" {
		nameMax -= delayW
	}
	if nameMax < 1 {
		nameMax = 1
	}

	prefix := mark + truncateVisual(name, nameMax)
	if delay == "" {
		return padVisual(prefix, width)
	}

	gap := width - visualWidth(prefix) - delayW
	if gap < 0 {
		gap = 0
	}
	return padVisual(prefix+strings.Repeat(" ", gap)+delay, width)
}

func buildRow(width int, mark, name, delay string, rowStyle, delayStyle lipgloss.Style, fullRow bool) string {
	plain := buildPlainRow(width, mark, name, delay)
	if fullRow || delay == "" {
		return rowStyle.Render(plain)
	}
	prefix := strings.TrimSuffix(plain, delay)
	return rowStyle.Render(prefix) + delayStyle.Render(delay)
}
