package paths

import (
	"os"
	"path/filepath"
	"testing"
)

// themeFixture - a cwd with web/templates holding engine files and a
// theme "t1" overriding one of them and adding another.
func themeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"web/templates/teaser/default.html":       "engine default",
		"web/templates/teaser/article.html":       "engine article",
		"web/templates/partials/theme_head.html":  "engine head",
		"themes/t1/templates/teaser/article.html": "theme article",
		"themes/t1/templates/teaser/event.html":   "theme event",
	}
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	t.Cleanup(func() { SetTheme("") })
	return filepath.Join(root, "web", "templates")
}

func TestResolveTemplate(t *testing.T) {
	base := themeFixture(t)

	got, err := ResolveTemplate("teaser/article.html")
	if err != nil || got != filepath.Join(base, "teaser/article.html") {
		t.Fatalf("no theme: %q %v", got, err)
	}

	SetTheme("t1")
	if got, _ := ResolveTemplate("teaser/article.html"); got != filepath.Join(base, "../../themes/t1/templates/teaser/article.html") {
		t.Errorf("theme copy expected, got %q", got)
	}
	if got, _ := ResolveTemplate("partials/theme_head.html"); got != filepath.Join(base, "partials/theme_head.html") {
		t.Errorf("engine fallback expected, got %q", got)
	}

	SetTheme("missing")
	if got, _ := ResolveTemplate("teaser/article.html"); got != filepath.Join(base, "teaser/article.html") {
		t.Errorf("missing theme dir must fall back to the engine, got %q", got)
	}
}

func TestGlobTemplates(t *testing.T) {
	base := themeFixture(t)

	got, err := GlobTemplates("teaser/*.html")
	if err != nil || len(got) != 2 {
		t.Fatalf("no theme: %v %v", got, err)
	}

	SetTheme("t1")
	got, err = GlobTemplates("teaser/*.html")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(base, "../../themes/t1/templates/teaser/article.html"), // replaces the engine's
		filepath.Join(base, "teaser/default.html"),
		filepath.Join(base, "../../themes/t1/templates/teaser/event.html"), // theme-only, last
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	seen := map[string]bool{}
	for _, p := range got {
		seen[p] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("missing %s in %v", w, got)
		}
	}
	if got[len(got)-1] != want[2] {
		t.Errorf("theme-only file must come last: %v", got)
	}
}

// A request's own theme (a preview) takes the active one's place in
// ThemeFor; without it the active one renders.
func TestThemeFor(t *testing.T) {
	base := themeFixture(t)
	SetTheme("t1")
	ctx := t.Context()
	if got := ThemeFor(ctx); got != "t1" {
		t.Errorf("no preview: %q, want the active t1", got)
	}
	engine := WithTheme(ctx, "")
	if got, ok := PreviewTheme(engine); !ok || got != "" {
		t.Errorf("the engine's templates previewed: %q %v", got, ok)
	}
	if got, _ := ResolveTemplateIn(ThemeFor(engine), "teaser/article.html"); got != filepath.Join(base, "teaser/article.html") {
		t.Errorf("the engine's copy expected in its preview, got %q", got)
	}
	if got, _ := ResolveTemplateIn(ThemeFor(WithTheme(ctx, "t1")), "teaser/article.html"); got != filepath.Join(base, "../../themes/t1/templates/teaser/article.html") {
		t.Errorf("the theme's copy expected in its preview, got %q", got)
	}
}

// subthemeFixture - themeFixture with "sub" based on t1 (it replaces one
// of t1's templates and adds its own; a stylesheet only t1 has) and a
// "loop" based on itself.
func subthemeFixture(t *testing.T) string {
	t.Helper()
	base := themeFixture(t)
	root := filepath.Dir(filepath.Dir(base))
	files := map[string]string{
		"themes/sub/templates/teaser/event.html":  "sub event",
		"themes/sub/templates/teaser/review.html": "sub review",
		"themes/t1/static/css/t1.css":             "",
		"themes/sub/static/css/sub.css":           "",
	}
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	SetBaseThemes(func(name string) string {
		return map[string]string{"sub": "t1", "loop": "loop"}[name]
	})
	t.Cleanup(func() { SetBaseThemes(nil) })
	return base
}

// A subtheme's chain: its templates, then its base's, then the engine's.
func TestSubthemeTemplates(t *testing.T) {
	base := subthemeFixture(t)
	if got := ThemeChain("sub"); len(got) != 2 || got[0] != "sub" || got[1] != "t1" {
		t.Errorf("chain: %v", got)
	}
	if got := ThemeChain("loop"); len(got) != 1 {
		t.Errorf("a loop: %v, want it once", got)
	}
	for rel, want := range map[string]string{
		"teaser/event.html":        "../../themes/sub/templates/teaser/event.html",  // its own over its base's
		"teaser/article.html":      "../../themes/t1/templates/teaser/article.html", // its base's
		"partials/theme_head.html": "partials/theme_head.html",                      // the engine's
	} {
		if got, _ := ResolveTemplateIn("sub", rel); got != filepath.Join(base, want) {
			t.Errorf("%s: %q, want %s", rel, got, want)
		}
	}
	got, err := GlobTemplatesIn("sub", "teaser/*.html")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(base, "teaser/default.html"),
		filepath.Join(base, "../../themes/t1/templates/teaser/article.html"),
		filepath.Join(base, "../../themes/sub/templates/teaser/event.html"),
		filepath.Join(base, "../../themes/sub/templates/teaser/review.html"),
	}
	seen := map[string]bool{}
	for _, p := range got {
		seen[p] = true
	}
	if len(got) != len(want) {
		t.Fatalf("glob: %v, want %v", got, want)
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("glob: no %s in %v", w, got)
		}
	}
	if ThemeWithStatic("sub", "css/t1.css") != "t1" || ThemeWithStatic("sub", "css/sub.css") != "sub" || ThemeWithStatic("sub", "css/none.css") != "sub" {
		t.Error("a stylesheet's theme: the first of the chain with it, else the theme")
	}
}
