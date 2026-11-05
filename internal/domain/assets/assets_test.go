package assets

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// resetVersion - Version resolves again (from the environment the test
// sets).
func resetVersion() {
	version = ""
	versionOnce = sync.Once{}
}

func TestStaticCSS(t *testing.T) {
	resetVersion()
	t.Setenv("STATIC_ASSET_VERSION", "abc123")
	got := StaticCSS("styles")
	want := "/static/css/styles.min.css?v=abc123"
	if got != want {
		t.Fatalf("StaticCSS() = %q, want %q", got, want)
	}
}

func TestStaticCSSAsync(t *testing.T) {
	resetVersion()
	t.Setenv("STATIC_ASSET_VERSION", "v2")

	got := string(StaticCSSAsync("forum"))
	if !strings.Contains(got, `rel="preload"`) || !strings.Contains(got, "forum.min.css?v=v2") {
		t.Fatalf("StaticCSSAsync() = %q", got)
	}
	if !strings.Contains(got, `<noscript><link rel="stylesheet"`) {
		t.Fatalf("StaticCSSAsync() missing noscript fallback: %q", got)
	}
}

func TestStaticJS(t *testing.T) {
	resetVersion()
	t.Setenv("STATIC_ASSET_VERSION", "abc123")
	got := StaticJS("app")
	want := "/static/js/app.min.js?v=abc123"
	if got != want {
		t.Fatalf("StaticJS() = %q, want %q", got, want)
	}
}

func TestStaticCSSStripsExtension(t *testing.T) {
	resetVersion()
	t.Setenv("STATIC_ASSET_VERSION", "v1")

	got := StaticCSS("forum.css")
	if !strings.HasSuffix(got, "forum.min.css?v=v1") {
		t.Fatalf("StaticCSS() = %q", got)
	}
}

func TestVersionFromEnv(t *testing.T) {
	resetVersion()
	t.Setenv("STATIC_ASSET_VERSION", "deploy-42")
	_ = os.Unsetenv("GIT_DIR")

	if v := Version(); v != "deploy-42" {
		t.Fatalf("Version() = %q, want deploy-42", v)
	}
}
