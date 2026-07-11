package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"tun-tui/internal/config"
)

func renderUsageBar(used, total int64, width int) string {
	if total <= 0 || width < 8 {
		return ""
	}
	ratio := float64(used) / float64(total)
	if ratio > 1 {
		ratio = 1
	}
	label := fmt.Sprintf(" %s/%s ", formatTraffic(used), formatTraffic(total))
	barWidth := width - visualWidth(label) - 2
	if barWidth < 4 {
		barWidth = 4
	}

	filled := int(ratio * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}

	var fillStyle lipgloss.Style
	switch {
	case ratio > 0.9:
		fillStyle = barDanger
	case ratio > 0.7:
		fillStyle = barWarning
	default:
		fillStyle = barFull
	}

	s := fillStyle.Render(strings.Repeat("█", filled))
	s += barEmpty.Render(strings.Repeat("░", barWidth-filled))
	s += textSubtle.Render(label)
	return s
}

func formatRate(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB/s", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB/s", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B/s", n)
	}
}

func formatTraffic(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func maskURL(raw string) string {
	if raw == "" {
		return "-"
	}
	runes := []rune(raw)
	if len(runes) <= 24 {
		return raw
	}
	return string(runes[:16]) + "..." + string(runes[len(runes)-8:])
}

func modeLabel(mode string) string {
	switch config.NormalizeMode(mode) {
	case "global":
		return "全局"
	case "direct":
		return "直连"
	case "rule":
		return "分流"
	default:
		if mode == "" {
			return "分流"
		}
		return mode
	}
}

func nextMode(current string) string {
	switch config.NormalizeMode(current) {
	case "rule":
		return "global"
	case "global":
		return "direct"
	case "direct":
		return "rule"
	default:
		return "rule"
	}
}
