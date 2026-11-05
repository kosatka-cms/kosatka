package i18npack

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// testSources - the code's strings of the tests.
var testSources = []i18n.Source{
	{Source: "Save"},
	{Source: "@count comment", Plural: "@count comments"},
	{Source: "Comments"},
	{Context: "menu", Source: "Open"},
	{Source: "Reply", Module: "forum"},
	{Source: `<a href=":login">Log in</a> to post comments`},
}

// testRepo - a translations repository: German (a draft) in two
// components, one string translated by both alike, one not the code's
// any more, one with a tag of its own; Russian complete; directories that
// aren't languages.
func testRepo() fstest.MapFS {
	return fstest.MapFS{
		"de/language.yaml": {Data: []byte("version: 0.1.0\ndraft: true\n")},
		"de/engine.json": {Data: []byte(`{
			"strings": {
				"Save": "Speichern",
				"@count comment": ["@count Kommentar", "@count Kommentare"],
				"Comments": "<b>Kommentare</b>",
				"<a href=\":login\">Log in</a> to post comments": "<a href=\":login\">Anmelden</a>, um zu kommentieren",
				"An old string": "Alt"
			},
			"contexts": {"menu": {"Open": "Öffnen"}}
		}`)},
		"de/forum.json":    {Data: []byte(`{"strings": {"Save": "Speichern", "Reply": "Antworten"}}`)},
		"ru/language.yaml": {Data: []byte("version: 1.2.0\n")},
		"ru/engine.json": {Data: []byte(`{
			"strings": {
				"Save": "Сохранить",
				"@count comment": ["@count комментарий", "@count комментария", "@count комментариев"],
				"Comments": "Комментарии",
				"Reply": "Ответить",
				"<a href=\":login\">Log in</a> to post comments": "<a href=\":login\">Войдите</a>, чтобы комментировать"
			},
			"contexts": {"menu": {"Open": "Открыть"}}
		}`)},
		"en/language.yaml":     {Data: []byte("version: 0.1.0\n")},
		"sources/strings.json": {Data: []byte(`[]`)},
		"README.md":            {Data: []byte("# Translations\n")},
	}
}

// TestBuild - each language (not English, not another directory): its
// version and draft; the strings that stand, each once; one not the
// code's left out as stale; one that can't stand a problem (and missing);
// the code's strings it doesn't translate.
func TestBuild(t *testing.T) {
	langs, problems, err := Build(testRepo(), testSources)
	if err != nil {
		t.Fatal(err)
	}
	if len(langs) != 2 || langs[0].Code != "de" || langs[1].Code != "ru" {
		t.Fatalf("languages: %+v", langs)
	}
	de, ru := langs[0], langs[1]
	if de.Version != "0.1.0" || !de.Draft || ru.Version != "1.2.0" || ru.Draft {
		t.Errorf("manifests: de %s %v, ru %s %v", de.Version, de.Draft, ru.Version, ru.Draft)
	}
	var got []string
	for _, e := range de.Entries {
		got = append(got, e.Context+"|"+e.Source)
	}
	want := []string{`|<a href=":login">Log in</a> to post comments`, "|@count comment", "|Reply", "|Save", "menu|Open"}
	if !slices.Equal(got, want) {
		t.Errorf("de entries: %v, want %v", got, want)
	}
	if len(de.Stale) != 1 || de.Stale[0].Source != "An old string" {
		t.Errorf("de stale: %+v", de.Stale)
	}
	if len(de.Missing) != 1 || de.Missing[0].Source != "Comments" {
		t.Errorf("de missing: %+v", de.Missing)
	}
	if len(problems) != 1 || problems[0].File != "de/engine.json" || problems[0].Source != "Comments" ||
		!strings.Contains(problems[0].Error(), "<b>") {
		t.Errorf("problems: %+v", problems)
	}
	if len(ru.Entries) != 6 || len(ru.Missing) != 0 || len(ru.Stale) != 0 {
		t.Errorf("ru: %d entries, missing %+v, stale %+v", len(ru.Entries), ru.Missing, ru.Stale)
	}
}

// TestBuildProblems - two files translating a string differently, plural
// forms not the language's number, an empty translation, a dropped
// placeholder; a manifest without a version or one broken - an error.
func TestBuildProblems(t *testing.T) {
	repo := fstest.MapFS{
		"de/language.yaml": {Data: []byte("version: 0.1.0\n")},
		"de/a.json":        {Data: []byte(`{"strings": {"Save": "Speichern", "@count comment": ["@count Kommentar"], "Reply": ""}}`)},
		"de/b.json":        {Data: []byte(`{"strings": {"Save": "Sichern", "<a href=\":login\">Log in</a> to post comments": "<a href=\"/login\">Anmelden</a>"}}`)},
	}
	_, problems, err := Build(repo, testSources)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range problems {
		got = append(got, p.Error())
	}
	for _, want := range []string{`de/a.json: "@count comment": 1 plural forms, the language has 2`, `de/a.json: "Reply": the translation is empty`,
		`de/b.json: "Save": another file of the language translates it differently`, `de/b.json: "<a href=\":login\">Log in</a> to post comments": the translation drops the placeholder :login`} {
		if !slices.Contains(got, want) {
			t.Errorf("no %q in %q", want, got)
		}
	}
	for name, manifest := range map[string]string{"no version": "draft: true\n", "a bad version": "version: \"../x\"\n", "broken": "version: [\n"} {
		repo := fstest.MapFS{"de/language.yaml": {Data: []byte(manifest)}, "de/a.json": {Data: []byte(`{}`)}}
		if _, _, err := Build(repo, testSources); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, _, err := Build(fstest.MapFS{"de/a.json": {Data: []byte(`{}`)}}, testSources); err == nil {
		t.Error("no language.yaml: no error")
	}
}

// TestWrite - a catalog file per language (its strings, HTML as it is,
// the same bytes each time) and the index with each file's checksum and
// strings; the sites read it (service.TranslationsCatalog).
func TestWrite(t *testing.T) {
	langs, _, err := Build(testRepo(), testSources)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	at := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	index, err := Write(out, langs, at)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var read service.TranslationsCatalog
	if err := json.Unmarshal(b, &read); err != nil {
		t.Fatal(err)
	}
	if !read.Generated.Equal(at) || len(index.Languages) != 2 {
		t.Errorf("index: %+v", read)
	}
	for _, l := range langs {
		i := slices.IndexFunc(read.Languages, func(c service.CatalogLanguage) bool { return c.Code == l.Code })
		if i < 0 {
			t.Fatalf("%s isn't in the index: %s", l.Code, b)
		}
		entry := read.Languages[i]
		file, err := os.ReadFile(filepath.Join(out, entry.File))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(file)
		if entry.SHA256 != hex.EncodeToString(sum[:]) || entry.Strings != len(l.Entries) || entry.Version != l.Version || entry.Draft != l.Draft {
			t.Errorf("%s: %+v", l.Code, entry)
		}
		back, err := i18n.ParseCatalog(l.Code, file)
		if err != nil || !slices.EqualFunc(back, l.Entries, func(a, b i18n.Entry) bool {
			return a.Context == b.Context && a.Source == b.Source && a.Text == b.Text && slices.Equal(a.Forms, b.Forms)
		}) {
			t.Errorf("%s read back: %+v (%v), want %+v", l.Code, back, err, l.Entries)
		}
		again, _ := CatalogJSON(l.Entries)
		if string(again) != string(file) {
			t.Errorf("%s: not the same bytes again", l.Code)
		}
	}
	de, _ := os.ReadFile(filepath.Join(out, "de.json"))
	if !strings.Contains(string(de), `"<a href=\":login\">Log in</a> to post comments": "<a href=\":login\">Anmelden</a>, um zu kommentieren"`) {
		t.Errorf("de.json's HTML escaped:\n%s", de)
	}
}
