package i18n

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func testTranslator(t *testing.T) *Translator {
	t.Helper()
	engine := fstest.MapFS{"ru.json": {Data: []byte(`{
		"strings": {
			"Save": "Сохранить",
			"Yes": "Да",
			"@count view": ["@count просмотр", "@count просмотра", "@count просмотров"],
			"Hello @name": "Привет, @name",
			"See %title": "Смотри %title",
			"Comments": "Комментарии"
		},
		"contexts": {"Long month name": {"May": "май"}}
	}`)}}
	theme := fstest.MapFS{"ru.json": {Data: []byte(`{"strings": {"Comments": "Отзывы"}}`)}}
	tr, err := Load(theme, nil, engine)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestPluralIndex(t *testing.T) {
	ru := map[int]int{0: 2, 1: 0, 2: 1, 4: 1, 5: 2, 11: 2, 12: 2, 14: 2, 21: 0, 22: 1, 25: 2, 101: 0, 111: 2, 112: 2, 122: 1, -1: 0}
	for n, want := range ru {
		if got := PluralIndex("ru", n); got != want {
			t.Errorf("ru %d: %d, want %d", n, got, want)
		}
	}
	if PluralIndex("en", 1) != 0 || PluralIndex("en", 0) != 1 || PluralIndex("en", 2) != 1 || PluralIndex("de-DE", 3) != 1 {
		t.Fatal("en/de")
	}
	// Russian and Belarusian keep the catalogs' three forms: the old
	// hand-written rule for every count.
	legacy := func(n int) int {
		switch {
		case n%10 == 1 && n%100 != 11:
			return 0
		case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
			return 1
		}
		return 2
	}
	for _, lang := range []string{"ru", "be"} {
		for n := 0; n <= 2000; n++ {
			if got := PluralIndex(lang, n); got != legacy(n) {
				t.Fatalf("%s %d: %d, want %d", lang, n, got, legacy(n))
			}
		}
	}
	// Other languages by their own CLDR rules.
	for _, c := range []struct {
		lang  string
		count int
		want  int
	}{
		{"pl", 1, 0}, {"pl", 3, 1}, {"pl", 5, 2}, {"pl", 22, 1}, {"pl", 12, 2}, {"pl", 21, 2},
		{"cs", 1, 0}, {"cs", 3, 1}, {"cs", 5, 2},
		{"fr", 0, 0}, {"fr", 1, 0}, {"fr", 2, len(PluralCategories("fr")) - 1},
		{"ja", 1, 0}, {"ja", 5, 0},
		{"xx-unknown", 1, 0}, {"xx-unknown", 2, 1},
	} {
		if got := PluralIndex(c.lang, c.count); got != c.want {
			t.Errorf("%s %d: %d, want %d", c.lang, c.count, got, c.want)
		}
	}
}

func TestPluralCategories(t *testing.T) {
	for lang, want := range map[string]string{"ru": "one few many", "en": "one other", "pl": "one few many", "ja": "other", "": "one other"} {
		if got := strings.Join(PluralCategories(lang), " "); got != want {
			t.Errorf("%q: %s, want %s", lang, got, want)
		}
	}
}

func TestTranslate(t *testing.T) {
	tr := testTranslator(t)
	cases := []struct{ got, want string }{
		{tr.T("ru", "", "Save"), "Сохранить"},
		{tr.T("ru", "", "Untranslated"), "Untranslated"},
		{tr.T("en", "", "Save"), "Save"},
		{tr.T("ru", "", "Comments"), "Отзывы"}, // the theme wins
		{tr.T("ru", "Long month name", "May"), "май"},
		{tr.T("ru", "", "May"), "May"}, // no context - no match
		{tr.T("ru", "", "Hello @name", "@name", "<b>Боб</b>"), "Привет, &lt;b&gt;Боб&lt;/b&gt;"},
		{tr.T("ru", "", "See %title", "%title", "X"), `Смотри <em class="placeholder">X</em>`},
		{tr.Plural("ru", "", 1, "@count view", "@count views"), "1 просмотр"},
		{tr.Plural("ru", "", 3, "@count view", "@count views"), "3 просмотра"},
		{tr.Plural("ru", "", 11, "@count view", "@count views"), "11 просмотров"},
		{tr.Plural("ru", "", 21, "@count view", "@count views"), "21 просмотр"},
		{tr.Plural("en", "", 1, "@count view", "@count views"), "1 view"},
		{tr.Plural("en", "", 5, "@count view", "@count views"), "5 views"},
		{tr.Plural("ru", "", 5, "@count reply", "@count replies"), "5 replies"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d: %q, want %q", i, c.got, c.want)
		}
	}
	if !tr.Has("ru", "", "Save") || tr.Has("ru", "", "Untranslated") {
		t.Fatal("Has")
	}
	if langs := tr.Languages(); len(langs) != 1 || langs[0] != "ru" {
		t.Fatalf("Languages = %v", langs)
	}
}

func TestFormatLongestFirst(t *testing.T) {
	if got := Format("@count of @countdown", "@count", 1, "@countdown", 9); got != "1 of 9" {
		t.Fatalf("got %q", got)
	}
	if got := Format("x", "@a"); got != "x" {
		t.Fatalf("odd args: %q", got)
	}
}

func TestContextLang(t *testing.T) {
	t.Cleanup(func() { SetDefault("") })
	SetDefault("ru")
	if FromContext(t.Context()) != "ru" || FromContext(WithLang(t.Context(), "en")) != "en" {
		t.Fatal("FromContext")
	}
	SetTranslator(testTranslator(t))
	t.Cleanup(func() { SetTranslator(&Translator{chain: map[string][]catalog{}}) })
	if T(t.Context(), "Save") != "Сохранить" || Plural(t.Context(), 2, "@count view", "@count views") != "2 просмотра" || TC(t.Context(), "Long month name", "May") != "май" {
		t.Fatal("package funcs")
	}
}

func TestFormatURL(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"/login?destination=forum/тема#x", "/login?destination=forum/%D1%82%D0%B5%D0%BC%D0%B0#x"},
		{"https://example.com/a b", "https://example.com/a%20b"},
		{"javascript:alert(1)", ""},
		{"/a?x=1&y=2", "/a?x=1&amp;y=2"},
	} {
		if got := Format(`<a href=":u">x</a>`, ":u", c.in); got != `<a href="`+c.want+`">x</a>` {
			t.Errorf("%q: got %s", c.in, got)
		}
	}
}

func TestGoTextAndErrors(t *testing.T) {
	tr := &Translator{chain: map[string][]catalog{}}
	if got := tr.Text("ru", "", "Hi @name & %who", "@name", "<b>", "%who", "x"); got != "Hi <b> & x" {
		t.Errorf("Text: %q", got)
	}
	if got := tr.T("ru", "", "Hi @name", "@name", "<b>"); got != "Hi &lt;b&gt;" {
		t.Errorf("T: %q", got)
	}
	err := fmt.Errorf("save: %w", Errorf("User @name exists.", "@name", "bob"))
	if err.Error() != "save: User bob exists." {
		t.Errorf("Error(): %q", err.Error())
	}
	if got := ErrorText(context.Background(), err); got != "User bob exists." {
		t.Errorf("ErrorText: %q", got)
	}
	if got := ErrorText(context.Background(), errors.New("open /srv/site/x: denied")); got != "The operation failed. Try again." {
		t.Errorf("ErrorText of an internal error: %q", got)
	}
	if !IsUserError(err) || IsUserError(errors.New("plain")) || IsUserError(nil) {
		t.Error("IsUserError")
	}
	if N("Label") != "Label" {
		t.Error("N")
	}
}

func TestFormatPointers(t *testing.T) {
	id := 42
	var nilID *int
	if got := FormatText("#@id #@nil", "@id", &id, "@nil", nilID); got != "#42 #" {
		t.Errorf("got %q", got)
	}
}

// TestWithSite - the site's translation wins over the catalogs, Default
// leaves it out, an English rewording stands in for a language that
// doesn't translate the source, plural forms too.
func TestWithSite(t *testing.T) {
	base, err := Load(fstest.MapFS{"ru.json": {Data: []byte(`{"strings":{"Add comment":"Добавить комментарий","@count comment":["@count комментарий","@count комментария","@count комментариев"]}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	tr := base.WithSite([]Entry{
		{Lang: "ru", Source: "Add comment", Text: "Написать отзыв"},
		{Lang: "en", Source: "Log in", Text: "Sign in"},
		{Lang: "en", Source: "@count reply", Forms: []string{"@count answer", "@count answers"}},
	})
	if got := tr.Text("ru", "", "Add comment"); got != "Написать отзыв" {
		t.Errorf("site's: %q", got)
	}
	if text, _, ok := tr.Default("ru", "", "Add comment"); !ok || text != "Добавить комментарий" {
		t.Errorf("default: %q %v", text, ok)
	}
	if text, _, ok := tr.Site("ru", "", "Add comment"); !ok || text != "Написать отзыв" {
		t.Errorf("site: %q %v", text, ok)
	}
	if got := base.Text("ru", "", "Add comment"); got != "Добавить комментарий" {
		t.Errorf("the base changed: %q", got)
	}
	for lang, want := range map[string]string{"en": "Sign in", "ru": "Sign in", "de": "Sign in"} {
		if got := tr.Text(lang, "", "Log in"); got != want {
			t.Errorf("%s: %q, want the site's English %q", lang, got, want)
		}
	}
	if got := tr.PluralText("ru", "", 5, "@count reply", "@count replies"); got != "5 answers" {
		t.Errorf("the site's English plural in Russian: %q", got)
	}
	if got := tr.PluralText("ru", "", 5, "@count comment", "@count comments"); got != "5 комментариев" {
		t.Errorf("Russian plural: %q", got)
	}
}

// TestWithPacks - a downloaded language is a language of the translator,
// its pack after the theme's catalog (the theme's string wins), its
// plural forms by its rules; the site's own still first; the base is left
// as it was.
func TestWithPacks(t *testing.T) {
	theme := fstest.MapFS{"de.json": {Data: []byte(`{"strings":{"Comments":"Kommentare (Thema)"}}`)}}
	base, err := Load(theme, fstest.MapFS{"ru.json": {Data: []byte(`{"strings":{"Save":"Сохранить"}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	tr := base.WithSite([]Entry{{Lang: "de", Source: "Log in", Text: "Einloggen"}}).WithPacks([]Entry{
		{Lang: "de", Source: "Save", Text: "Speichern"},
		{Lang: "de", Source: "Comments", Text: "Kommentare"},
		{Lang: "de", Source: "Log in", Text: "Anmelden"},
		{Lang: "de", Context: "menu", Source: "Open", Text: "Öffnen"},
		{Lang: "de", Source: "@count view", Forms: []string{"@count Aufruf", "@count Aufrufe"}},
	})
	if got := tr.Languages(); !slices.Equal(got, []string{"de", "ru"}) {
		t.Errorf("languages: %v", got)
	}
	for _, c := range []struct{ ctx, source, want string }{
		{"", "Save", "Speichern"},
		{"", "Comments", "Kommentare (Thema)"},
		{"", "Log in", "Einloggen"},
		{"menu", "Open", "Öffnen"},
	} {
		if got := tr.Text("de", c.ctx, c.source); got != c.want {
			t.Errorf("%s: %q, want %q", c.source, got, c.want)
		}
	}
	if text, _, ok := tr.Default("de", "", "Save"); !ok || text != "Speichern" {
		t.Errorf("default: %q %v", text, ok)
	}
	if got := tr.PluralText("de", "", 5, "@count view", "@count views"); got != "5 Aufrufe" {
		t.Errorf("plural: %q", got)
	}
	if got := tr.Text("ru", "", "Save"); got != "Сохранить" {
		t.Errorf("ru: %q", got)
	}
	if got := base.Languages(); !slices.Equal(got, []string{"de", "ru"}) || base.Text("de", "", "Save") != "Save" {
		t.Errorf("the base changed: %v %q", got, base.Text("de", "", "Save"))
	}
	if again := tr.WithPacks(nil); again.Text("de", "", "Save") != "Speichern" {
		t.Error("another layer lost the packs")
	}
}

// TestParseCatalog - a catalog file's entries: plain, plural, in a
// context; sorted; a broken file is an error.
func TestParseCatalog(t *testing.T) {
	got, err := ParseCatalog("de", []byte(`{
		"strings": {"Save": "Speichern", "@count view": ["@count Aufruf", "@count Aufrufe"]},
		"contexts": {"menu": {"Open": "Öffnen"}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Lang: "de", Source: "@count view", Forms: []string{"@count Aufruf", "@count Aufrufe"}},
		{Lang: "de", Source: "Save", Text: "Speichern"},
		{Lang: "de", Context: "menu", Source: "Open", Text: "Öffnen"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if _, err := ParseCatalog("de", []byte(`{"strings": [`)); err == nil {
		t.Error("a broken catalog parsed")
	}
}
