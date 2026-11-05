package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// memPacks - entity.InterfacePackRepository in memory.
type memPacks struct {
	packs   []entity.InterfacePack
	strings []entity.InterfaceString
}

func (m *memPacks) Packs(context.Context) ([]entity.InterfacePack, error) {
	return slices.Clone(m.packs), nil
}

func (m *memPacks) Strings(context.Context) ([]entity.InterfaceString, error) {
	return slices.Clone(m.strings), nil
}

func (m *memPacks) Install(ctx context.Context, p entity.InterfacePack, strings []entity.InterfaceString) error {
	_ = m.Delete(ctx, p.Lang)
	m.packs = append(m.packs, p)
	for _, s := range strings {
		s.Lang = p.Lang
		m.strings = append(m.strings, s)
	}
	return nil
}

func (m *memPacks) Delete(_ context.Context, lang string) error {
	m.packs = slices.DeleteFunc(m.packs, func(p entity.InterfacePack) bool { return p.Lang == lang })
	m.strings = slices.DeleteFunc(m.strings, func(s entity.InterfaceString) bool { return s.Lang == lang })
	return nil
}

// testDePack - a German pack of testInterfaceStrings' strings: four
// that stand (a text, a plural one, one in a context, a disabled module's)
// and three left out (a placeholder dropped, a tag of its own, not a
// string of the code).
var testDePack = `{
	"strings": {
		"Comments": "Kommentare",
		"@count comment": ["@count Kommentar", "@count Kommentare"],
		"Hidden": "Versteckt",
		"<a href=\":login\">Log in</a> to post comments": "<a href=\"/login\">Anmelden</a>",
		"Polls": "<b>Umfragen</b>",
		"Not a string": "Keine"
	},
	"contexts": {"menu": {"Open": "Öffnen"}}
}`

// writeCatalog writes a catalog to dir: the index of files (name →
// content; the index's checksum of fr is wrong) and the files.
func writeCatalog(t *testing.T, dir string, files map[string]string, versions map[string]string) {
	t.Helper()
	var index TranslationsCatalog
	for code, content := range files {
		sum := sha256.Sum256([]byte(content))
		l := CatalogLanguage{Code: code, Version: versions[code], File: code + ".json", SHA256: hex.EncodeToString(sum[:]), Strings: 4, Draft: code == "de"}
		if code == "fr" {
			l.SHA256 = hex.EncodeToString(make([]byte, 32))
		}
		index.Languages = append(index.Languages, l)
		if err := os.WriteFile(filepath.Join(dir, code+".json"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	slices.SortFunc(index.Languages, func(a, b CatalogLanguage) int { return strings.Compare(a.Code, b.Code) })
	b, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTranslationsCatalogClean - the index's languages one can download:
// a valid code (not English, each once), version, file name next to the
// index and checksum (lower-cased).
func TestTranslationsCatalogClean(t *testing.T) {
	const sum = "ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd"
	c := TranslationsCatalog{Languages: []CatalogLanguage{
		{Code: "de", Version: "0.1.0", File: "de.json", SHA256: sum},
		{Code: "pt-BR", Version: "1.2.3+build", File: "pt-BR.v2.json", SHA256: "AB0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCD", Strings: -3},
		{Code: "de", Version: "0.2.0", File: "de2.json", SHA256: sum},
		{Code: "en", Version: "0.1.0", File: "en.json", SHA256: sum},
		{Code: "../x", Version: "0.1.0", File: "x.json", SHA256: sum},
		{Code: "fr", Version: "0.1.0", File: "../fr.json", SHA256: sum},
		{Code: "it", Version: "0.1.0", File: "https://example.com/it.json", SHA256: sum},
		{Code: "es", Version: "", File: "es.json", SHA256: sum},
		{Code: "nl", Version: "0.1.0", File: "nl.json", SHA256: "abc"},
	}}.clean()
	var got []string
	for _, l := range c.Languages {
		got = append(got, l.Code+" "+l.Version+" "+l.File)
	}
	if want := []string{"de 0.1.0 de.json", "pt-BR 1.2.3+build pt-BR.v2.json"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if l := c.Languages[1]; l.SHA256 != sum || l.Strings != 0 {
		t.Errorf("pt-BR = %+v", l)
	}
}

// TestInterfacePackDownload - a language of the catalog downloaded (over
// HTTP, its file next to the index): its strings that stand are the
// site's at once, the others left out; a language the site has now (its
// own strings changeable); the catalog read once an hour, again on a
// refresh; refused: a file whose checksum differs, a language the catalog
// hasn't or one built in, no catalog.
func TestInterfacePackDownload(t *testing.T) {
	s, _ := testInterfaceStrings(t)
	dir := t.TempDir()
	writeCatalog(t, dir, map[string]string{"de": testDePack, "fr": `{"strings": {"Comments": "Commentaires"}}`}, map[string]string{"de": "0.1.0", "fr": "0.1.0"})
	var indexReads atomic.Int32
	srv := httptest.NewServer(http.StripPrefix("/i18n/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "index.json" {
			indexReads.Add(1)
		}
		http.ServeFile(w, r, filepath.Join(dir, filepath.Base(r.URL.Path)))
	})))
	t.Cleanup(srv.Close)
	packs := &memPacks{}
	s.SetPacks(packs, srv.URL+"/i18n/index.json", []string{"en", "ru"})
	ctx := context.Background()
	de := i18n.WithLang(ctx, "de")

	res, err := s.Download(ctx, "de")
	if err != nil {
		t.Fatal(err)
	}
	if res.Kept != 4 || res.Dropped != 3 || res.Updated {
		t.Errorf("result = %+v, want 4 kept, 3 dropped", res)
	}
	if got := i18n.T(de, "Comments"); got != "Kommentare" {
		t.Errorf("T = %q", got)
	}
	if got := i18n.TC(de, "menu", "Open"); got != "Öffnen" {
		t.Errorf("TC = %q", got)
	}
	if got := i18n.Plural(de, 3, "@count comment", "@count comments"); got != "3 Kommentare" {
		t.Errorf("plural = %q", got)
	}
	if got := i18n.T(de, `<a href=":login">Log in</a> to post comments`); got != `<a href=":login">Log in</a> to post comments` {
		t.Errorf("a string left out = %q", got)
	}
	if !i18n.Current().Available("de") {
		t.Error("de isn't a language of the site")
	}
	if p := packs.packs; len(p) != 1 || p[0].Version != "0.1.0" || !p[0].Draft || p[0].Strings != 4 {
		t.Errorf("packs = %+v", p)
	}

	if err := s.Save(ctx, "de", "", "Comments", "Anmerkungen", nil); err != nil {
		t.Fatal(err)
	}
	res, err = s.Download(ctx, "de")
	if err != nil || !res.Updated {
		t.Fatalf("again: %+v, %v", res, err)
	}
	if got := i18n.T(de, "Comments"); got != "Anmerkungen" {
		t.Errorf("the site's change after an update = %q", got)
	}
	if n := indexReads.Load(); n != 1 {
		t.Errorf("the index read %d times, want once", n)
	}
	if _, _, err := s.Catalog(ctx, true); err != nil || indexReads.Load() != 2 {
		t.Errorf("a refresh: %v, %d reads", err, indexReads.Load())
	}

	for lang, want := range map[string]error{"fr": ErrPackChecksum, "xx": ErrUnknownPack, "ru": ErrUnknownPack, "en": ErrUnknownPack} {
		if _, err := s.Download(ctx, lang); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", lang, err, want)
		}
	}
	if i18n.Current().Available("fr") {
		t.Error("a refused download installed fr")
	}
	s.SetPacks(packs, "", []string{"en", "ru"})
	if _, err := s.Download(ctx, "de"); !errors.Is(err, ErrNoTranslationsCatalog) {
		t.Errorf("no catalog: %v", err)
	}
}

// TestInterfacePackDelete - a downloaded language removed: the language
// is gone from the site, the site's own changes of it stay (back with the
// language); the site's interface language can't be removed, nor a
// language that isn't downloaded.
func TestInterfacePackDelete(t *testing.T) {
	s, repo := testInterfaceStrings(t)
	dir := t.TempDir()
	writeCatalog(t, dir, map[string]string{"de": testDePack}, map[string]string{"de": "0.1.0"})
	packs := &memPacks{}
	s.SetPacks(packs, filepath.Join(dir, "index.json"), []string{"en", "ru"})
	ctx := context.Background()
	if _, err := s.Download(ctx, "de"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, "de", "", "Comments", "Anmerkungen", nil); err != nil {
		t.Fatal(err)
	}

	if err := s.DeletePack(ctx, "fr"); !errors.Is(err, ErrPackNotInstalled) {
		t.Errorf("a language not downloaded: %v", err)
	}
	i18n.SetDefault("de")
	err := s.DeletePack(ctx, "de")
	i18n.SetDefault(i18n.SourceLanguage)
	if !errors.Is(err, ErrPackInUse) {
		t.Errorf("the site's language: %v", err)
	}
	if err := s.DeletePack(ctx, "de"); err != nil {
		t.Fatal(err)
	}
	if i18n.Current().Available("de") || len(packs.packs) != 0 || len(packs.strings) != 0 {
		t.Errorf("de still there: %v, %+v", i18n.Current().Available("de"), packs)
	}
	if len(repo.rows) != 1 {
		t.Errorf("the site's changes: %+v", repo.rows)
	}
	if _, err := s.Download(ctx, "de"); err != nil {
		t.Fatal(err)
	}
	if got := i18n.T(i18n.WithLang(ctx, "de"), "Comments"); got != "Anmerkungen" {
		t.Errorf("the site's change back = %q", got)
	}
}

// TestInterfaceLanguagesPage - the Languages tab: English (the source),
// the built-in ones, the downloaded ones (an update when the catalog's
// checksum differs; the site's strings they translate counted), the
// catalog's others (their strings counted from the index), in that order; when the catalog was read; no catalog or one
// that can't be read - the languages still.
func TestInterfaceLanguagesPage(t *testing.T) {
	s, _ := testInterfaceStrings(t)
	now := time.Date(2026, 11, 4, 15, 10, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	dir := t.TempDir()
	writeCatalog(t, dir, map[string]string{"de": testDePack, "fr": `{}`, "ru": `{}`}, map[string]string{"de": "0.1.0", "fr": "0.1.0", "ru": "0.1.0"})
	packs := &memPacks{}
	s.SetPacks(packs, filepath.Join(dir, "index.json"), []string{"en", "ru"})
	ctx := context.Background()
	states := func(refresh bool) string {
		t.Helper()
		page, err := s.LanguagesPage(ctx, refresh)
		if err != nil || page.CatalogErr != nil {
			t.Fatal(err, page.CatalogErr)
		}
		if !page.CatalogChecked.Equal(now) {
			t.Errorf("checked %v", page.CatalogChecked)
		}
		out := ""
		for _, l := range page.Languages {
			out += l.Code + ":" + string(l.State) + " "
		}
		return out
	}
	if got := states(false); got != "en:source ru:builtin de:available fr:available " {
		t.Errorf("before = %s", got)
	}
	page, _ := s.LanguagesPage(ctx, false)
	if de := page.Languages[2]; de.Total != 5 || de.Translated != 4 || de.Catalog == nil || !de.Catalog.Draft {
		t.Errorf("de available = %+v", de)
	}
	if _, err := s.Download(ctx, "de"); err != nil {
		t.Fatal(err)
	}
	if got := states(false); got != "en:source ru:builtin de:installed fr:available " {
		t.Errorf("installed = %s", got)
	}
	writeCatalog(t, dir, map[string]string{"de": testDePack + " ", "fr": `{}`}, map[string]string{"de": "0.2.0", "fr": "0.1.0"})
	if got := states(false); got != "en:source ru:builtin de:installed fr:available " {
		t.Errorf("the catalog kept an hour = %s", got)
	}
	if got := states(true); got != "en:source ru:builtin de:update fr:available " {
		t.Errorf("an update = %s", got)
	}
	page, _ = s.LanguagesPage(ctx, false)
	if de := page.Languages[2]; de.Pack == nil || de.Pack.Version != "0.1.0" || de.Catalog.Version != "0.2.0" || de.Translated != 3 {
		t.Errorf("de update = %+v", de)
	}

	s.SetPacks(packs, filepath.Join(dir, "missing.json"), []string{"en", "ru"})
	page, err := s.LanguagesPage(ctx, false)
	if err != nil || page.CatalogErr == nil || len(page.Languages) != 3 {
		t.Errorf("a catalog that can't be read: %v, %v, %d languages", err, page.CatalogErr, len(page.Languages))
	}
	s.SetPacks(packs, "", []string{"en", "ru"})
	if page, _ := s.LanguagesPage(ctx, false); !errors.Is(page.CatalogErr, ErrNoTranslationsCatalog) || page.CatalogURL != "" {
		t.Errorf("no catalog: %v", page.CatalogErr)
	}
}

// TestInterfacePackStarterTexts - a text of the starter content (Starter):
// not one of the site's strings (the list, the counts), kept in a
// downloaded language - a site installed in it gets it so.
func TestInterfacePackStarterTexts(t *testing.T) {
	s, _ := testInterfaceStrings(t)
	s.sources = append(s.sources, i18n.Source{Source: "Front page", Starter: true})
	dir := t.TempDir()
	writeCatalog(t, dir, map[string]string{"de": `{"strings": {"Front page": "Startseite", "Comments": "Kommentare"}}`}, map[string]string{"de": "0.1.0"})
	s.SetPacks(&memPacks{}, filepath.Join(dir, "index.json"), []string{"en", "ru"})
	ctx := context.Background()
	res, err := s.Download(ctx, "de")
	if err != nil || res.Kept != 2 || res.Dropped != 0 {
		t.Fatalf("download: %+v %v", res, err)
	}
	if got := i18n.T(i18n.WithLang(ctx, "de"), "Front page"); got != "Startseite" {
		t.Errorf("the starter text in German: %q", got)
	}
	p, err := s.List(ctx, InterfaceStringsFilter{Lang: "de", Query: "front page"})
	if err != nil || p.Total != 0 {
		t.Errorf("the strings list has the starter text: %+v %v", p.Rows, err)
	}
	for _, l := range s.Languages() {
		if l.Total != 5 {
			t.Errorf("%s counts %d strings, want 5", l.Code, l.Total)
		}
	}
}
