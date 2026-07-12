package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// overlayCenter draws fg centered on top of bg without wiping the background.
func overlayCenter(bg, fg string, width, height int) string {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = lineCount(bg)
		if height < 1 {
			height = 1
		}
	}

	bgLines := fitLines(bg, width, height)
	fg = strings.TrimRight(fg, "\n")
	fgLines := strings.Split(fg, "\n")
	if len(fgLines) == 0 {
		return strings.Join(bgLines, "\n")
	}

	fgH := len(fgLines)
	fgW := 0
	for _, line := range fgLines {
		if w := lipgloss.Width(line); w > fgW {
			fgW = w
		}
	}
	if fgW > width {
		fgW = width
	}
	if fgH > height {
		fgH = height
		fgLines = fgLines[:fgH]
	}

	startY := (height - fgH) / 2
	if startY < 0 {
		startY = 0
	}
	startX := (width - fgW) / 2
	if startX < 0 {
		startX = 0
	}

	for i := 0; i < fgH; i++ {
		y := startY + i
		if y >= height {
			break
		}
		line := fgLines[i]
		if lipgloss.Width(line) > fgW {
			line = ansi.Truncate(line, fgW, "")
		}
		bgLines[y] = spliceLine(bgLines[y], line, startX, width)
	}
	return strings.Join(bgLines, "\n")
}

func fitLines(s string, width, height int) []string {
	raw := strings.Split(strings.TrimRight(s, "\n"), "\n")
	out := make([]string, height)
	for i := 0; i < height; i++ {
		line := ""
		if i < len(raw) {
			line = raw[i]
		}
		out[i] = pad(truncate(line, width), width)
	}
	return out
}

func spliceLine(bg, fg string, x, width int) string {
	if x < 0 {
		x = 0
	}
	if x >= width {
		return pad(truncate(bg, width), width)
	}
	fgW := lipgloss.Width(fg)
	if x+fgW > width {
		fg = ansi.Truncate(fg, width-x, "")
		fgW = lipgloss.Width(fg)
	}
	left := ""
	if x > 0 {
		left = ansi.Cut(bg, 0, x)
	}
	right := ""
	end := x + fgW
	if end < width {
		right = ansi.Cut(bg, end, width)
	}
	return pad(left+fg+right, width)
}
