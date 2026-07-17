package ui

import (
	"strings"
	"testing"
)

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
	title := f.row(sectionTitle.Render(fitCells(layoutString("*= 节点 1/48 | 香港08 =*"), width-2)))
	if cellWidth(top) != width || cellWidth(bot) != width || cellWidth(title) != width {
		t.Fatalf("top=%d bot=%d title=%d want %d", cellWidth(top), cellWidth(bot), cellWidth(title), width)
	}
}
