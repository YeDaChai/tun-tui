package config

import "testing"

func TestNormalizeMode(t *testing.T) {
	cases := map[string]string{
		"rule": "rule", "RULE": "rule", " global ": "global",
		"direct": "direct", "": "", "weird": "",
	}
	for in, want := range cases {
		if got := NormalizeMode(in); got != want {
			t.Fatalf("NormalizeMode(%q)=%q want %q", in, got, want)
		}
	}
}

func TestParseSubscriptionLinks(t *testing.T) {
	urls, active, err := parseSubscriptionLinks([]byte(
		"# comment\nhttps://a.example/sub\n*https://b.example/sub\nhttps://c.example/sub\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 3 || active != 1 || urls[1] != "https://b.example/sub" {
		t.Fatalf("got urls=%v active=%d", urls, active)
	}
}

func TestValidateSubscriptionURL(t *testing.T) {
	if err := ValidateSubscriptionURL("https://example.com/sub"); err != nil {
		t.Fatalf("public url: %v", err)
	}
	for _, bad := range []string{
		"", "ftp://x", "https://localhost/x", "http://127.0.0.1/x", "http://192.168.1.1/x",
	} {
		if err := ValidateSubscriptionURL(bad); err == nil {
			t.Fatalf("expected reject for %q", bad)
		}
	}
}
