package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// memInterfaceStrings - entity.InterfaceStringRepository in memory.
type memInterfaceStrings struct {
	rows []entity.InterfaceString
}

func (m *memInterfaceStrings) All(context.Context) ([]entity.InterfaceString, error) {
	return slices.Clone(m.rows), nil
}

func (m *memInterfaceStrings) Save(_ context.Context, s entity.InterfaceString) error {
	for i, r := range m.rows {
		if r.Lang == s.Lang && r.Context == s.Context && r.Source == s.Source {
			m.rows[i] = s
			return nil
		}
	}
	m.rows = append(m.rows, s)
	return nil
}

func (m *memInterfaceStrings) Delete(_ context.Context, lang, msgctx, source string) error {
	m.rows = slices.DeleteFunc(m.rows, func(r entity.InterfaceString) bool {
		return r.Lang == lang && r.Context == msgctx && r.Source == source
	})
	return nil
}

// testInterfaceStrings - the service on a catalog of its own (ru: three
// of the strings, the plural one included) as the site's translator,
// rebuilt with the downloaded and the site's strings on a change as the
// app does; the
// strings: the core's, an enabled and a disabled module's, an inactive
// theme's. The translator and the modules are restored after the test.
func testInterfaceStrings(t *testing.T) (*InterfaceStrings, *memInterfaceStrings) {
	t.Helper()
	catalog := fstest.MapFS{"ru.json": {Data: []byte(`{
		"strings": {
			"Comments": "Комментарии",
			"@count comment": ["@count комментарий", "@count комментария", "@count комментариев"],
			"Polls": "Опросы"
		},
		"contexts": {"menu": {"Open": "Открыть"}}
	}`)}}
	base, err := i18n.Load(catalog)
	if err != nil {
		t.Fatal(err)
	}
	old := i18n.Current()
	t.Cleanup(func() { i18n.SetTranslator(old) })
	i18n.SetTranslator(base)
	SetModules(map[string]bool{"zz_on": true, "zz_off": false})
	t.Cleanup(func() { SetModules(map[string]bool{"zz_off": true}) })

	repo := &memInterfaceStrings{}
	s := NewInterfaceStrings(repo, []i18n.Source{
		{Source: "Comments"},
		{Source: "@count comment", Plural: "@count comments"},
		{Context: "menu", Source: "Open"},
		{Source: `<a href=":login">Log in</a> to post comments`},
		{Source: "Polls", Module: "zz_on"},
		{Source: "Hidden", Module: "zz_off"},
		{Source: "Elsewhere", Theme: "zz-inactive"},
	})
	s.SetReload(func(ctx context.Context) error {
		packs, err := s.PackEntries(ctx)
		if err != nil {
			return err
		}
		entries, err := s.Entries(ctx)
		if err != nil {
			return err
		}
		i18n.SetTranslator(base.WithPacks(packs).WithSite(entries))
		return nil
	})
	return s, repo
}

// sources - the rows' sources, joined.
func sources(rows []InterfaceStringRow) string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Source.Source)
	}
	return strings.Join(out, "; ")
}

// TestInterfaceStringsList - the strings the site shows (not a disabled
// module's or an inactive theme's) in a language: the catalog's
// translation, untranslated, the source itself in English; filtered by
// a text (the translation's too, any case), untranslated or changed,
// where from; a page of them with the language's counts.
func TestInterfaceStringsList(t *testing.T) {
	s, _ := testInterfaceStrings(t)
	ctx := context.Background()
	list := func(f InterfaceStringsFilter) InterfaceStringsPage {
		t.Helper()
		p, err := s.List(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}

	p := list(InterfaceStringsFilter{Lang: "ru"})
	if got, want := sources(p.Rows), `Comments; @count comment; Open; <a href=":login">Log in</a> to post comments; Polls`; got != want {
		t.Errorf("rows = %s, want %s", got, want)
	}
	if p.Total != 5 || p.Untranslated != 1 || p.Edited != 0 {
		t.Errorf("total %d, untranslated %d, edited %d; want 5, 1, 0", p.Total, p.Untranslated, p.Edited)
	}
	if !slices.Equal(p.Origins, []string{"core", "module:zz_on"}) {
		t.Errorf("origins = %v", p.Origins)
	}
	r := p.Rows[1]
	if !r.IsPlural() || !slices.Equal(r.DefaultForms, []string{"@count комментарий", "@count комментария", "@count комментариев"}) || r.Untranslated {
		t.Errorf("plural row = %+v", r)
	}
	if r := p.Rows[2]; r.Default != "Открыть" || r.Context != "menu" {
		t.Errorf("context row = %+v", r)
	}

	for _, c := range []struct {
		f    InterfaceStringsFilter
		want string
	}{
		{InterfaceStringsFilter{Lang: "ru", Query: "КОММЕНТ"}, "Comments; @count comment"},
		{InterfaceStringsFilter{Lang: "ru", Query: "Log In"}, `<a href=":login">Log in</a> to post comments`},
		{InterfaceStringsFilter{Lang: "ru", Show: "untranslated"}, `<a href=":login">Log in</a> to post comments`},
		{InterfaceStringsFilter{Lang: "ru", From: "module:zz_on"}, "Polls"},
		{InterfaceStringsFilter{Lang: "ru", From: "core", Page: 2, PerPage: 3}, `<a href=":login">Log in</a> to post comments`},
		{InterfaceStringsFilter{Lang: "en", Show: "untranslated"}, ""},
	} {
		if got := sources(list(c.f).Rows); got != c.want {
			t.Errorf("%+v: %s, want %s", c.f, got, c.want)
		}
	}
	if p := list(InterfaceStringsFilter{Lang: "ru", From: "core", Page: 2, PerPage: 3}); p.Total != 4 {
		t.Errorf("a page's total = %d, want 4", p.Total)
	}
	en := list(InterfaceStringsFilter{Lang: "en", Query: "comment"})
	if len(en.Rows) != 3 || en.Rows[0].Default != "Comments" || !slices.Equal(en.Rows[1].DefaultForms, []string{"@count comment", "@count comments"}) {
		t.Errorf("en rows = %+v", en.Rows)
	}
}

// TestInterfaceStringsSave - a translation saved is the site's at once
// (the translator rebuilt) and the row's, changed; a plural one - a form
// per plural category; refused: an unknown string or language, empty,
// the wrong number of forms, a placeholder dropped, HTML the source
// hasn't; reset - the catalog's is back.
func TestInterfaceStringsSave(t *testing.T) {
	s, repo := testInterfaceStrings(t)
	ctx := context.Background()
	ru := i18n.WithLang(ctx, "ru")

	if err := s.Save(ctx, "ru", "", "Comments", " Отзывы ", nil); err != nil {
		t.Fatal(err)
	}
	if got := i18n.T(ru, "Comments"); got != "Отзывы" {
		t.Errorf("T after a save = %q", got)
	}
	if err := s.Save(ctx, "ru", "", "@count comment", "", []string{"@count отзыв", "@count отзыва", "@count отзывов"}); err != nil {
		t.Fatal(err)
	}
	if got := i18n.Plural(ru, 5, "@count comment", "@count comments"); got != "5 отзывов" {
		t.Errorf("plural after a save = %q", got)
	}
	if err := s.Save(ctx, "ru", "", `<a href=":login">Log in</a> to post comments`, `<a href=":login">Войдите</a>, чтобы оставить отзыв`, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, "en", "menu", "Open", "Expand", nil); err != nil {
		t.Fatal(err)
	}
	if got := i18n.TC(ctx, "menu", "Open"); got != "Expand" {
		t.Errorf("English after a save = %q", got)
	}
	p, err := s.List(ctx, InterfaceStringsFilter{Lang: "ru", Show: "edited"})
	if err != nil {
		t.Fatal(err)
	}
	if got := sources(p.Rows); got != `Comments; @count comment; <a href=":login">Log in</a> to post comments` || p.Untranslated != 0 {
		t.Errorf("edited = %s (untranslated %d)", got, p.Untranslated)
	}
	if r := p.Rows[0]; !r.Edited || r.Site != "Отзывы" || r.Default != "Комментарии" {
		t.Errorf("edited row = %+v", r)
	}

	var te *i18n.TranslationError
	for _, c := range []struct {
		name                   string
		lang, msgctx, src, txt string
		forms                  []string
		is                     error
		placeholder, tag       string
	}{
		{name: "unknown string", lang: "ru", src: "Nope", txt: "Нет", is: ErrUnknownString},
		{name: "disabled module's", lang: "ru", src: "Hidden", txt: "Скрыто", is: ErrUnknownString},
		{name: "unknown language", lang: "xx", src: "Comments", txt: "x", is: ErrUnknownString},
		{name: "wrong context", lang: "ru", msgctx: "other", src: "Open", txt: "x", is: ErrUnknownString},
		{name: "empty", lang: "ru", src: "Comments", txt: "  ", is: ErrEmptyTranslation},
		{name: "two forms of three", lang: "ru", src: "@count comment", forms: []string{"a", "b"}, is: ErrEmptyTranslation},
		{name: "an empty form", lang: "ru", src: "@count comment", forms: []string{"a", " ", "c"}, is: ErrEmptyTranslation},
		{name: "placeholder dropped", lang: "ru", src: `<a href=":login">Log in</a> to post comments`, txt: `<a href="/login">Войдите</a>`, placeholder: ":login"},
		{name: "a tag of its own", lang: "ru", src: "Comments", txt: `<script>x</script>`, tag: "<script>"},
		{name: "a changed tag", lang: "ru", src: `<a href=":login">Log in</a> to post comments`, txt: `<a href=":login" onclick="x">Войдите</a>`, tag: `<a href=":login" onclick="x">`},
		{name: "a tag not closed", lang: "ru", src: "Comments", txt: "a <b", tag: "<"},
	} {
		err := s.Save(ctx, c.lang, c.msgctx, c.src, c.txt, c.forms)
		switch {
		case c.is != nil && !errors.Is(err, c.is):
			t.Errorf("%s: %v, want %v", c.name, err, c.is)
		case c.is == nil && (!errors.As(err, &te) || te.Placeholder != c.placeholder || te.Tag != c.tag):
			t.Errorf("%s: %v (%+v), want placeholder %q tag %q", c.name, err, te, c.placeholder, c.tag)
		}
	}
	if got := i18n.T(ru, "Comments"); got != "Отзывы" {
		t.Errorf("a refused save changed the string: %q", got)
	}

	if err := s.Reset(ctx, "ru", "", "Comments"); err != nil {
		t.Fatal(err)
	}
	if got := i18n.T(ru, "Comments"); got != "Комментарии" {
		t.Errorf("T after a reset = %q", got)
	}
	if len(repo.rows) != 3 {
		t.Errorf("rows after a reset = %+v", repo.rows)
	}
}

// TestInterfaceStringsOutdated - a site's translation of a string the
// code no longer has is listed apart (its language only) and removed
// with a reset.
func TestInterfaceStringsOutdated(t *testing.T) {
	s, repo := testInterfaceStrings(t)
	ctx := context.Background()
	repo.rows = []entity.InterfaceString{
		{Lang: "ru", Source: "Old wording", Text: "Старое"},
		{Lang: "en", Source: "Old wording", Text: "Old"},
		{Lang: "ru", Source: "Comments", Text: "Отзывы"},
	}
	p, err := s.List(ctx, InterfaceStringsFilter{Lang: "ru"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Outdated) != 1 || p.Outdated[0].Text != "Старое" {
		t.Errorf("outdated = %+v", p.Outdated)
	}
	if err := s.Reset(ctx, "ru", "", "Old wording"); err != nil {
		t.Fatal(err)
	}
	if len(repo.rows) != 2 {
		t.Errorf("rows after removing = %+v", repo.rows)
	}
}

// TestInterfaceLanguages - each language of the interface with how many
// of the site's strings it translates (English - all, the source) and
// how many the site changed; the percent rounded down.
func TestInterfaceLanguages(t *testing.T) {
	s, _ := testInterfaceStrings(t)
	if err := s.Save(context.Background(), "ru", "", "Comments", "Отзывы", nil); err != nil {
		t.Fatal(err)
	}
	got := map[string]InterfaceLanguage{}
	for _, l := range s.Languages() {
		got[l.Code] = l
	}
	if en := got["en"]; !en.Source || en.Translated != 5 || en.Percent() != 100 {
		t.Errorf("en = %+v", en)
	}
	if ru := got["ru"]; ru.Total != 5 || ru.Translated != 4 || ru.Untranslated != 1 || ru.Edited != 1 || ru.Percent() != 80 {
		t.Errorf("ru = %+v (%d%%)", ru, ru.Percent())
	}
	if p := (InterfaceLanguage{Total: 3, Translated: 2}).Percent(); p != 66 {
		t.Errorf("2 of 3 = %d%%", p)
	}
}

// TestPluralExamples - numbers for each plural form of a language, the
// labels of a plural string's fields.
func TestPluralExamples(t *testing.T) {
	for lang, want := range map[string][][]int{
		"en": {{1}, {0, 2, 3}},
		"ru": {{1, 21, 31}, {2, 3, 4}, {0, 5, 6}},
	} {
		if got := PluralExamples(lang); !slices.EqualFunc(got, want, slices.Equal) {
			t.Errorf("%s: %v, want %v", lang, got, want)
		}
	}
}

// TestInterfaceStringsSaveSpaces - the spaces and line breaks around a
// translation go; a no-break space is a translation (a thousands
// separator), spaces alone aren't.
func TestInterfaceStringsSaveSpaces(t *testing.T) {
	s, repo := testInterfaceStrings(t)
	ctx := context.Background()
	if err := s.Save(ctx, "ru", "", "Comments", " ", nil); err != nil {
		t.Fatal(err)
	}
	if len(repo.rows) != 1 || repo.rows[0].Text != " " {
		t.Errorf("a no-break space: %+v", repo.rows)
	}
	if err := s.Save(ctx, "ru", "", "Comments", " \n Отзывы\t", nil); err != nil || repo.rows[0].Text != "Отзывы" {
		t.Errorf("trimmed: %+v (%v)", repo.rows, err)
	}
	if err := s.Save(ctx, "ru", "", "Comments", " \t\n ", nil); !errors.Is(err, ErrEmptyTranslation) {
		t.Errorf("spaces alone: %v", err)
	}
}
