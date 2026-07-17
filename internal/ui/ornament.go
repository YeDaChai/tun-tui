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
