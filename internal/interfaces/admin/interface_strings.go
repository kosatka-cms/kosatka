package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// Interface translations (/admin/config/translations) - the interface's
// languages and its strings, a string's translation changed for the site
// (service.InterfaceStrings).

const (
	interfaceStringsURL     = "/admin/config/translations"
	interfaceStringsListURL = interfaceStringsURL + "/strings"
	interfaceStringsPerPage = 50
)

// InterfaceLanguages handles GET /admin/config/translations: the
// interface's languages - how much of the site's strings each translates,
// the downloaded ones and the translations catalog's others - and what
// came of a download, an update, a removal (the address's done, error,
// lang, kept, dropped).
func (c *Controller) InterfaceLanguages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, err := c.interfaceStrings.LanguagesPage(ctx, false)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error listing the interface languages")
		return
	}
	if page.CatalogErr != nil && !errors.Is(page.CatalogErr, service.ErrNoTranslationsCatalog) {
		c.LogFailure(page.CatalogErr, "error reading the translations catalog")
	}
	data := c.getAdminData(r)
	title := i18n.T(ctx, "Interface translations")
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["page"] = page
	data["noCatalog"] = errors.Is(page.CatalogErr, service.ErrNoTranslationsCatalog)
	data["siteLanguage"] = i18n.Default()
	q := r.URL.Query()
	name := i18n.NativeName(q.Get("lang"))
	kept, _ := strconv.Atoi(q.Get("kept"))
	dropped, _ := strconv.Atoi(q.Get("dropped"))
	switch q.Get("done") {
	case "downloaded":
		data["downloaded"] = true
		data["done"] = i18n.Plural(ctx, kept, "The @language translation is installed: @count string.", "The @language translation is installed: @count strings.", "@language", name)
	case "updated":
		data["done"] = i18n.Plural(ctx, kept, "The @language translation is updated: @count string.", "The @language translation is updated: @count strings.", "@language", name)
	case "deleted":
		data["done"] = i18n.T(ctx, "The @language translation is removed.", "@language", name)
	case "checked":
		data["done"] = i18n.T(ctx, "The catalog is checked.")
	}
	if dropped > 0 {
		data["dropped"] = i18n.Plural(ctx, dropped, "@count string of the file was left out: not a string of the site, or it fails the checks of a translation.", "@count strings of the file were left out: not strings of the site, or they fail the checks of a translation.")
	}
	data["error"] = c.interfacePackError(r, q.Get("error"), name)
	c.RenderOrFail(w, r, "admin/config/translations.html", data)
}

// interfacePackErrors - the error codes of the Languages tab's address
// by the service's errors.
var interfacePackErrors = map[error]string{
	service.ErrNoTranslationsCatalog: "no_catalog",
	service.ErrUnknownPack:           "unknown",
	service.ErrPackChecksum:          "checksum",
	service.ErrPackInUse:             "in_use",
	service.ErrPackNotInstalled:      "not_installed",
}

// interfacePackError - the message of an error code of the Languages
// tab's address ("" - none).
func (c *Controller) interfacePackError(r *http.Request, code, language string) string {
	ctx := r.Context()
	switch code {
	case "":
		return ""
	case "no_catalog":
		return i18n.T(ctx, "No translations catalog is configured.")
	case "unknown":
		return i18n.T(ctx, "The catalog has no @language translation to download.", "@language", language)
	case "checksum":
		return i18n.T(ctx, "The @language translation's file isn't what the catalog says (its checksum differs): nothing was installed.", "@language", language)
	case "not_installed":
		return i18n.T(ctx, "@language isn't a downloaded language.", "@language", language)
	case "in_use":
		return i18n.T(ctx, "@language is the site's interface language: choose another one in Site information first.", "@language", language)
	case "catalog":
		return i18n.T(ctx, "The translations catalog couldn't be read. Try again later.")
	case "delete":
		return i18n.T(ctx, "The @language translation couldn't be removed. Try again later.", "@language", language)
	}
	return i18n.T(ctx, "The @language translation couldn't be installed. Try again later.", "@language", language)
}

// interfacePackRedirect - back to the Languages tab with what came of an
// action on lang: done, or the error's code (logged unless it is the
// visitor's).
func (c *Controller) interfacePackRedirect(w http.ResponseWriter, r *http.Request, lang string, q url.Values, err error, fallback string) {
	if lang != "" {
		q.Set("lang", lang)
	}
	if err != nil {
		code := fallback
		for e, known := range interfacePackErrors {
			if errors.Is(err, e) {
				code = known
			}
		}
		if code == fallback {
			c.LogFailure(err, "error with an interface translation", "lang", lang)
		}
		q = url.Values{"lang": q["lang"], "error": {code}}
	}
	http.Redirect(w, r, interfaceStringsURL+"?"+q.Encode(), http.StatusSeeOther)
}

// InterfacePackDownload handles POST /admin/config/translations/download:
// the catalog's translation of a language installed (or updated).
func (c *Controller) InterfacePackDownload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, interfaceStringsURL, http.StatusSeeOther)
		return
	}
	lang := r.PostForm.Get("lang")
	res, err := c.interfaceStrings.Download(r.Context(), lang)
	q := url.Values{"done": {"downloaded"}, "kept": {strconv.Itoa(res.Kept)}}
	if res.Updated {
		q.Set("done", "updated")
	}
	if res.Dropped > 0 {
		q.Set("dropped", strconv.Itoa(res.Dropped))
	}
	c.interfacePackRedirect(w, r, lang, q, err, "download")
}

// InterfacePackDelete handles POST /admin/config/translations/delete: a
// downloaded translation removed.
func (c *Controller) InterfacePackDelete(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, interfaceStringsURL, http.StatusSeeOther)
		return
	}
	lang := r.PostForm.Get("lang")
	err := c.interfaceStrings.DeletePack(r.Context(), lang)
	c.interfacePackRedirect(w, r, lang, url.Values{"done": {"deleted"}}, err, "delete")
}

// InterfaceCatalogCheck handles POST /admin/config/translations/check: the
// translations catalog read again (its updates).
func (c *Controller) InterfaceCatalogCheck(w http.ResponseWriter, r *http.Request) {
	_, _, err := c.interfaceStrings.Catalog(r.Context(), true)
	c.interfacePackRedirect(w, r, "", url.Values{"done": {"checked"}}, err, "catalog")
}

// stringEdit - a string's translation sent and refused: shown open with
// what was sent and why.
type stringEdit struct {
	Context, Source string
	Text            string
	Forms           []string
	Error           string
}

// interfaceStringsFilter - the strings tab's filter from the request; a
// language the interface doesn't have - the site's.
func interfaceStringsFilter(r *http.Request) service.InterfaceStringsFilter {
	v := r.URL.Query()
	if r.Method == http.MethodPost {
		v = r.PostForm
	}
	f := service.InterfaceStringsFilter{Lang: v.Get("lang"), Query: strings.TrimSpace(v.Get("q")),
		Show: v.Get("show"), From: v.Get("from"), PerPage: interfaceStringsPerPage}
	if !i18n.Current().Available(f.Lang) {
		f.Lang = i18n.Default()
	}
	if f.Show != "untranslated" && f.Show != "edited" {
		f.Show = ""
	}
	f.Page, _ = strconv.Atoi(v.Get("page"))
	f.Page = max(f.Page, 1)
	return f
}

// filterQuery - the strings tab's address query of f (page left out
// unless above 1).
func filterQuery(f service.InterfaceStringsFilter) url.Values {
	q := url.Values{"lang": {f.Lang}}
	for k, v := range map[string]string{"q": f.Query, "show": f.Show, "from": f.From} {
		if v != "" {
			q.Set(k, v)
		}
	}
	if f.Page > 1 {
		q.Set("page", strconv.Itoa(f.Page))
	}
	return q
}

// InterfaceStringsPage handles GET /admin/config/translations/strings.
func (c *Controller) InterfaceStringsPage(w http.ResponseWriter, r *http.Request) {
	c.renderInterfaceStrings(w, r, interfaceStringsFilter(r), nil)
}

// renderInterfaceStrings renders the strings tab of f; failed - a string
// whose translation was refused, shown open.
func (c *Controller) renderInterfaceStrings(w http.ResponseWriter, r *http.Request, f service.InterfaceStringsFilter, failed *stringEdit) {
	ctx := r.Context()
	page, err := c.interfaceStrings.List(ctx, f)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error listing the interface strings")
		return
	}
	data := c.getAdminData(r)
	title := i18n.T(ctx, "Interface strings")
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["page"] = page
	data["rows"] = stringRows(page.Rows, f.Lang, failed)
	data["filter"] = f
	data["filterQuery"] = filterQuery(f).Encode()
	data["filtered"] = f.Query != "" || f.Show != "" || f.From != ""
	data["languages"] = i18n.Current().Options()
	data["sourceLanguage"] = f.Lang == i18n.SourceLanguage
	origins := c.interfaceStringOrigins(r, page.Origins)
	labels := make(map[string]string, len(origins))
	for _, o := range origins {
		labels[o.Value] = o.Label
	}
	data["origins"] = origins
	data["originLabels"] = labels
	data["failed"] = failed
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["reset"] = r.URL.Query().Get("reset") == "1"
	data["error"] = r.URL.Query().Get("error")
	q := filterQuery(f)
	q.Del("page")
	data["pagination"] = pagination.New(f.Page, page.Total, interfaceStringsPerPage, interfaceStringsListURL, q)
	if failed != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	c.RenderOrFail(w, r, "admin/config/translations_strings.html", data)
}

// stringRow - a string on the strings tab: the translation shown (Site,
// Default; a plural one's forms joined), the edit form's fields (Text, or
// Forms - one per plural form of the language, labelled with numbers it
// is for), open with Error when its translation was refused.
type stringRow struct {
	service.InterfaceStringRow
	SiteShown, DefaultShown string
	Text                    string
	Forms                   []stringForm
	Open                    bool
	Error                   string
}

// stringForm - a plural form's field: which numbers it is for, its value.
type stringForm struct {
	Label, Value string
}

// stringRows - rows of the strings tab in lang; failed - the one refused.
func stringRows(rows []service.InterfaceStringRow, lang string, failed *stringEdit) []stringRow {
	examples := service.PluralExamples(lang)
	labels := make([]string, len(examples))
	for i, ns := range examples {
		parts := make([]string, len(ns))
		for j, n := range ns {
			parts[j] = strconv.Itoa(n)
		}
		labels[i] = strings.Join(parts, ", ")
		if len(ns) == 3 {
			labels[i] += "..."
		}
	}
	out := make([]stringRow, len(rows))
	for i, r := range rows {
		v := stringRow{InterfaceStringRow: r, SiteShown: r.Site, DefaultShown: r.Default}
		text, forms := r.Default, r.DefaultForms
		if r.Edited {
			text, forms = r.Site, r.SiteForms
		}
		if failed != nil && failed.Context == r.Context && failed.Source == r.Source.Source {
			text, forms = failed.Text, failed.Forms
			v.Open, v.Error = true, failed.Error
		}
		if r.IsPlural() {
			v.SiteShown = strings.Join(r.SiteForms, " / ")
			v.DefaultShown = strings.Join(r.DefaultForms, " / ")
			for j, label := range labels {
				f := stringForm{Label: label}
				if j < len(forms) {
					f.Value = forms[j]
				}
				v.Forms = append(v.Forms, f)
			}
		} else {
			v.Text = text
		}
		out[i] = v
	}
	return out
}

// stringOrigin - a filter choice of where strings come from.
type stringOrigin struct {
	Value, Label string
}

// interfaceStringOrigins - the "From" choices: the engine, the modules
// (by their labels), the themes.
func (c *Controller) interfaceStringOrigins(r *http.Request, origins []string) []stringOrigin {
	ctx := r.Context()
	var out []stringOrigin
	for _, o := range origins {
		kind, name, _ := strings.Cut(o, ":")
		label := i18n.T(ctx, "Engine")
		switch kind {
		case "module":
			label = name
			if m, ok := entity.ModuleByName(name); ok {
				label = i18n.T(ctx, m.Label)
			}
			label = i18n.T(ctx, "Module: @name", "@name", label)
		case "theme":
			label = i18n.T(ctx, "Theme: @name", "@name", name)
		}
		out = append(out, stringOrigin{Value: o, Label: label})
	}
	return out
}

// InterfaceStringSave handles POST /admin/config/translations/strings: a
// string's translation for the site; refused - the tab again, the string
// open with why.
func (c *Controller) InterfaceStringSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, interfaceStringsListURL, http.StatusSeeOther)
		return
	}
	f := interfaceStringsFilter(r)
	msgctx, source := r.PostForm.Get("context"), r.PostForm.Get("source")
	text := r.PostForm.Get("text")
	var forms []string
	for i := 0; ; i++ {
		v, ok := r.PostForm["form_"+strconv.Itoa(i)]
		if !ok {
			break
		}
		forms = append(forms, strings.Join(v, ""))
	}
	err := c.interfaceStrings.Save(ctx, f.Lang, msgctx, source, text, forms)
	if err == nil {
		q := filterQuery(f)
		q.Set("saved", "1")
		http.Redirect(w, r, interfaceStringsListURL+"?"+q.Encode(), http.StatusSeeOther)
		return
	}
	failed := &stringEdit{Context: msgctx, Source: source, Text: text, Forms: forms, Error: c.interfaceStringError(r, err)}
	if failed.Error == "" {
		c.LogFailure(err, "error saving an interface string", "source", source)
		failed.Error = i18n.T(ctx, "Error while saving.")
	}
	c.renderInterfaceStrings(w, r, f, failed)
}

// interfaceStringError - why a translation was refused, for the page; ""
// - not the translation's fault.
func (c *Controller) interfaceStringError(r *http.Request, err error) string {
	ctx := r.Context()
	var te *i18n.TranslationError
	switch {
	case errors.As(err, &te) && te.Placeholder != "":
		return i18n.T(ctx, "The translation must keep @placeholder: the value goes in its place.", "@placeholder", te.Placeholder)
	case errors.As(err, &te) && te.Tag == "<":
		return i18n.T(ctx, "The translation has a tag not closed: \"<\" before a letter.")
	case errors.As(err, &te):
		return i18n.T(ctx, "The translation can have only the tags of the English string, as they are: @tag isn't one.", "@tag", te.Tag)
	case errors.Is(err, service.ErrEmptyTranslation):
		return i18n.T(ctx, "Enter the translation (every form of it).")
	case errors.Is(err, service.ErrUnknownString):
		return i18n.T(ctx, "The site has no such string.")
	}
	return ""
}

// InterfaceStringReset handles POST /admin/config/translations/strings/reset:
// a string's translation for the site removed (an outdated one too).
func (c *Controller) InterfaceStringReset(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, interfaceStringsListURL, http.StatusSeeOther)
		return
	}
	f := interfaceStringsFilter(r)
	q := filterQuery(f)
	if err := c.interfaceStrings.Reset(r.Context(), f.Lang, r.PostForm.Get("context"), r.PostForm.Get("source")); err != nil {
		c.LogFailure(err, "error resetting an interface string")
		q.Set("error", "1")
	} else {
		q.Set("reset", "1")
	}
	http.Redirect(w, r, interfaceStringsListURL+"?"+q.Encode(), http.StatusSeeOther)
}
