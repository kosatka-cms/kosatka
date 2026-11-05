package languages

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/modules/languages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// prefixPattern - an address prefix: lower-case Latin letters, digits and
// hyphens, up to 12 characters.
var prefixPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,11}$`)

// siteInformation - what the Languages page reads and saves of the site
// information.
type siteInformation interface {
	Get(ctx context.Context) (*modulekit.SiteInformation, error)
	Update(ctx context.Context, s *modulekit.SiteInformation) error
	SetLanguages(ctx context.Context, languages []modulekit.SiteLanguage) error
}

// Controller - the Languages page: the site's languages, adding one,
// removing one, making one the default.
type Controller struct {
	*modulekit.BaseController
	db      *sql.DB
	repo    *Repo
	site    siteInformation
	changed func()
	// translations - which content types are translated; types - the
	// site's content types to choose from.
	translations *Translations
	types        modulekit.ContentTypeService
}

// NewController creates a Controller; changed is called after the site's
// languages change (the cached blocks hold the old addresses).
func NewController(base *modulekit.BaseController, db *sql.DB, repo *Repo, site siteInformation, changed func(), translations *Translations, types modulekit.ContentTypeService) *Controller {
	base.SetLogger(base.Logger().With("component", "languages.handler"))
	return &Controller{BaseController: base, db: db, repo: repo, site: site, changed: changed, translations: translations, types: types}
}

// typeChoice - a content type on the Languages page: translated or not.
type typeChoice struct {
	Name, Label string
	On          bool
}

// row - a language as the page shows it.
type row struct {
	Code   string
	Name   string
	Prefix string
	// Default - the site's default language (its addresses without a
	// prefix).
	Default bool
	// Nodes - its content (the trash's too).
	Nodes int
	// Interface - the interface has a catalog in it (else it is shown in
	// the source language); only such a language can be the default.
	Interface bool
}

// List renders GET /admin/config/languages: the default language first,
// then the others in the order added; the form adding one of the known
// languages the site hasn't.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	info, err := c.site.Get(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the site's languages")
		return
	}
	counts, err := c.repo.NodeCounts(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "counting the content by language")
		return
	}
	def := defaultLanguage(info)
	tr := i18n.Current()
	rows := []row{{Code: def, Name: i18n.NativeName(def), Default: true, Nodes: counts[def], Interface: tr.Available(def)}}
	has := map[string]bool{def: true}
	for _, l := range info.Languages {
		rows = append(rows, row{Code: l.Code, Name: i18n.NativeName(l.Code), Prefix: l.Prefix, Nodes: counts[l.Code], Interface: tr.Available(l.Code)})
		has[l.Code] = true
	}
	var choices []i18n.LanguageOption
	for _, o := range i18n.KnownLanguages() {
		if !has[o.Code] {
			choices = append(choices, o)
		}
	}
	types, err := c.typeChoices(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the content types")
		return
	}
	q := r.URL.Query()
	data := c.AdminPageData(r)
	data["types"] = types
	st, err := c.translations.Settings(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the languages settings")
		return
	}
	data["untranslated"] = st.Untranslated
	data["title"] = i18n.T(ctx, "Languages")
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["rows"] = rows
	data["choices"] = choices
	data["sourceName"] = i18n.NativeName(i18n.SourceLanguage)
	data["error"] = q.Get("error")
	data["owner"] = q.Get("owner")
	data["added"] = q.Get("done") == "added"
	data["removed"] = q.Get("done") == "removed"
	data["defaultSet"] = q.Get("done") == "default"
	data["saved"] = q.Get("done") == "saved"
	c.RenderOrFail(w, r, "admin/languages.html", data)
}

// typeChoices - the site's content types (field sets left out), by label,
// the translated ones on.
func (c *Controller) typeChoices(ctx context.Context) ([]typeChoice, error) {
	defs, err := c.types.ContentTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the content types: %w", err)
	}
	st, err := c.translations.Settings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]typeChoice, 0, len(defs))
	for _, d := range defs {
		out = append(out, typeChoice{Name: d.MachineName, Label: d.Label, On: slices.Contains(st.TranslatableTypes, d.MachineName)})
	}
	slices.SortFunc(out, func(a, b typeChoice) int { return strings.Compare(a.Label, b.Label) })
	return out, nil
}

// Settings handles POST /admin/config/languages/settings: the content
// types whose items are translated (of the site's) and what a list in a
// language shows of the items without a version in it.
func (c *Controller) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	types, err := c.typeChoices(ctx)
	if err != nil {
		c.LogFailure(err, "loading the content types")
		c.back(w, r, "error", "save")
		return
	}
	_ = r.ParseForm()
	picked := r.Form["translatable"]
	var on []string
	for _, t := range types {
		if slices.Contains(picked, t.Name) {
			on = append(on, t.Name)
		}
	}
	st := Settings{TranslatableTypes: on, Untranslated: r.FormValue("untranslated")}
	if !slices.Contains(untranslatedChoices, st.Untranslated) {
		st.Untranslated = UntranslatedOriginal
	}
	if err := c.translations.SaveSettings(ctx, st); err != nil {
		c.LogFailure(err, "saving the languages settings")
		c.back(w, r, "error", "save")
		return
	}
	if c.changed != nil {
		c.changed()
	}
	c.back(w, r, "done", "saved")
}

// Add handles POST /admin/config/languages: a known language the site
// hasn't, at a prefix (its code when none is given) no other language,
// address or content takes.
func (c *Controller) Add(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.FormValue("code")
	prefix := r.FormValue("prefix")
	if prefix == "" {
		prefix = code
	}
	info, err := c.site.Get(ctx)
	if err != nil {
		c.LogFailure(err, "loading the site's languages")
		c.back(w, r, "error", "save")
		return
	}
	if !known(code) || code == defaultLanguage(info) || slices.ContainsFunc(info.Languages, func(l modulekit.SiteLanguage) bool { return l.Code == code }) {
		c.back(w, r, "error", "code")
		return
	}
	if problem, owner, err := c.prefixProblem(ctx, info, prefix); err != nil {
		c.LogFailure(err, "checking a language prefix", "prefix", prefix)
		c.back(w, r, "error", "save")
		return
	} else if problem != "" {
		c.back(w, r, "error", problem, "owner", owner)
		return
	}
	languages := append(slices.Clone(info.Languages), modulekit.SiteLanguage{Code: code, Prefix: prefix})
	if err := c.site.SetLanguages(ctx, languages); err != nil {
		c.LogFailure(err, "saving the site's languages")
		c.back(w, r, "error", "save")
		return
	}
	c.apply(defaultLanguage(info), languages)
	c.back(w, r, "done", "added")
}

// prefixProblem - why prefix can't be a language's ("" - it can): its
// format, another language's, an address of the site (owner - what
// answers it), a node's address under it.
func (c *Controller) prefixProblem(ctx context.Context, info *modulekit.SiteInformation, prefix string) (problem, owner string, err error) {
	if !prefixPattern.MatchString(prefix) {
		return "prefix", "", nil
	}
	if slices.ContainsFunc(info.Languages, func(l modulekit.SiteLanguage) bool { return l.Prefix == prefix }) {
		return "prefix-language", "", nil
	}
	owner, err = modulekit.PathOwner(ctx, prefix, manifest.Name)
	if err != nil {
		return "", "", err
	}
	if owner != "" {
		return "prefix-taken", owner, nil
	}
	under, err := c.repo.SlugsUnder(ctx, prefix)
	if err != nil {
		return "", "", err
	}
	if under {
		return "prefix-content", "", nil
	}
	return "", "", nil
}

// Remove handles POST /admin/config/languages/{code}/remove: the language
// leaves the site; its content stays, at addresses without the prefix.
func (c *Controller) Remove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.PathValue("code")
	info, err := c.site.Get(ctx)
	if err != nil {
		c.LogFailure(err, "loading the site's languages")
		c.back(w, r, "error", "save")
		return
	}
	languages := slices.DeleteFunc(slices.Clone(info.Languages), func(l modulekit.SiteLanguage) bool { return l.Code == code })
	if len(languages) == len(info.Languages) {
		c.back(w, r, "error", "code")
		return
	}
	if err := c.site.SetLanguages(ctx, languages); err != nil {
		c.LogFailure(err, "saving the site's languages")
		c.back(w, r, "error", "save")
		return
	}
	c.apply(defaultLanguage(info), languages)
	c.back(w, r, "done", "removed")
}

// MakeDefault handles POST /admin/config/languages/{code}/default: one of
// the others, with the interface in it, becomes the default language -
// its addresses lose the prefix, the old default's get its code
// (modulekit.MoveDefaultLanguage); the site's interface follows.
func (c *Controller) MakeDefault(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.PathValue("code")
	info, err := c.site.Get(ctx)
	if err != nil {
		c.LogFailure(err, "loading the site's languages")
		c.back(w, r, "error", "save")
		return
	}
	if !slices.ContainsFunc(info.Languages, func(l modulekit.SiteLanguage) bool { return l.Code == code }) || !i18n.Current().Available(code) {
		c.back(w, r, "error", "code")
		return
	}
	old := defaultLanguage(info)
	languages := modulekit.MoveDefaultLanguage(info.Languages, old, code)
	err = modulekit.InTx(ctx, c.db, func(ctx context.Context) error {
		info.DefaultLanguage = code
		if err := c.site.Update(ctx, info); err != nil {
			return fmt.Errorf("saving the default language: %w", err)
		}
		return c.site.SetLanguages(ctx, languages)
	})
	if err != nil {
		c.LogFailure(err, "making a language the default", "code", code)
		c.back(w, r, "error", "save")
		return
	}
	c.apply(code, languages)
	c.back(w, r, "done", "default")
}

// apply puts the saved languages in effect.
func (c *Controller) apply(def string, others []modulekit.SiteLanguage) {
	modulekit.ApplySiteLanguages(def, others)
	if c.changed != nil {
		c.changed()
	}
}

// back redirects to the Languages page with the query's pairs (key,
// value…).
func (c *Controller) back(w http.ResponseWriter, r *http.Request, kv ...string) {
	q := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			q.Set(kv[i], kv[i+1])
		}
	}
	http.Redirect(w, r, manifest.AdminPath+"?"+q.Encode(), http.StatusSeeOther)
}

// defaultLanguage - the site's default language (the source one when
// none is set).
func defaultLanguage(info *modulekit.SiteInformation) string {
	if info.DefaultLanguage == "" {
		return i18n.SourceLanguage
	}
	return info.DefaultLanguage
}

// known - code is one of the languages the engine has names for.
func known(code string) bool {
	return slices.ContainsFunc(i18n.KnownLanguages(), func(o i18n.LanguageOption) bool { return o.Code == code })
}
