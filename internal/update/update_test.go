package update

import (
	"runtime"
	"testing"
)

func TestNormalize(t *testing.T) {
	if got := Normalize(" v1.2.3 "); got != "1.2.3" {
		t.Fatalf("got %q", got)
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.2.3", "0.2.2", true},
		{"v0.2.2", "0.2.2", false},
		{"0.2.2", "0.2.3", false},
		{"0.3.0", "0.2.9", true},
		{"1.0.0", "dev", true},
		{"dev", "0.2.2", false},
		{"", "0.2.2", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.latest, c.current); got != c.want {
			t.Fatalf("IsNewer(%q,%q)=%v want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestPlatformArchive(t *testing.T) {
	name, err := PlatformArchive("0.2.2")
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		if name != "tun-tui-0.2.2-macos-apple-silicon-arm64.tar.gz" {
			t.Fatalf("got %s", name)
		}
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		if name != "tun-tui-0.2.2-macos-intel-x86_64.tar.gz" {
			t.Fatalf("got %s", name)
		}
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		if name != "tun-tui-0.2.2-linux-x86_64.tar.gz" {
			t.Fatalf("got %s", name)
		}
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		if name != "tun-tui-0.2.2-windows-x86_64.zip" {
			t.Fatalf("got %s", name)
		}
	default:
		t.Logf("platform archive: %s", name)
	}
}
