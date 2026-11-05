package languages

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/languages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// useLanguages - an English site with Russian at /ru/, the module on.
func useLanguages(t *testing.T) {
	t.Helper()
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	modulekit.SetLanguagesSwitch(manifest.Name)
	modulekit.ApplySiteLanguages("en", []modulekit.SiteLanguage{{Code: "ru", Prefix: "ru"}})
	t.Cleanup(func() {
		modulekit.ApplySiteLanguages("en", nil)
		modulekit.SetLanguagesSwitch("")
	})
}

// TestSwitcherLinks - a page's link in each language at its prefix, the
// shown one current; a node's page sends to the others' front pages.
func TestSwitcherLinks(t *testing.T) {
	useLanguages(t)
	links := switcherLinks([]string{"en", "ru"}, "ru", "/news", false, nil)
	if len(links) != 2 || links[0] != (Link{Code: "en", Name: "English", Label: "EN", Href: "/news"}) ||
		links[1] != (Link{Code: "ru", Name: "Русский", Label: "RU", Href: "/ru/news", Current: true}) {
		t.Errorf("a page's links: %+v", links)
	}
	links = switcherLinks([]string{"en", "ru"}, "en", "/news/x", true, nil)
	if links[0].Href != "/news/x" || !links[0].Current || links[1].Href != "/ru/" || !links[1].Home {
		t.Errorf("a node's links: %+v", links)
	}
	// A public translation is linked; an unpublished one isn't.
	ru := Version{Node: modulekit.Node{Type: "article", Slug: "novosti/x", Langcode: "ru", Status: 1}}
	links = switcherLinks([]string{"en", "ru"}, "en", "/news/x", true, map[string]Version{"ru": ru})
	if links[1].Href != "/ru/novosti/x" || links[1].Home {
		t.Errorf("a translated node's links: %+v", links)
	}
	ru.Node.Status = 0
	links = switcherLinks([]string{"en", "ru"}, "en", "/news/x", true, map[string]Version{"ru": ru})
	if !links[1].Home {
		t.Errorf("an unpublished translation is linked: %+v", links)
	}
}

// TestPageLanguages - the front page gets the links and its addresses in
// both languages plus x-default; another page the links only; the
// module off - nothing.
func TestPageLanguages(t *testing.T) {
	useLanguages(t)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(i18n.WithLang(r.Context(), "ru"))
	data := map[string]any{}
	pageData{}.add(r, data)
	alternates, _ := data["languageAlternates"].([]Alternate)
	if len(alternates) != 3 || alternates[0].Lang != "en" || alternates[1] != (Alternate{Lang: "ru", Href: alternates[0].Href + "ru/"}) || alternates[2].Lang != "x-default" {
		t.Errorf("the front page's alternates: %+v", alternates)
	}
	if links, _ := data["languageLinks"].([]Link); len(links) != 2 || !links[1].Current {
		t.Errorf("the front page's links: %+v", links)
	}
	data = map[string]any{}
	pageData{}.add(httptest.NewRequest(http.MethodGet, "/news", nil), data)
	if _, ok := data["languageAlternates"]; ok || data["languageLinks"] == nil {
		t.Errorf("another page: %v", data)
	}
	testkit.WithModules(t, map[string]bool{manifest.Name: false})
	data = map[string]any{}
	pageData{}.add(httptest.NewRequest(http.MethodGet, "/", nil), data)
	if len(data) != 0 {
		t.Errorf("the module off: %v", data)
	}
}

// TestNodeAlternates - a content item's public versions, two at least,
// x-default the default language's.
func TestNodeAlternates(t *testing.T) {
	useLanguages(t)
	en := Version{Node: modulekit.Node{Type: "article", Slug: "news/x", Langcode: "en", Status: 1}}
	ru := Version{Node: modulekit.Node{Type: "article", Slug: "novosti/x", Langcode: "ru", Status: 1}}
	got := nodeAlternates("https://site.example", []string{"en", "ru"}, map[string]Version{"en": en, "ru": ru})
	want := []Alternate{{"en", "https://site.example/news/x"}, {"ru", "https://site.example/ru/novosti/x"}, {"x-default", "https://site.example/news/x"}}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("alternates = %v", got)
	}
	ru.Node.Status = 0
	if got := nodeAlternates("https://site.example", []string{"en", "ru"}, map[string]Version{"en": en, "ru": ru}); got != nil {
		t.Errorf("one public version: %v", got)
	}
}

// TestTranslationInfo - a translation names its original and whether it
// is outdated; the original names its translations; an item alone -
// none.
func TestTranslationInfo(t *testing.T) {
	versions := []Version{
		{Member: Member{Nid: 1, Original: true}, Node: modulekit.Node{ID: 1, Langcode: "en"}},
		{Member: Member{Nid: 2, Outdated: true}, Node: modulekit.Node{ID: 2, Langcode: "ru"}},
	}
	if info, ok := translationInfo(2, versions); !ok || info.Language != "Русский" || info.Original == nil || info.Original.Node.ID != 1 || !info.Outdated {
		t.Errorf("the translation's: %+v %v", info, ok)
	}
	if info, ok := translationInfo(1, versions); !ok || info.Original != nil || len(info.Translations) != 1 || info.Translations[0] != "Русский" {
		t.Errorf("the original's: %+v %v", info, ok)
	}
	if _, ok := translationInfo(1, versions[:1]); ok {
		t.Error("an item alone has a set")
	}
}

// TestPageAlternates - a page other than the front page or a content
// item's gets its addresses in every language for its template to show
// (a View's, a term's), x-default - the default language's.
func TestPageAlternates(t *testing.T) {
	useLanguages(t)
	data := map[string]any{}
	pageData{}.add(httptest.NewRequest(http.MethodGet, "/events", nil), data)
	all, _ := data["languagePageAlternates"].([]Alternate)
	if _, ok := data["languageAlternates"]; ok || len(all) != 3 || all[1].Href != all[0].Href[:len(all[0].Href)-len("/events")]+"/ru/events" || all[2].Lang != "x-default" || all[2].Href != all[0].Href {
		t.Errorf("the page's alternates: %+v / %v", all, data["languageAlternates"])
	}
}
