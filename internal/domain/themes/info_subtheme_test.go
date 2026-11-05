package themes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/assets"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// subthemeManifests - a cwd with a base theme "b" (regions, a setting,
// a stylesheet of its own templates use) and subthemes of it.
func subthemeManifests(t *testing.T, extra map[string]string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"web/templates/theme.yaml": "name: Base\nregions:\n  - id: sidebar\n    label: Sidebar\n",
		"themes/b/theme.yaml": `name: B
default_region: sidebar
regions:
  - id: header
    label: Header
  - id: sidebar
    label: Sidebar
settings:
  - id: accent
    label: Accent
    type: color
    default: "#0e7490"
    css_var: --b-accent
`,
		"themes/b/static/css/b.css": "",
		"themes/sub/theme.yaml": `name: Sub
base: b
stylesheets: [sub]
settings:
  - id: accent
    label: Accent
    type: color
    default: "#aa0000"
    css_var: --b-accent
  - id: stripes
    label: Stripes
    type: bool
    default: false
`,
		"themes/sub/static/css/sub.css": "",
	}
	for k, v := range extra {
		files[k] = v
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

// A subtheme takes its base's regions, and its settings over its base's.
func TestSubthemeInfo(t *testing.T) {
	subthemeManifests(t, nil)
	info, err := LoadInfoFor("sub")
	if err != nil {
		t.Fatal(err)
	}
	if info.Base != "b" || len(info.Regions) != 2 || info.DefaultRegion != "sidebar" {
		t.Errorf("regions from the base: %+v", info)
	}
	if len(info.Settings) != 2 || info.Settings[0].ID != "accent" || info.Settings[0].Default != "#aa0000" || info.Settings[1].ID != "stripes" {
		t.Errorf("settings: %+v", info.Settings)
	}
	themes, err := Available()
	if err != nil {
		t.Fatal(err)
	}
	var sub *Option
	for i := range themes {
		if themes[i].Dir == "sub" {
			sub = &themes[i]
		}
	}
	if sub == nil || sub.Base != "b" || sub.Regions != 2 {
		t.Errorf("the subtheme on the Appearance page: %+v", themes)
	}
}

// The chain's stylesheets: the subtheme's own after its base's
// (theme_head); a stylesheet only the base has resolves to the base.
func TestSubthemeStylesheets(t *testing.T) {
	subthemeManifests(t, nil)
	got := Stylesheets("sub")
	if len(got) != 1 || !strings.HasPrefix(got[0], "/static/themes/sub/css/sub.min.css") {
		t.Errorf("stylesheets: %v", got)
	}
	if css := assets.ThemeCSS("sub", "b"); !strings.HasPrefix(css, "/static/themes/b/css/b.min.css") {
		t.Errorf("the base's stylesheet in the subtheme: %q", css)
	}
	if chain := paths.ThemeChain("sub"); len(chain) != 2 || chain[1] != "b" {
		t.Errorf("chain from the manifests: %v", chain)
	}
}

// A theme's script (themeJS): its own, else its base's; minified and
// versioned; none without a theme.
func TestThemeJS(t *testing.T) {
	subthemeManifests(t, map[string]string{"themes/b/static/js/menu.js": "", "themes/sub/static/js/extra.js": ""})
	if js := assets.ThemeJS("sub", "extra"); !strings.HasPrefix(js, "/static/themes/sub/js/extra.min.js?v=") {
		t.Errorf("the theme's own script: %q", js)
	}
	if js := assets.ThemeJS("sub", "menu.js"); !strings.HasPrefix(js, "/static/themes/b/js/menu.min.js?v=") {
		t.Errorf("the base's script in the subtheme: %q", js)
	}
	if js := assets.ThemeJS("", "menu"); js != "" {
		t.Errorf("without a theme: %q", js)
	}
}

// A subtheme of a missing theme, of itself, or naming a stylesheet
// badly doesn't load.
func TestSubthemeInvalid(t *testing.T) {
	subthemeManifests(t, map[string]string{
		"themes/orphan/theme.yaml": "name: Orphan\nbase: missing\n",
		"themes/self/theme.yaml":   "name: Self\nbase: self\n",
		"themes/bad/theme.yaml":    "name: Bad\nbase: b\nstylesheets: [\"../x\"]\n",
	})
	for _, dir := range []string{"orphan", "self", "bad"} {
		if _, err := LoadInfoFor(dir); err == nil {
			t.Errorf("theme %s: no error", dir)
		}
	}
	// They're listed as invalid, each with its error, paths relative; a
	// hidden directory isn't a theme.
	if err := os.MkdirAll("themes/.git", 0o755); err != nil {
		t.Fatal(err)
	}
	invalid, err := Invalid()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, b := range invalid {
		got[b.Dir] = b.Err
	}
	if len(got) != 3 || got["orphan"] == "" || got["self"] == "" || !strings.Contains(got["bad"], "themes/bad/theme.yaml") {
		t.Errorf("invalid themes: %v", got)
	}
}

// A chain of base themes loads as deep as the engine renders it
// (paths.MaxThemeChain themes), not a theme deeper.
func TestSubthemeChainDepth(t *testing.T) {
	// c1 → c2 → … → c<n> → b: n+1 themes.
	n := paths.MaxThemeChain - 1
	extra := map[string]string{}
	for i := 1; i <= n; i++ {
		base := fmt.Sprintf("c%d", i+1)
		if i == n {
			base = "b"
		}
		extra[fmt.Sprintf("themes/c%d/theme.yaml", i)] = fmt.Sprintf("name: C%d\nbase: %s\n", i, base)
	}
	extra["themes/c0/theme.yaml"] = "name: C0\nbase: c1\n"
	subthemeManifests(t, extra)
	if _, err := LoadInfoFor("c1"); err != nil {
		t.Errorf("a chain of %d themes: %v", paths.MaxThemeChain, err)
	}
	if _, err := LoadInfoFor("c0"); err == nil || !strings.Contains(err.Error(), "deeper than") {
		t.Errorf("a chain of %d themes: %v, want an error", paths.MaxThemeChain+1, err)
	}
}
