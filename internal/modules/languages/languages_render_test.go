package languages

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/languages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestLanguagesPage - the default language first, without a prefix and
// actions; another with its prefix, its content, Make default (it has
// the interface) and Remove with their warnings; one without the
// interface has no Make default; the form offers the languages the site
// hasn't; an error explains the taken prefix.
func TestLanguagesPage(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	rows := []row{
		{Code: "en", Name: "English", Default: true, Nodes: 12, Interface: true},
		{Code: "ru", Name: "Русский", Prefix: "ru", Nodes: 3, Interface: true},
		{Code: "fr", Name: "Français", Prefix: "fr"},
	}
	out := testkit.RenderPageContent(t, "admin/languages.html", map[string]any{"rows": rows, "sourceName": "English",
		"types": []typeChoice{{Name: "article", Label: "Article", On: true}, {Name: "page", Label: "Page"}}, "untranslated": UntranslatedHide,
		"choices": []i18n.LanguageOption{{Code: "de", Label: "Deutsch"}}, "error": "prefix-taken", "owner": "routes"})
	for _, want := range []string{
		`<span class="admin-table-title" lang="en">English</span> <span class="admin-pill admin-pill--ok">default</span>`,
		`no prefix`, `<code>/ru/</code>`, `<td>3</td>`,
		`action="/admin/config/languages/ru/default"`,
		`data-confirm="Make Русский the default language? Its addresses lose /ru/, those of English get /en/."`,
		`action="/admin/config/languages/ru/remove"`,
		`data-confirm="Remove Русский? Its content (3) stays, at addresses without /ru/."`,
		`action="/admin/config/languages/fr/remove"`, `no: shown in English`,
		`<option value="de" lang="de">Deutsch - de</option>`,
		`The site has an address like this prefix (routes): a prefix would hide it.`,
		`name="translatable" value="article" checked>`, `name="translatable" value="page">`,
		`name="untranslated" value="hide" checked>`, `name="untranslated" value="original">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	for _, unwanted := range []string{`/admin/config/languages/en/`, `/admin/config/languages/fr/default`} {
		if strings.Contains(out, unwanted) {
			t.Errorf("%s in %s", unwanted, out)
		}
	}
}

// TestSwitcherBlock - the block lists the page's links, the current one
// marked, one without the page's version titled as the front page.
func TestSwitcherBlock(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	id := modulekit.PluginBlockID(manifest.PluginBlockSwitcher)
	links := []Link{{Code: "en", Name: "English", Label: "EN", Href: "/news/x", Current: true}, {Code: "ru", Name: "Русский", Label: "RU", Href: "/ru/", Home: true}}
	out := testkit.RenderPlacement(t, modulekit.BlockPlacement{ID: 1, PlacementType: modulekit.BlockPlacementPlugin, PluginID: &id, Region: "header", VisibilityScope: modulekit.BlockVisibilityAll}, map[string]any{"languageLinks": links})
	for _, want := range []string{
		`<nav class="language-switcher" aria-label="Language">`,
		`<a class="nav-link active" href="/news/x" lang="en" title="English" aria-current="true">EN</a>`,
		`<a class="nav-link" href="/ru/" lang="ru" hreflang="ru" title="Русский: the home page">RU</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if out := testkit.RenderPlacement(t, modulekit.BlockPlacement{ID: 1, PlacementType: modulekit.BlockPlacementPlugin, PluginID: &id, Region: "header", VisibilityScope: modulekit.BlockVisibilityAll}, map[string]any{}); strings.Contains(out, "language-switcher") {
		t.Errorf("one language: %s", out)
	}
}

// TestHreflangInHead - <head> has the module's alternates while it is
// on, none while it is off.
func TestHreflangInHead(t *testing.T) {
	testkit.UseEngineRoot(t)
	render := func() string {
		data := testkit.PageData()
		data["languageAlternates"] = []Alternate{{Lang: "en", Href: "https://site.example/"}, {Lang: "ru", Href: "https://site.example/ru/"}, {Lang: "x-default", Href: "https://site.example/"}}
		return testkit.RenderPageBlock(t, "errors/not_found.html", "html_start", data)
	}
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	out := render()
	for _, want := range []string{
		`<link rel="alternate" hreflang="ru" href="https://site.example/ru/">`,
		`<link rel="alternate" hreflang="x-default" href="https://site.example/">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in the head", want)
		}
	}
	testkit.WithModules(t, map[string]bool{manifest.Name: false})
	if strings.Contains(render(), "hreflang") {
		t.Error("the module off: hreflang in the head")
	}
}

// TestTranslationsPage - the item's actions with Translations marked;
// the original, a translation marked outdated with Edit, a language
// without a version with Translate.
func TestTranslationsPage(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	en := Version{Member: Member{Nid: 1, Original: true}, Node: modulekit.Node{ID: 1, Type: "article", Title: "We reopened", Slug: "news/we-reopened", Langcode: "en", Status: 1}}
	ru := Version{Member: Member{Nid: 2, Outdated: true}, Node: modulekit.Node{ID: 2, Type: "article", Title: "Мы открылись", Slug: "news/my-otkrylis", Langcode: "ru"}}
	data := map[string]any{"item": struct{ Node modulekit.Node }{en.Node}, "canEdit": true, "translationsPage": true, "canTranslate": true,
		"moduleNode": map[string]any{manifest.Name: map[string]any{"translatable": true}},
		"rows":       []translationRow{{Code: "en", Name: "English", Version: &en, Current: true}, {Code: "ru", Name: "Русский", Version: &ru}, {Code: "de", Name: "Deutsch"}}}
	out := testkit.RenderPageContent(t, "languages/translations.html", data)
	for _, want := range []string{
		`<a href="/node/1/translations" class="action-link translations" aria-current="page">Translations</a>`,
		`<tr class="table-active">`, `original`,
		`<a href="/news/my-otkrylis">Мы открылись</a>`, `Not published`, `Outdated: the original changed`,
		`<a href="/node/2/edit"`, `action="/node/1/translations/de"`, `>Translate</button>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}

// TestTranslationInfoNote - over a translation's form its language, its
// original and the outdated mark; over the original's, its translations.
func TestTranslationInfoNote(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	original := Version{Member: Member{Nid: 1, Original: true}, Node: modulekit.Node{ID: 1, Type: "article", Title: "We reopened", Slug: "news/we-reopened", Langcode: "en"}}
	out := testkit.RenderNamed(t, "node_form_top_languages", map[string]any{"translationInfo": TranslationInfo{Language: "Русский", Original: &original, Outdated: true}}, "partials/languages_hooks.html")
	for _, want := range []string{"Translation into Русский", `<a href="/news/we-reopened" lang="en"`, "Outdated: the original changed"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	out = testkit.RenderNamed(t, "node_form_top_languages", map[string]any{"translationInfo": TranslationInfo{Language: "English", Translations: []string{"Русский", "Deutsch"}}}, "partials/languages_hooks.html")
	if !strings.Contains(out, "The original (English)") || !strings.Contains(out, "Русский, Deutsch") {
		t.Errorf("the original's note: %s", out)
	}
}

// TestLanguageSection - the form's Language: the item's language
// selected among the offered.
func TestLanguageSection(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	data := map[string]any{"moduleSettings": map[string]any{manifest.Name: map[string]any{"Current": "ru", "CurrentName": "Русский",
		"Choices": []languageChoice{{Code: "en", Name: "English"}, {Code: "ru", Name: "Русский"}}}}}
	out := testkit.RenderNamed(t, "node_settings_languages", data, "partials/languages_hooks.html")
	for _, want := range []string{`<span class="node-settings-value">Русский</span>`, `name="langcode"`, `<option value="en" lang="en">English</option>`, `<option value="ru" lang="ru" selected>Русский</option>`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if out := testkit.RenderNamed(t, "node_settings_languages", map[string]any{"moduleSettings": map[string]any{}}, "partials/languages_hooks.html"); strings.Contains(out, "langcode") {
		t.Errorf("a site of one language: %s", out)
	}
}

// TestTeaserLanguageMark - a teaser of content in another language than
// the page's is marked with it; of the page's, not.
func TestTeaserLanguageMark(t *testing.T) {
	testkit.UseEngineRoot(t)
	useLanguages(t)
	render := func(lang string) string {
		item := modulekit.TeaserItem{Node: modulekit.Node{ID: 1, Type: "article", Title: "Море", Slug: "more", Langcode: lang}}
		return testkit.RenderNamed(t, "teaser_default", item, "teaser/default.html")
	}
	if out := render("ru"); !strings.Contains(out, `lang="ru">Море</a> <span class="badge text-bg-secondary node-teaser-language">Русский</span>`) {
		t.Errorf("a Russian teaser on an English page: %s", out)
	}
	if out := render("en"); strings.Contains(out, "node-teaser-language") {
		t.Errorf("an English teaser on an English page: %s", out)
	}
}

// TestMenuLinkSection - a menu link's form: its languages (some chosen)
// and its name in the other languages.
func TestMenuLinkSection(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	data := map[string]any{"moduleForms": map[string]map[string]any{"menu_link": {manifest.Name: map[string]any{
		"Texts": []textRow{{Code: "en", Name: "English", Prefix: "en", Values: []textValue{{Field: textField{Name: "text"}, Value: "News"}}}}, "Field": "text",
		"Languages": []languageOption{{Code: "ru", Name: "Русский", On: true}, {Code: "en", Name: "English"}},
		"Only":      true,
	}}}}
	out := testkit.RenderNamed(t, "form_menu_link_languages", data, "partials/languages_hooks.html")
	for _, want := range []string{
		`name="languages_only" value="1" checked>`, `name="languages" value="ru" checked>`, `name="languages" value="en">`,
		`Name in other languages`, `name="text_en" class="form-control" value="News" lang="en">`, `/en/ · empty - as in the default language`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}

// TestBlockPlacementSection - a placement's languages and title in the
// other languages; on a custom block's page, its languages only (the
// title is the block's).
func TestBlockPlacementSection(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	section := map[string]any{"Texts": []textRow{{Code: "ru", Name: "Русский", Prefix: "ru", Values: []textValue{{Field: textField{Name: "placement_title"}, Value: "Популярное"}}}}, "Field": "placement_title",
		"Languages": []languageOption{{Code: "en", Name: "English"}, {Code: "ru", Name: "Русский"}}}
	data := map[string]any{"moduleForms": map[string]map[string]any{"block_placement": {manifest.Name: section}}}
	out := testkit.RenderNamed(t, "form_block_placement_languages", data, "partials/languages_hooks.html")
	for _, want := range []string{`On the pages of which languages the block shows.`, `name="placement_title_ru" class="form-control" value="Популярное"`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	data["block"] = map[string]any{"ID": 3}
	if out := testkit.RenderNamed(t, "form_block_placement_languages", data, "partials/languages_hooks.html"); strings.Contains(out, "placement_title_ru") || !strings.Contains(out, `name="languages"`) {
		t.Errorf("a custom block's page: %s", out)
	}
}

// TestBlockInLanguage - a placement's title in the page's language; a
// placement for other languages' pages not shown.
func TestBlockInLanguage(t *testing.T) {
	testkit.UseEngineRoot(t)
	useLanguages(t)
	modulekit.SetTextTranslator(manifest.Name, func(lang, kind, ref string) (string, bool) {
		return "Language", lang == "en" && kind == modulekit.TextPlacementTitle && ref == "1"
	})
	modulekit.SetLanguageVisibility(manifest.Name, func(kind, ref, lang string) bool { return ref != "2" })
	t.Cleanup(func() {
		modulekit.SetTextTranslator(manifest.Name, nil)
		modulekit.SetLanguageVisibility(manifest.Name, nil)
	})
	id := modulekit.PluginBlockID(manifest.PluginBlockSwitcher)
	links := map[string]any{"languageLinks": []Link{{Code: "en", Name: "English", Label: "EN", Href: "/", Current: true}}}
	place := func(pid int) modulekit.BlockPlacement {
		return modulekit.BlockPlacement{ID: pid, PlacementType: modulekit.BlockPlacementPlugin, PluginID: &id, Region: "header", VisibilityScope: modulekit.BlockVisibilityAll, Title: "Язык", ShowTitle: true}
	}
	if out := testkit.RenderPlacement(t, place(1), links); !strings.Contains(out, "<h2>Language</h2>") {
		t.Errorf("the title in English: %s", out)
	}
	if out := testkit.RenderPlacement(t, place(2), links); strings.Contains(out, "language-switcher") {
		t.Errorf("a placement for other languages shown: %s", out)
	}
}

// TestTermSection - a term's name and description in each other
// language, labelled.
func TestTermSection(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	name, desc := textField{Name: "term_name", Label: "Name"}, textField{Name: "term_description", Label: "Description", Multiline: true}
	data := map[string]any{"moduleForms": map[string]map[string]any{"term": {manifest.Name: map[string]any{
		"Texts": []textRow{{Code: "en", Name: "English", Prefix: "en", Values: []textValue{{Field: name, Value: "News"}, {Field: desc, Value: "<p>What's new.</p>"}}}},
		"Field": "term_name", "Multi": true,
	}}}}
	out := testkit.RenderNamed(t, "form_term_languages", data, "partials/languages_hooks.html")
	for _, want := range []string{"In other languages", `>Name</label>`, `name="term_name_en" class="form-control" value="News"`,
		`>Description</label>`, `<textarea id="term_description-en" name="term_description_en" class="form-control" rows="3" lang="en">&lt;p&gt;What&#39;s new.&lt;/p&gt;</textarea>`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}

// TestViewSection - a View's title and meta description in the other
// languages, in its settings modal's layout; hreflang of a View's page.
func TestViewSection(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	title, desc := textField{Name: "view_title", Label: "Title"}, textField{Name: "view_description", Label: "Meta description"}
	data := map[string]any{"moduleForms": map[string]map[string]any{"view": {manifest.Name: map[string]any{
		"Texts": []textRow{{Code: "en", Name: "English", Prefix: "en", Values: []textValue{{Field: title, Value: "Events"}, {Field: desc}}}},
	}}}}
	out := testkit.RenderNamed(t, "form_view_languages", data, "partials/languages_hooks.html")
	for _, want := range []string{`<legend class="fs-6 mb-1" lang="en">English`, `name="view_title_en" class="form-control form-control-sm mb-2" value="Events"`, `name="view_description_en"`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	alts := []Alternate{{Lang: "en", Href: "https://site.example/events"}, {Lang: "ru", Href: "https://site.example/ru/events"}}
	if out := testkit.RenderNamed(t, "head_languages", map[string]any{"languagePageAlternates": alts, "viewURLPath": "events"}, "partials/languages_hooks.html"); !strings.Contains(out, `hreflang="ru" href="https://site.example/ru/events"`) {
		t.Errorf("a View's page: %s", out)
	}
	if out := testkit.RenderNamed(t, "head_languages", map[string]any{"languagePageAlternates": alts}, "partials/languages_hooks.html"); strings.Contains(out, "hreflang") {
		t.Errorf("another page: %s", out)
	}
}
