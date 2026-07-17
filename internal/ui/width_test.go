package ui

import (
	"strings"
	"testing"
)

func TestScrollPlaque(t *testing.T) {
	got := scrollPlaque("节点")
	if !strings.HasPrefix(got, "~.*=") || !strings.HasSuffix(got, "=*.~") {
		t.Fatalf("plaque=%q", got)
	}
}

func TestMistSepWidth(t *testing.T) {
	for _, w := range []int{4, 8, 40} {
		s := mistSep(w)
		if cellWidth(s) != w {
			t.Fatalf("width %d: got %d for %q", w, cellWidth(s), s)
		}
	}
}

func TestReplaceFlags(t *testing.T) {
	out := replaceFlags("🇭🇰 香港08")
	if out != "[HK] 香港08" {
		t.Fatalf("got %q", out)
	}
	if cellWidth(out) != 11 {
		t.Fatalf("width=%d want 11 for %q", cellWidth(out), out)
	}
}

func TestPadStableWithFlags(t *testing.T) {
	w := 40
	a := pad("DIRECT", w)
	b := pad("🇯🇵 JP1-低倍率", w)
	if cellWidth(a) != w || cellWidth(b) != w {
		t.Fatalf("pad widths a=%d b=%d want %d", cellWidth(a), cellWidth(b), w)
	}
	if !strings.HasPrefix(strings.TrimRight(b, " "), "[JP]") {
		t.Fatalf("expected flag replaced: %q", b)
	}
}

func TestFrameTitleRowAligned(t *testing.T) {
	width := 80
	f := newFrame(width, true)
	top := f.top()
	bot := f.bottom()
	title := f.row(sectionTitle.Render(fitCells(layoutString(scrollPlaque("节点 1/48 | 香港08")), width-2)))
	if cellWidth(top) != width || cellWidth(bot) != width || cellWidth(title) != width {
		t.Fatalf("top=%d bot=%d title=%d want %d", cellWidth(top), cellWidth(bot), cellWidth(title), width)
	}
}

func TestFlexRowSpaceBetween(t *testing.T) {
	items := []string{"[A]", "[BB]", "[C]"}
	w := 20
	got := flexRow(items, w)
	if cellWidth(got) != w {
		t.Fatalf("width=%d want %d: %q", cellWidth(got), w, got)
	}
	if !strings.HasPrefix(got, "[A]") || !strings.HasSuffix(got, "[C]") {
		t.Fatalf("not space-between: %q", got)
	}
}
