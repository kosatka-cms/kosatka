package themes

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeSettingsRepo struct{ rows map[string]map[string]string }

func (f *fakeSettingsRepo) All(context.Context) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	for k, v := range f.rows {
		out[k] = v
	}
	return out, nil
}

func (f *fakeSettingsRepo) Save(_ context.Context, theme string, values map[string]string) error {
	f.rows[theme] = values
	return nil
}

// settingsFixture - a cwd whose theme "t1" offers a colour (a CSS
// variable of the dark scheme), a select and a bool.
func settingsFixture(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"web/templates/theme.yaml": "name: Base\nregions:\n  - id: sidebar\n    label: Sidebar\n",
		"themes/t1/theme.yaml": `name: T1
regions:
  - id: sidebar
    label: Sidebar
settings:
  - id: accent
    label: Accent
    type: color
    default: "#0e7490"
    css_var: --t1-accent
    selector: ':root[data-bs-theme="dark"]'
  - id: scheme
    label: Scheme
    type: select
    default: light
    options:
      - value: light
        label: Light
      - value: dark
        label: Dark
  - id: marquee
    label: Marquee
    type: bool
    default: true
  - id: tagline
    label: Tagline
    type: text
    max_length: 20
`,
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
}

// A theme's values: the defaults, then the saved ones (each checked
// against its setting); the <style> holds its colours.
func TestSettingsService(t *testing.T) {
	settingsFixture(t)
	ctx := t.Context()
	repo := &fakeSettingsRepo{rows: map[string]map[string]string{}}
	svc := NewSettingsService(repo, slog.New(slog.DiscardHandler))

	if got := svc.Values(ctx, "t1"); got["accent"] != "#0e7490" || got["scheme"] != "light" || got["marquee"] != "1" {
		t.Fatalf("defaults: %v", got)
	}
	if style := string(svc.Style(ctx, "t1")); !strings.Contains(style, "--t1-accent: #0e7490;") {
		t.Errorf("the default colour's style: %q", style)
	}
	if len(svc.Settings("")) != 0 || svc.Style(ctx, "") != "" {
		t.Error("the engine's templates offer no settings")
	}

	// A bool missing from the form is off; a colour's case is kept low.
	if err := svc.Save(ctx, "t1", map[string]string{"accent": "#AA0000", "scheme": "dark"}); err != nil {
		t.Fatal(err)
	}
	if got := svc.Values(ctx, "t1"); got["accent"] != "#aa0000" || got["scheme"] != "dark" || got["marquee"] != "0" {
		t.Errorf("saved: %v", got)
	}
	if v := svc.Value(ctx, "t1", "marquee"); v != false {
		t.Errorf("a bool's template value: %#v, want false", v)
	}
	if v := svc.Value(ctx, "t1", "scheme"); v != "dark" {
		t.Errorf("a select's template value: %#v", v)
	}
	if v := svc.Value(ctx, "t1", "nope"); v != nil {
		t.Errorf("an unknown setting: %#v, want nil", v)
	}
	want := `<style id="theme-settings">:root[data-bs-theme="dark"] { --t1-accent: #aa0000; }</style>`
	if style := string(svc.Style(ctx, "t1")); style != want {
		t.Errorf("style: %q, want %q", style, want)
	}

	// A text: its line for the templates; empty allowed; too long or with
	// a line break - invalid.
	if err := svc.Save(ctx, "t1", map[string]string{"accent": "#aa0000", "scheme": "dark", "tagline": "Under construction"}); err != nil {
		t.Fatal(err)
	}
	if v := svc.Value(ctx, "t1", "tagline"); v != "Under construction" {
		t.Errorf("a text's template value: %#v", v)
	}
	for _, bad := range []string{"a line far longer than twenty", "two\nlines"} {
		if err := svc.Save(ctx, "t1", map[string]string{"accent": "#aa0000", "scheme": "dark", "tagline": bad}); !errors.Is(err, ErrSettingInvalid) {
			t.Errorf("text %q: %v", bad, err)
		}
	}
	if err := svc.Save(ctx, "t1", map[string]string{"accent": "#aa0000", "scheme": "dark"}); err != nil {
		t.Fatal(err)
	}
	if v := svc.Value(ctx, "t1", "tagline"); v != "" {
		t.Errorf("an empty text: %#v", v)
	}

	// An invalid value saves nothing.
	if err := svc.Save(ctx, "t1", map[string]string{"accent": "red", "scheme": "dark"}); !errors.Is(err, ErrSettingInvalid) {
		t.Errorf("an invalid colour: %v", err)
	}
	if got := svc.Values(ctx, "t1")["accent"]; got != "#aa0000" {
		t.Errorf("after an invalid save: %q", got)
	}

	// A value the theme no longer offers falls back to the default.
	repo.rows[entity.ThemeKey("t1")] = map[string]string{"scheme": "sepia"}
	fresh := NewSettingsService(repo, slog.New(slog.DiscardHandler))
	if got := fresh.Values(ctx, "t1")["scheme"]; got != "light" {
		t.Errorf("a stale value: %q, want the default", got)
	}

	if err := svc.Reset(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	if got := svc.Values(ctx, "t1"); got["accent"] != "#0e7490" || got["marquee"] != "1" {
		t.Errorf("after reset: %v", got)
	}
}

// Every theme of the engine has a valid manifest (its settings too), or
// it would vanish from the Appearance page.
func TestThemeManifestsValid(t *testing.T) {
	zzChdirToProjectRoot(t)
	entries, err := os.ReadDir("themes")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := LoadInfoFor(e.Name()); err != nil {
			t.Errorf("theme %s: %v", e.Name(), err)
		}
	}
}

// TestSettingsConcurrentSave - pages read a theme's values while an
// admin saves them (go test -race: the stored values are replaced, not
// changed in place).
func TestSettingsConcurrentSave(t *testing.T) {
	settingsFixture(t)
	ctx := t.Context()
	svc := NewSettingsService(&fakeSettingsRepo{rows: map[string]map[string]string{}}, slog.New(slog.DiscardHandler))
	svc.Values(ctx, "t1") // loaded
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				svc.Values(ctx, "t1")
				svc.Values(ctx, "t2")
			}
		}()
	}
	for i := range 50 {
		if err := svc.Save(ctx, "t1", map[string]string{"accent": "#0e7490", "scheme": []string{"light", "dark"}[i%2]}); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}
