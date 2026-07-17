package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// cellWidth is the layout width we trust for padding/truncation.
func cellWidth(s string) int {
	return ansi.StringWidth(s)
}

// layoutString makes a string safe for fixed-width framing.
// Flag emoji become [XX] so box edges stay aligned across terminals.
func layoutString(s string) string {
	if s == "" {
		return s
	}
	return replaceFlags(s)
}

func replaceFlags(s string) string {
	runes := []rune(s)
	if len(runes) < 2 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(runes); i++ {
		if i+1 < len(runes) && isRegionalIndicator(runes[i]) && isRegionalIndicator(runes[i+1]) {
			b.WriteByte('[')
			b.WriteByte(regionalLetter(runes[i]))
			b.WriteByte(regionalLetter(runes[i+1]))
			b.WriteByte(']')
			i++
			continue
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

func isRegionalIndicator(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}

func regionalLetter(r rune) byte {
	return byte('A' + (r - 0x1F1E6))
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	s = layoutString(s)
	if cellWidth(s) <= max {
		return s
	}
	return ansi.Truncate(s, max, "…")
}

func pad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = layoutString(s)
	w := cellWidth(s)
	switch {
	case w == width:
		return s
	case w > width:
		return ansi.Truncate(s, width, "")
	default:
		return s + strings.Repeat(" ", width-w)
	}
}

func fitCells(s string, width int) string {
	return pad(truncate(s, width), width)
}
