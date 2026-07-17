package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPadExactWidth(t *testing.T) {
	if got := pad("ab", 5); got != "ab   " {
		t.Fatalf("pad short: %q", got)
	}
	if got := pad("abcdef", 4); lipgloss.Width(got) != 4 {
		t.Fatalf("pad over-wide width=%d got %q", lipgloss.Width(got), got)
	}
	if got := pad("hello", 5); got != "hello" {
		t.Fatalf("pad exact: %q", got)
	}
}

func TestDashedLeader(t *testing.T) {
	if got := dashedLeader(0); got != "" {
		t.Fatalf("gap0: %q", got)
	}
	if got := dashedLeader(2); got != "  " {
		t.Fatalf("gap2: %q", got)
	}
	got := dashedLeader(6)
	if lipgloss.Width(got) != 6 || !strings.Contains(got, "·") {
		t.Fatalf("gap6: %q", got)
	}
}
