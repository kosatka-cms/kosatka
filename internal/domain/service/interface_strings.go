package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// ErrUnknownString - a string the site's code doesn't have.
var ErrUnknownString = errors.New("no such interface string")

// ErrEmptyTranslation - a translation (or one of its plural forms) left
// empty.
var ErrEmptyTranslation = errors.New("the translation is empty")

// InterfaceStrings - the interface strings of the site and their
// translations (the admin's Interface translations page): the code's
// strings (the sources list, translations/sources) with the catalogs'
// translation in a language and the site's own over it (InterfaceString,
// pkg/i18n WithSite), changed and reset there. A change builds the site's
// translator again (SetReload).
type InterfaceStrings struct {
	repo    entity.InterfaceStringRepository
	sources []i18n.Source
	// themeSources - the active theme's strings from its catalogs, for a
	// theme the sources list hasn't (one from its own repository).
	themeSources func() []i18n.Source
	reload       func(ctx context.Context) error
	now          func() time.Time

	// packs, catalog, builtIn - the downloaded translations (SetPacks):
	// where they are kept, the catalog, the languages built in.
	packs   entity.InterfacePackRepository
	catalog *translationsCatalog
	builtIn []string
	// mu guards the catalog's cached index.
	mu sync.Mutex
}

// NewInterfaceStrings - the service on repo and the code's strings.
func NewInterfaceStrings(repo entity.InterfaceStringRepository, sources []i18n.Source) *InterfaceStrings {
	return &InterfaceStrings{repo: repo, sources: sources, now: time.Now}
}

// SetReload sets what builds the site's translator again after a change.
func (s *InterfaceStrings) SetReload(reload func(ctx context.Context) error) { s.reload = reload }

// SetThemeSources sets where the active theme's catalog strings come
// from.
func (s *InterfaceStrings) SetThemeSources(f func() []i18n.Source) { s.themeSources = f }

// Entries - the site's own translations, for the translator (WithSite).
func (s *InterfaceStrings) Entries(ctx context.Context) ([]i18n.Entry, error) {
	rows, err := s.repo.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]i18n.Entry, len(rows))
	for i, r := range rows {
		out[i] = i18n.Entry{Lang: r.Lang, Context: r.Context, Source: r.Source, Text: r.Text, Forms: r.Forms}
	}
	return out, nil
}

// visible - the strings the site shows: the core's, the enabled modules',
// the active theme's chain's (its catalogs' too), each once; not the
// starter content's texts.
func (s *InterfaceStrings) visible() []i18n.Source {
	chain := paths.ThemeChain(paths.Theme())
	seen := map[string]bool{}
	var out []i18n.Source
	add := func(src i18n.Source) {
		k := src.Context + "\x04" + src.Source
		if !seen[k] {
			seen[k] = true
			out = append(out, src)
		}
	}
	for _, src := range s.sources {
		switch {
		case src.Starter:
			// The starter content's text: data written at install, not
			// a string the interface shows.
			continue
		case src.Theme != "":
			if !slices.Contains(chain, src.Theme) {
				continue
			}
		case src.Module != "":
			if !ModuleEnabled(src.Module) {
				continue
			}
		}
		add(src)
	}
	if s.themeSources != nil {
		for _, src := range s.themeSources() {
			add(src)
		}
	}
	return out
}

// find - the visible string of msgctx and source.
func (s *InterfaceStrings) find(msgctx, source string) (i18n.Source, bool) {
	for _, src := range s.visible() {
		if src.Context == msgctx && src.Source == source {
			return src, true
		}
	}
	return i18n.Source{}, false
}

// InterfaceStringRow - a string on the page in a language: the catalogs'
// translation (Default, DefaultForms; the source itself in English), the
// site's own (Site, SiteForms; Edited), Untranslated - neither.
type InterfaceStringRow struct {
	i18n.Source
	Default      string
	DefaultForms []string
	Site         string
	SiteForms    []string
	Edited       bool
	Untranslated bool
}

// IsPlural - whether the string has plural forms.
func (r InterfaceStringRow) IsPlural() bool {
	return r.Plural != "" || len(r.DefaultForms) > 0 || len(r.SiteForms) > 0
}

// Origin - where the string comes from: "core", "module:<name>",
// "theme:<name>".
func (r InterfaceStringRow) Origin() string { return origin(r.Source) }

// row - src in lang.
func row(tr *i18n.Translator, lang string, src i18n.Source) InterfaceStringRow {
	r := InterfaceStringRow{Source: src}
	var ok bool
	if lang == i18n.SourceLanguage {
		r.Default, ok = src.Source, true
		if src.Plural != "" {
			r.DefaultForms = []string{src.Source, src.Plural}
		}
	} else {
		r.Default, r.DefaultForms, ok = tr.Default(lang, src.Context, src.Source)
	}
	r.Site, r.SiteForms, r.Edited = tr.Site(lang, src.Context, src.Source)
	r.Untranslated = !ok && !r.Edited
	return r
}

// InterfaceStringsFilter - what the page lists: the language, a text in
// the string or its translation, Show ("" - all, "untranslated",
// "edited"), From ("" - all, "core", "module:<name>", "theme:<name>"), a
// page of PerPage.
type InterfaceStringsFilter struct {
	Lang, Query, Show, From string
	Page, PerPage           int
}

// InterfaceStringsPage - a page of the strings (Rows of Total), the
// language's counts, the site's strings of the language the code no
// longer has (Outdated: its English changed) and where the strings come
// from (Origins: "core", "module:<name>", "theme:<name>").
type InterfaceStringsPage struct {
	Rows                 []InterfaceStringRow
	Total                int
	Untranslated, Edited int
	Outdated             []entity.InterfaceString
	Origins              []string
}

// origin - where src comes from, as the filter names it.
func origin(src i18n.Source) string {
	switch {
	case src.Theme != "":
		return "theme:" + src.Theme
	case src.Module != "":
		return "module:" + src.Module
	}
	return "core"
}

// List - the page of the strings f asks for.
func (s *InterfaceStrings) List(ctx context.Context, f InterfaceStringsFilter) (InterfaceStringsPage, error) {
	var page InterfaceStringsPage
	tr := i18n.Current()
	q := strings.ToLower(strings.TrimSpace(f.Query))
	known := map[string]bool{}
	var matched []InterfaceStringRow
	for _, src := range s.visible() {
		known[src.Context+"\x04"+src.Source] = true
		if o := origin(src); !slices.Contains(page.Origins, o) {
			page.Origins = append(page.Origins, o)
		}
		r := row(tr, f.Lang, src)
		if r.Untranslated {
			page.Untranslated++
		}
		if r.Edited {
			page.Edited++
		}
		switch {
		case f.Show == "untranslated" && !r.Untranslated,
			f.Show == "edited" && !r.Edited,
			f.From != "" && origin(src) != f.From,
			q != "" && !r.matches(q):
			continue
		}
		matched = append(matched, r)
	}
	rows, err := s.repo.All(ctx)
	if err != nil {
		return page, err
	}
	for _, r := range rows {
		if r.Lang == f.Lang && !known[r.Context+"\x04"+r.Source] {
			page.Outdated = append(page.Outdated, r)
		}
	}
	page.Total = len(matched)
	if f.PerPage > 0 {
		from := min(max(f.Page-1, 0)*f.PerPage, len(matched))
		matched = matched[from:min(from+f.PerPage, len(matched))]
	}
	page.Rows = matched
	return page, nil
}

// matches - whether the string or a translation of it has q (lower case).
func (r InterfaceStringRow) matches(q string) bool {
	texts := append([]string{r.Source.Source, r.Plural, r.Default, r.Site}, r.DefaultForms...)
	for _, t := range append(texts, r.SiteForms...) {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

// InterfaceLanguage - a language of the interface: how many of the site's
// strings it translates (the catalogs or the site), how many the site
// changed; on the Languages tab (LanguagesPage) where it stands, its
// downloaded pack and its entry in the translations catalog (nil - none).
type InterfaceLanguage struct {
	Code, Name                string
	Total, Translated, Edited int
	Untranslated              int
	Source                    bool
	State                     PackState
	Pack                      *entity.InterfacePack
	Catalog                   *CatalogLanguage
}

// Percent - the share translated, whole percent (a fraction rounded down,
// so 100 means all).
func (l InterfaceLanguage) Percent() int {
	if l.Total == 0 {
		return 100
	}
	return l.Translated * 100 / l.Total
}

// Languages - the interface's languages (the source language and those
// with a catalog) with their counts.
func (s *InterfaceStrings) Languages() []InterfaceLanguage {
	tr := i18n.Current()
	visible := s.visible()
	var out []InterfaceLanguage
	for _, o := range tr.Options() {
		l := InterfaceLanguage{Code: o.Code, Name: i18n.NativeName(o.Code), Total: len(visible), Source: o.Code == i18n.SourceLanguage}
		for _, src := range visible {
			r := row(tr, o.Code, src)
			if !r.Untranslated {
				l.Translated++
			} else {
				l.Untranslated++
			}
			if r.Edited {
				l.Edited++
			}
		}
		out = append(out, l)
	}
	return out
}

// Save sets the site's translation of a string in lang: text, or the
// plural forms (one per plural category of lang). Each must keep the
// source's placeholders and no HTML but its tags (i18n.CheckTranslation).
func (s *InterfaceStrings) Save(ctx context.Context, lang, msgctx, source, text string, forms []string) error {
	src, ok := s.find(msgctx, source)
	if !ok || !i18n.Current().Available(lang) {
		return ErrUnknownString
	}
	r := row(i18n.Current(), lang, src)
	entry := entity.InterfaceString{Lang: lang, Context: msgctx, Source: source, UpdatedAt: s.now()}
	if r.IsPlural() {
		n := len(i18n.PluralCategories(lang))
		if len(forms) != n {
			return ErrEmptyTranslation
		}
		for i, form := range forms {
			form = trimTranslation(form)
			if form == "" {
				return ErrEmptyTranslation
			}
			if err := i18n.CheckTranslation(source, form); err != nil {
				return err
			}
			forms[i] = form
		}
		entry.Forms = forms
	} else {
		entry.Text = trimTranslation(text)
		if entry.Text == "" {
			return ErrEmptyTranslation
		}
		if err := i18n.CheckTranslation(source, entry.Text); err != nil {
			return err
		}
	}
	if err := s.repo.Save(ctx, entry); err != nil {
		return err
	}
	return s.reloaded(ctx)
}

// trimTranslation - a translation as typed without the spaces and line
// breaks around it; a no-break space stays (a translation may be one: a
// thousands separator).
func trimTranslation(s string) string { return strings.Trim(s, " \t\r\n") }

// Reset removes the site's translation of a string in lang (an outdated
// one too): the catalogs' is back.
func (s *InterfaceStrings) Reset(ctx context.Context, lang, msgctx, source string) error {
	if err := s.repo.Delete(ctx, lang, msgctx, source); err != nil {
		return err
	}
	return s.reloaded(ctx)
}

// reloaded builds the site's translator again.
func (s *InterfaceStrings) reloaded(ctx context.Context) error {
	if s.reload == nil {
		return nil
	}
	if err := s.reload(ctx); err != nil {
		return fmt.Errorf("applying the interface strings: %w", err)
	}
	return nil
}

// PluralExamples - for each plural form of lang (PluralCategories' order),
// a few numbers it is for: the labels of a plural string's fields.
func PluralExamples(lang string) [][]int {
	out := make([][]int, len(i18n.PluralCategories(lang)))
	for n := 0; n <= 200; n++ {
		i := i18n.PluralIndex(lang, n)
		if i < len(out) && len(out[i]) < 3 {
			out[i] = append(out[i], n)
		}
	}
	return out
}
