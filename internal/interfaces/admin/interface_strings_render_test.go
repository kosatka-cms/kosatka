package admin

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestInterfaceLanguagesPageRender - the Interface translations page's
// tabs, "Languages" active; English - the source strings; another
// language's share translated as a bar and its figures (linked to its
// untranslated strings when it has some, a draft marked), how many the
// site changed (linked to them); the state of each and what can be done:
// a built-in one - nothing, a downloaded one - removed (not the site's
// language), updated when the catalog has another version, the catalog's
// others - downloaded; the catalog's address, when it was read, "Check
// for updates"; what came of an action.
func TestInterfaceLanguagesPageRender(t *testing.T) {
	checked := time.Date(2026, 11, 4, 15, 10, 0, 0, time.UTC)
	out := renderMailPage(t, "admin/config/translations.html", map[string]any{
		"siteLanguage": "fr",
		"done":         "The Deutsch translation is installed: 10 strings.",
		"downloaded":   true,
		"dropped":      "2 strings of the file were left out.",
		"page": service.InterfaceLanguagesPage{
			CatalogURL:     "https://example.com/translations/index.json",
			CatalogChecked: checked,
			Languages: []service.InterfaceLanguage{
				{Code: "en", Name: "English", Total: 10, Translated: 10, Source: true, State: service.PackSource},
				{Code: "ru", Name: "Русский", Total: 10, Translated: 10, Edited: 2, State: service.PackBuiltIn},
				{Code: "de", Name: "Deutsch", Total: 10, Translated: 7, Untranslated: 3, State: service.PackUpdate,
					Pack:    &entity.InterfacePack{Lang: "de", Version: "0.1.0"},
					Catalog: &service.CatalogLanguage{Code: "de", Version: "0.2.0", Draft: true}},
				{Code: "fr", Name: "Français", Total: 10, Translated: 10, State: service.PackInstalled,
					Pack: &entity.InterfacePack{Lang: "fr", Version: "0.1.0"}},
				{Code: "es", Name: "Español", Total: 10, Translated: 8, State: service.PackAvailable,
					Catalog: &service.CatalogLanguage{Code: "es", Version: "0.2.0"}},
			},
		},
	})
	for _, want := range []string{
		`<a class="nav-link active" aria-current="page" href="/admin/config/translations">Languages</a>`,
		`<a class="nav-link" href="/admin/config/translations/strings">Strings</a>`,
		`<div class="alert alert-success" role="status">The Deutsch translation is installed: 10 strings. It can be chosen as the interface language in <a href="/admin/config/system/site-information">Site information</a>.</div>`,
		`<div class="alert alert-warning" role="status">2 strings of the file were left out.</div>`,
		`<span class="admin-table-muted">the source strings</span>`,
		`<span class="admin-pill">Built in</span>`,
		`<span style="width: 100%"></span></div> 100% - 10 of 10 <span class="admin-table-sub"><a href="/admin/config/translations/strings?lang=ru&amp;show=edited">Changed on the site: 2</a></span>`,
		`<a href="/admin/config/translations/strings?lang=de&amp;show=untranslated" title="Untranslated: 3">70% - 7 of 10</a> <span class="admin-table-sub">a draft (machine translated)</span>`,
		`0.1.0 <span class="admin-table-muted">&rarr; 0.2.0</span>`,
		`<span class="admin-pill admin-pill--warn">Update available</span>`,
		`<input type="hidden" name="lang" value="de"> <button type="submit" class="btn btn-sm btn-primary">Update</button>`,
		`data-confirm="Remove the Deutsch translation? The site&#39;s own changes of its strings stay."`,
		`<span class="admin-pill admin-pill--ok">Installed</span>`,
		`<span class="admin-table-muted small">the site's language</span>`,
		`<span class="admin-pill">Available</span>`,
		`<input type="hidden" name="lang" value="es"> <button type="submit" class="btn btn-sm btn-primary">Download</button>`,
		`<code>https://example.com/translations/index.json</code>`,
		`04.11.2026`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if strings.Contains(out, `name="lang" value="fr"> <button type="submit" class="btn btn-sm btn-outline-secondary">Remove`) {
		t.Error("the site's language can be removed")
	}
	if actions := renderTranslationsActions(t, map[string]any{}); !strings.Contains(actions, `<form method="post" action="/admin/config/translations/check"><button type="submit" class="btn btn-outline-primary">Check for updates</button></form>`) {
		t.Errorf("no Check for updates in %s", actions)
	}
	if actions := renderTranslationsActions(t, map[string]any{"noCatalog": true}); strings.Contains(actions, "Check for updates") {
		t.Error("no catalog: Check for updates")
	}

	out = renderMailPage(t, "admin/config/translations.html", map[string]any{
		"noCatalog": true,
		"error":     "No translations catalog is configured.",
		"page":      service.InterfaceLanguagesPage{Languages: []service.InterfaceLanguage{{Code: "en", Name: "English", Source: true, State: service.PackSource}}},
	})
	for _, want := range []string{
		`<div class="alert alert-danger" role="alert">No translations catalog is configured.</div>`,
		`<span class="admin-table-muted">not set: other languages can't be downloaded</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no catalog: no %s in %s", want, out)
		}
	}
	if strings.Contains(out, "Checked") {
		t.Error("no catalog: checked")
	}
}

// renderTranslationsActions renders the Languages tab's actions over the
// page.
func renderTranslationsActions(t *testing.T, data map[string]any) string {
	t.Helper()
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	if err := ts.RenderPageBlock(&buf, "admin/config/translations.html", "page_actions", data); err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// TestInterfaceStringsPageRender - the "Strings" tab: the filter as
// asked; the untranslated count linked; the changes no longer in use
// counted, linked and listed with a button removing each; a string
// changed - the site's text, the default under it, set back to it beside
// the edit; an untranslated one - "Translate"; a plural one - a field per
// form labelled with numbers; a refused one - open with what was sent
// and why; each string's form keeps the filter.
func TestInterfaceStringsPageRender(t *testing.T) {
	f := service.InterfaceStringsFilter{Lang: "ru", Query: "comment", From: "core", Page: 2, PerPage: 50}
	rows := []service.InterfaceStringRow{
		{Source: i18n.Source{Source: "Comments"}, Default: "Комментарии", Site: "Отзывы", Edited: true},
		{Source: i18n.Source{Source: "@count comment", Plural: "@count comments"}, DefaultForms: []string{"@count комментарий", "@count комментария", "@count комментариев"}},
		{Source: i18n.Source{Source: `<a href=":login">Log in</a> to post comments`}, Default: `<a href=":login">Войдите</a>`},
		{Source: i18n.Source{Context: "menu", Source: "Reply", Module: "forum"}, Untranslated: true},
	}
	failed := &stringEdit{Source: `<a href=":login">Log in</a> to post comments`, Text: `<a href="/login">Войдите</a>`, Error: "The translation must keep :login."}
	out := renderMailPage(t, "admin/config/translations_strings.html", map[string]any{
		"filter":       f,
		"filtered":     true,
		"languages":    []i18n.LanguageOption{{Code: "en", Label: "English"}, {Code: "ru", Label: "Русский"}},
		"origins":      []stringOrigin{{Value: "core", Label: "Engine"}, {Value: "module:forum", Label: "Module: Forum"}},
		"originLabels": map[string]string{"core": "Engine", "module:forum": "Module: Forum"},
		"rows":         stringRows(rows, "ru", failed),
		"page": service.InterfaceStringsPage{Total: 54, Untranslated: 7,
			Outdated: []entity.InterfaceString{{Lang: "ru", Source: "@count old", Forms: []string{"@count а", "@count б", "@count в"}, UpdatedAt: time.Date(2026, 11, 3, 9, 30, 0, 0, time.UTC)}}},
		"pagination": pagination.New(2, 54, interfaceStringsPerPage, interfaceStringsListURL, url.Values{"lang": {"ru"}}),
	})
	keys := `<input type="hidden" name="lang" value="ru"> <input type="hidden" name="q" value="comment"> <input type="hidden" name="from" value="core"> <input type="hidden" name="page" value="2">`
	for _, want := range []string{
		`<a class="nav-link active" aria-current="page" href="/admin/config/translations/strings">Strings</a>`,
		`<option value="ru" selected>Русский</option>`,
		`<input type="search" id="strings-q" name="q" value="comment"`,
		`<option value="core" selected>Engine</option>`,
		`<a href="/admin/config/translations/strings?lang=ru" class="admin-filter-reset">Reset</a>`,
		`<a href="/admin/config/translations/strings?lang=ru&amp;show=untranslated">Untranslated: 7</a>`,
		`<a href="#outdated" class="admin-pill admin-pill--warn">1 change no longer in use</a>`,
		// Changed on the site.
		`<span class="admin-strings-text">Отзывы</span> <span class="admin-pill admin-pill--ok">Changed on the site</span> <span class="admin-table-sub">Default translation: Комментарии</span>`,
		keys + ` <input type="hidden" name="context" value=""> <input type="hidden" name="source" value="Comments">`,
		`<textarea id="string-0-text" name="text" rows="2" class="form-control">Отзывы</textarea>`,
		`<button type="submit" formaction="/admin/config/translations/strings/reset" class="btn btn-sm btn-link">Back to the default</button>`,
		`<form method="post" action="/admin/config/translations/strings/reset" class="admin-strings-reset">`,
		// Plural.
		`<span class="d-block">@count comment</span><span class="d-block">@count comments</span><span class="admin-table-sub">Plural</span>`,
		`@count комментарий / @count комментария / @count комментариев`,
		`<label for="string-1-form-0" class="admin-filter-label">1, 21, 31...</label> <input type="text" id="string-1-form-0" name="form_0" value="@count комментарий" class="form-control">`,
		`<label for="string-1-form-2" class="admin-filter-label">0, 5, 6...</label> <input type="text" id="string-1-form-2" name="form_2" value="@count комментариев" class="form-control">`,
		// Refused, open.
		`&lt;a href=&#34;:login&#34;&gt;Log in&lt;/a&gt; to post comments`,
		`<details class="admin-strings-edit" open>`,
		`class="form-control is-invalid" aria-describedby="string-2-error" autofocus>&lt;a href=&#34;/login&#34;&gt;Войдите&lt;/a&gt;</textarea>`,
		`<div id="string-2-error" class="invalid-feedback d-block">The translation must keep :login.</div>`,
		// Untranslated, in a context, a module's.
		`Reply <span class="admin-table-sub">Context: menu</span>`,
		`<span class="admin-table-muted">no translation - shown in English</span>`,
		`<summary class="btn btn-sm btn-outline-secondary">Translate</summary>`,
		`<input type="hidden" name="context" value="menu"> <input type="hidden" name="source" value="Reply">`,
		`<td class="admin-table-muted small">Module: Forum</td>`,
		// The changes no longer in use.
		`<section id="outdated"`,
		`<td>@count а / @count б / @count в</td>`,
		`<input type="hidden" name="source" value="@count old"> <button type="submit" class="btn btn-sm btn-outline-secondary">Remove</button>`,
		`51-54 of 54`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if n := strings.Count(out, `<details class="admin-strings-edit" open>`); n != 1 {
		t.Errorf("%d strings open, want 1", n)
	}
}
