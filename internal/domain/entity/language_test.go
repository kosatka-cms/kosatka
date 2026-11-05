package entity

import "testing"

// TestSiteLanguages - a language besides the default gets its prefix
// (one equal to the default or without a prefix is left out); a node's
// address carries its language's prefix, the default's none; switched
// off (the languages module disabled) - no prefixes.
func TestSiteLanguages(t *testing.T) {
	on := true
	SetLanguagesSwitch(func() bool { return on })
	SetSiteLanguages("ru", []SiteLanguage{{Code: "en", Prefix: "/en/"}, {Code: "ru", Prefix: "ru"}, {Code: "de"}})
	t.Cleanup(func() { SetSiteLanguages("", nil); SetLanguagesSwitch(nil) })
	def, others := SiteLanguages()
	if def != "ru" || len(others) != 1 || others[0] != (SiteLanguage{Code: "en", Prefix: "en"}) {
		t.Fatalf("SiteLanguages = %q %v", def, others)
	}
	if code, ok := LanguageOfPrefix("en"); !ok || code != "en" {
		t.Errorf("LanguageOfPrefix(en) = %q %v", code, ok)
	}
	if _, ok := LanguageOfPrefix("ru"); ok {
		t.Error("the default language has no prefix")
	}
	for in, want := range map[[2]string]string{
		{"en", "/news/x"}: "/en/news/x",
		{"en", "/"}:       "/en/",
		{"ru", "/news/x"}: "/news/x",
		{"de", "/news/x"}: "/news/x",
	} {
		if got := LocalPath(in[0], in[1]); got != want {
			t.Errorf("LocalPath(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
	if got := (Node{Type: "article", Slug: "news/x", Langcode: "en"}).URL(); got != "/en/news/x" {
		t.Errorf("an English node's URL = %q", got)
	}
	if got := (Node{Type: "article", Slug: "news/x", Langcode: "ru"}).URL(); got != "/news/x" {
		t.Errorf("a Russian node's URL = %q", got)
	}
	on = false
	if _, others := SiteLanguages(); len(others) != 0 {
		t.Errorf("switched off: SiteLanguages others = %v", others)
	}
	if got := LocalPath("en", "/news/x"); got != "/news/x" {
		t.Errorf("switched off: LocalPath(en) = %q", got)
	}
	if _, ok := LanguageOfPrefix("en"); ok {
		t.Error("switched off: the prefix en is a language")
	}
}

// TestMoveDefaultLanguage - the new default leaves the others, the old
// one joins them under its code.
func TestMoveDefaultLanguage(t *testing.T) {
	got := MoveDefaultLanguage([]SiteLanguage{{Code: "en", Prefix: "en"}, {Code: "de", Prefix: "de"}}, "ru", "en")
	want := []SiteLanguage{{Code: "de", Prefix: "de"}, {Code: "ru", Prefix: "ru"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("MoveDefaultLanguage = %v, want %v", got, want)
	}
}

// TestLocalizePath - the site's own address goes under the language's
// prefix; another site's, an anchor or one with a prefix already stay.
func TestLocalizePath(t *testing.T) {
	SetLanguagesSwitch(func() bool { return true })
	SetSiteLanguages("ru", []SiteLanguage{{Code: "en", Prefix: "en"}})
	t.Cleanup(func() { SetSiteLanguages("", nil); SetLanguagesSwitch(nil) })
	for in, want := range map[string]string{
		"/news?page=2":        "/en/news?page=2",
		"/":                   "/en/",
		"/en/news":            "/en/news",
		"/en?x=1":             "/en?x=1",
		"//cdn.example.com/x": "//cdn.example.com/x",
		"https://example.com": "https://example.com",
		"#top":                "#top",
	} {
		if got := LocalizePath("en", in); got != want {
			t.Errorf("LocalizePath(en, %q) = %q, want %q", in, got, want)
		}
	}
	if got := LocalizePath("ru", "/news"); got != "/news" {
		t.Errorf("the default language: %q", got)
	}
}

// TestLocalizeTerm - a term's name and description in a language; its
// page title follows a translated name; untranslated - as is.
func TestLocalizeTerm(t *testing.T) {
	SetTextTranslator(func(lang, kind, ref string) (string, bool) {
		switch {
		case lang == "en" && kind == TextTermName && ref == "5":
			return "News", true
		case lang == "en" && kind == TextTermDescription && ref == "5":
			return "<p>What's new.</p>", true
		}
		return "", false
	})
	t.Cleanup(func() { SetTextTranslator(nil) })
	term := TaxonomyTerm{ID: 5, Name: "Новости", Desc: "<p>Что нового.</p>", PageTitle: "Все новости"}
	if got := LocalizeTerm("en", term); got.Name != "News" || got.Desc != "<p>What's new.</p>" || got.PageTitle != "News" {
		t.Errorf("in English: %+v", got)
	}
	if got := LocalizeTerm("ru", term); got.Name != term.Name || got.Desc != term.Desc || got.PageTitle != term.PageTitle {
		t.Errorf("in Russian: %+v", got)
	}
}

// TestLocalizeView - a View's title and meta description in a language,
// its meta title following a translated title; fields' labels.
func TestLocalizeView(t *testing.T) {
	SetTextTranslator(func(lang, kind, ref string) (string, bool) {
		switch {
		case lang != "en":
			return "", false
		case kind == TextViewTitle && ref == "4":
			return "Events", true
		case kind == TextViewDescription && ref == "4":
			return "What's on.", true
		case kind == TextFieldLabel && ref == "9":
			return "Date", true
		}
		return "", false
	})
	t.Cleanup(func() { SetTextTranslator(nil) })
	v := LocalizeView("en", ViewDisplayDef{ID: 4, Label: "События", MetaTitle: "Все события", MetaDescription: "Афиша."})
	if v.Label != "Events" || v.MetaTitle != "Events" || v.MetaDescription != "What's on." {
		t.Errorf("the View in English: %+v", v)
	}
	fields := FieldInstanceDefs{{ID: 9, Label: "Дата"}, {ID: 10, Label: "Место"}}
	got := LocalizeFieldLabels("en", fields)
	if got[0].Label != "Date" || got[1].Label != "Место" || fields[0].Label != "Дата" {
		t.Errorf("the labels: %+v (the fields: %+v)", got, fields)
	}
}

// TestLocalizeSelfURL - a page's own address built from the request's
// path gets the page's language's prefix; content's own address and
// another one stay.
func TestLocalizeSelfURL(t *testing.T) {
	SetLanguagesSwitch(func() bool { return true })
	SetSiteLanguages("ru", []SiteLanguage{{Code: "en", Prefix: "en"}})
	t.Cleanup(func() { SetSiteLanguages("", nil); SetLanguagesSwitch(nil) })
	SetSiteDomain("https://example.com")
	t.Cleanup(func() { SetSiteDomain("") })
	for _, tc := range []struct{ u, current, want string }{
		{"https://example.com/videos", "/videos", "https://example.com/en/videos"},
		{"https://example.com/videos?page=2", "/videos", "https://example.com/en/videos?page=2"},
		{"https://example.com/", "/", "https://example.com/en/"},
		{"https://example.com/en/article/whale", "/article/whale", "https://example.com/en/article/whale"},
		{"https://example.com/article/whale", "", "https://example.com/article/whale"},
	} {
		if got := LocalizeSelfURL("en", tc.u, tc.current); got != tc.want {
			t.Errorf("%s at %q: %s, want %s", tc.u, tc.current, got, tc.want)
		}
	}
}
