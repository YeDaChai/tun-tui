package ui

import "strings"

// 国风微饰：云纹、卷轴匾额。全部半角，避免撑破边框。

func scrollPlaque(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "~.*=*~."
	}
	return "~.*= " + title + " =*.~"
}

// mistSep is a static cloud-mist divider for inner content width.
func mistSep(width int) string {
	if width <= 0 {
		return ""
	}
	if width < 4 {
		return textSubtle.Render(strings.Repeat("~", width))
	}
	inner := width - 2
	var b strings.Builder
	b.WriteByte('~')
	for i := 0; i < inner; i++ {
		if i%2 == 0 {
			b.WriteByte('-')
		} else {
			b.WriteByte('~')
		}
	}
	b.WriteByte('~')
	return textSubtle.Render(b.String())
}

func mistBreath(frame int) string {
	switch frame % 4 {
	case 0:
		return ". ~ . ~~ . ~ ."
	case 1:
		return "~ . ~~ . ~ . ~"
	case 2:
		return ". ~~ . ~ . ~~ ."
	default:
		return "~ . ~ . ~~ . ~"
	}
}

func splashLines(width int) []string {
	art := []string{
		"      .~~.      .~~.",
		"   .~~'  '~~..~~'  '~~.",
		"  ~                    ~",
		"   '~~.    tun-tui   .~~'",
		"      '~~.        .~~'",
		"         '~~~~~~'",
		"",
		scrollPlaque("ready"),
	}
	out := make([]string, len(art))
	for i, line := range art {
		style := sectionTitle
		if i == len(art)-1 {
			style = textSubtle
		}
		out[i] = style.Render(centerText(line, width))
	}
	return out
}

func (m Model) viewSplash() string {
	w := m.contentWidth()
	h := m.height
	if h <= 0 {
		h = 24
	}
	lines := make([]string, h)
	for i := range lines {
		lines[i] = pad("", w)
	}
	art := splashLines(w)
	start := (h - len(art)) / 2
	if start < 0 {
		start = 0
	}
	for i, line := range art {
		y := start + i
		if y >= h {
			break
		}
		lines[y] = fitCells(line, w)
	}
	// 启动页仅保留这一处轻呼吸。
	if h > 2 {
		lines[h-2] = fitCells(textSubtle.Render(centerText(mistBreath(m.anim), w)), w)
	}
	return strings.Join(lines, "\n")
}
