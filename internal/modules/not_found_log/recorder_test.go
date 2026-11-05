package notfoundlog

import (
	"context"
	"testing"
)

// TestGlobRegexp - * is any characters, the rest is literal, letter case
// aside, and the pattern covers the whole path.
func TestGlobRegexp(t *testing.T) {
	for _, tc := range []struct {
		pattern, path string
		want          bool
	}{
		{"*.php", "/index.php", true},
		{"*.php", "/a/b/INSTALL.PHP", true},
		{"*.php", "/index.php5", false},
		{"/wp-*", "/wp-login.php", true},
		{"/wp-*", "/news/wp-login", false},
		{"/.env*", "/.env.local", true},
		{"/a.b", "/axb", false},
		{"/cgi-bin/*", "/cgi-bin/x", true},
		{"/cgi-bin/*", "/cgi-bin", false},
	} {
		if got := globRegexp(tc.pattern).MatchString(tc.path); got != tc.want {
			t.Errorf("%q ~ %q = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

// TestIgnored - the saved patterns, compiled once, decide; the defaults
// keep the scanners' paths out.
func TestIgnored(t *testing.T) {
	r := &Recorder{}
	r.remember(defaultSettings())
	ctx := context.Background()
	for path, want := range map[string]bool{
		"/wp-admin/setup-config.php": true,
		"/.git/config":               true,
		"/xmlrpc.php":                true,
		"/news/old-article":          false,
		"/heroes/spider-man":         false,
	} {
		if got := r.Ignored(ctx, path); got != want {
			t.Errorf("%s: %v, want %v", path, got, want)
		}
	}
	r.remember(Settings{Ignore: []string{"/old/*"}})
	if !r.Ignored(ctx, "/old/page") || r.Ignored(ctx, "/wp-login.php") {
		t.Error("the saved patterns replace the defaults")
	}
}

// TestPatternLines - one pattern per non-empty line, trimmed.
func TestPatternLines(t *testing.T) {
	got := patternLines(" /wp-*\r\n\n  *.php \n")
	if len(got) != 2 || got[0] != "/wp-*" || got[1] != "*.php" {
		t.Errorf("%q", got)
	}
}
