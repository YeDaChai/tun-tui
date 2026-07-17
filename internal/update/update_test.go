package update

import (
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

func TestPlatformArchive_AllReleaseTargets(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "tun-tui-0.2.3-macos-apple-silicon-arm64.tar.gz"},
		{"darwin", "amd64", "tun-tui-0.2.3-macos-intel-x86_64.tar.gz"},
		{"linux", "amd64", "tun-tui-0.2.3-linux-x86_64.tar.gz"},
		{"linux", "arm64", "tun-tui-0.2.3-linux-arm64.tar.gz"},
		{"windows", "amd64", "tun-tui-0.2.3-windows-x86_64.zip"},
	}
	for _, c := range cases {
		got, err := platformArchive("0.2.3", c.goos, c.goarch)
		if err != nil {
			t.Fatalf("%s/%s: %v", c.goos, c.goarch, err)
		}
		if got != c.want {
			t.Fatalf("%s/%s: got %s want %s", c.goos, c.goarch, got, c.want)
		}
	}
}

func TestArchiveCandidates_IncludesLegacyMacNames(t *testing.T) {
	cands, err := archiveCandidates("0.2.2", "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if cands[0] != "tun-tui-0.2.2-macos-apple-silicon-arm64.tar.gz" {
		t.Fatalf("primary: %s", cands[0])
	}
	if len(cands) < 2 || cands[1] != "tun-tui-0.2.2-macos-apple-silicon.tar.gz" {
		t.Fatalf("missing legacy alias: %v", cands)
	}
}

func TestPickAsset_ExactAndLegacyAndSuffix(t *testing.T) {
	rel := Release{
		TagName: "v0.2.2",
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}{
			{Name: "tun-tui-0.2.2-macos-apple-silicon.tar.gz", BrowserDownloadURL: "https://example/legacy"},
			{Name: "tun-tui-0.2.2-linux-x86_64.tar.gz", BrowserDownloadURL: "https://example/linux"},
			{Name: "weird-prefix-0.2.2-windows-x86_64.zip", BrowserDownloadURL: "https://example/win"},
		},
	}

	name, url, err := pickAsset(rel, "0.2.2", "darwin", "arm64")
	if err != nil || name != "tun-tui-0.2.2-macos-apple-silicon.tar.gz" || url != "https://example/legacy" {
		t.Fatalf("legacy mac: name=%s url=%s err=%v", name, url, err)
	}

	name, url, err = pickAsset(rel, "0.2.2", "linux", "amd64")
	if err != nil || name != "tun-tui-0.2.2-linux-x86_64.tar.gz" {
		t.Fatalf("linux exact: name=%s err=%v", name, err)
	}

	name, url, err = pickAsset(rel, "0.2.2", "windows", "amd64")
	if err != nil || name != "weird-prefix-0.2.2-windows-x86_64.zip" || url != "https://example/win" {
		t.Fatalf("windows suffix: name=%s url=%s err=%v", name, url, err)
	}
}

func TestPlatformLabel_Unsupported(t *testing.T) {
	if _, _, err := platformLabel("windows", "arm64"); err == nil {
		t.Fatal("expected windows/arm64 unsupported")
	}
	if _, _, err := platformLabel("freebsd", "amd64"); err == nil {
		t.Fatal("expected freebsd unsupported")
	}
}
