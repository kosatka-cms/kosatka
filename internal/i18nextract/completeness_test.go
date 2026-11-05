package i18nextract

import (
	"path/filepath"
	"slices"
	"testing"

	// A subtheme's base theme (paths.ThemeChain) comes from its manifest.
	_ "github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/translations"
)

// TestSourcesUpToDate checks that the embedded source strings' list
// (translations/sources) is the code's: the admin's interface strings
// page lists what it has. A theme the list has no strings of is one from
// outside the engine - a site's own in the tree a site builds, one
// installed from its repository: its strings come from its catalogs
// (the theme runtime's sources), so the list doesn't hold them.
func TestSourcesUpToDate(t *testing.T) {
	root := filepath.Join("..", "..")
	paths.SetThemesDir(filepath.Join(root, "themes"))
	t.Cleanup(func() { paths.SetThemesDir("") })
	msgs, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := i18n.ParseSources(translations.Sources)
	if err != nil {
		t.Fatal(err)
	}
	engineThemes := map[string]bool{}
	for _, s := range listed {
		engineThemes[s.Theme] = true
	}
	msgs = slices.DeleteFunc(msgs, func(m Message) bool { return m.Theme != "" && !engineThemes[m.Theme] })
	want, err := SourcesJSON(Sources(msgs))
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(translations.Sources) {
		t.Error("translations/sources/strings.json is behind the code: go run ./cmd/i18n-extract -sources")
	}
}
