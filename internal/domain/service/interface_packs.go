package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Downloaded interface translations (packs): the translations catalog
// (translations_catalog_url) is an index (index.json) of the languages one
// can download, each one's catalog file next to the index. A language the
// engine has built in (English and those with a catalog in the binary)
// isn't downloaded. A pack's file is checked against the index's
// checksum, and each of its strings like a site's own change
// (i18n.CheckTranslation): a string of the code, its placeholders kept, no
// HTML but the English string's tags, a form per plural category; the
// others are left out. Its strings come after the theme's catalog
// (i18n.Translator.WithPacks); the site's own changes stay over them.

// Errors of the packs a page shows.
var (
	// ErrNoTranslationsCatalog - no translations catalog is configured.
	ErrNoTranslationsCatalog = errors.New("no translations catalog configured")
	// ErrUnknownPack - the catalog has no such language (or it is built
	// in).
	ErrUnknownPack = errors.New("the translations catalog has no such language")
	// ErrPackChecksum - the file isn't what the catalog says.
	ErrPackChecksum = errors.New("the translation's file doesn't match the catalog's checksum")
	// ErrPackInUse - the language is the site's: it can't be deleted.
	ErrPackInUse = errors.New("the language is the site's interface language")
	// ErrPackNotInstalled - the language isn't a downloaded one.
	ErrPackNotInstalled = errors.New("the language isn't a downloaded one")
)

// Catalog limits and how long the index is kept.
const (
	maxTranslationsIndexBytes = 1 << 20
	maxPackBytes              = 16 << 20
	translationsCatalogTTL    = time.Hour
	translationsHTTPTimeout   = 30 * time.Second
)

var (
	packLangRe    = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$`)
	packVersionRe = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,31}$`)
	packFileRe    = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*\.json$`)
	packSHA256Re  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// TranslationsCatalog - the catalog's index: when it was built and its
// languages.
type TranslationsCatalog struct {
	Generated time.Time         `json:"generated"`
	Languages []CatalogLanguage `json:"languages"`
}

// CatalogLanguage - a language of the catalog: its code, its pack's
// version, its file (a name, next to the index) and the file's SHA-256
// (hex), how many strings it translates, whether it is a draft (machine
// translated, not reviewed).
type CatalogLanguage struct {
	Code    string `json:"code"`
	Version string `json:"version"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
	Strings int    `json:"strings"`
	Draft   bool   `json:"draft,omitempty"`
}

// clean - the index's languages one can download: a valid code, version,
// file name and checksum, each code once. Others are left out (a broken
// entry doesn't break the page).
func (c TranslationsCatalog) clean() TranslationsCatalog {
	out := TranslationsCatalog{Generated: c.Generated}
	seen := map[string]bool{}
	for _, l := range c.Languages {
		l.SHA256 = strings.ToLower(l.SHA256)
		if len(l.Code) > 12 || !packLangRe.MatchString(l.Code) || l.Code == i18n.SourceLanguage || seen[l.Code] ||
			!packVersionRe.MatchString(l.Version) || !packFileRe.MatchString(l.File) || !packSHA256Re.MatchString(l.SHA256) {
			continue
		}
		seen[l.Code] = true
		l.Strings = max(l.Strings, 0)
		out.Languages = append(out.Languages, l)
	}
	return out
}

// Language - the catalog's language code.
func (c TranslationsCatalog) Language(code string) (CatalogLanguage, bool) {
	for _, l := range c.Languages {
		if l.Code == code {
			return l, true
		}
	}
	return CatalogLanguage{}, false
}

// translationsCatalog - the catalog at url ("" - none) and its index,
// kept an hour.
type translationsCatalog struct {
	url     string
	client  *http.Client
	cached  TranslationsCatalog
	fetched time.Time
}

// SetPacks sets the downloaded translations up: where they are kept, the
// catalog's address ("" - none; https, http or a local file), the
// languages built into the binary (not downloaded).
func (s *InterfaceStrings) SetPacks(repo entity.InterfacePackRepository, catalogURL string, builtIn []string) {
	s.packs = repo
	s.catalog = &translationsCatalog{url: catalogURL, client: &http.Client{Timeout: translationsHTTPTimeout}}
	s.builtIn = builtIn
}

// PackEntries - the downloaded translations' strings, for the translator
// (WithPacks).
func (s *InterfaceStrings) PackEntries(ctx context.Context) ([]i18n.Entry, error) {
	if s.packs == nil {
		return nil, nil
	}
	rows, err := s.packs.Strings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]i18n.Entry, len(rows))
	for i, r := range rows {
		out[i] = i18n.Entry{Lang: r.Lang, Context: r.Context, Source: r.Source, Text: r.Text, Forms: r.Forms}
	}
	return out, nil
}

// CatalogURL - the translations catalog's address ("" - none).
func (s *InterfaceStrings) CatalogURL() string {
	if s.catalog == nil {
		return ""
	}
	return s.catalog.url
}

// Catalog - the catalog's index and when it was read: kept an hour,
// read again with refresh.
func (s *InterfaceStrings) Catalog(ctx context.Context, refresh bool) (TranslationsCatalog, time.Time, error) {
	if s.catalog == nil || s.catalog.url == "" {
		return TranslationsCatalog{}, time.Time{}, ErrNoTranslationsCatalog
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.catalog
	if !refresh && !c.fetched.IsZero() && s.now().Sub(c.fetched) < translationsCatalogTTL {
		return c.cached, c.fetched, nil
	}
	b, err := c.read(ctx, "", maxTranslationsIndexBytes, true)
	if err != nil {
		return TranslationsCatalog{}, time.Time{}, fmt.Errorf("reading the translations catalog: %w", err)
	}
	var index TranslationsCatalog
	if err := json.Unmarshal(b, &index); err != nil {
		return TranslationsCatalog{}, time.Time{}, fmt.Errorf("parsing the translations catalog: %w", err)
	}
	c.cached, c.fetched = index.clean(), s.now()
	return c.cached, c.fetched, nil
}

// read - the index (index true) or a file next to it, up to limit bytes.
func (c *translationsCatalog) read(ctx context.Context, file string, limit int64, index bool) ([]byte, error) {
	var body io.ReadCloser
	if strings.HasPrefix(c.url, "https://") || strings.HasPrefix(c.url, "http://") {
		address := c.url
		if !index {
			base, err := url.Parse(c.url)
			if err != nil {
				return nil, err
			}
			address = base.ResolveReference(&url.URL{Path: file}).String()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("%s: %s", address, resp.Status)
		}
		body = resp.Body
	} else {
		name := c.url
		if !index {
			name = filepath.Join(filepath.Dir(c.url), file)
		}
		f, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		body = f
	}
	defer func() { _ = body.Close() }()
	b, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("over %d bytes", limit)
	}
	return b, nil
}

// PackResult - what a download installed: the language, its strings kept
// and left out (not the code's, or failing the checks), whether it
// replaced an installed pack (an update).
type PackResult struct {
	Lang          string
	Kept, Dropped int
	Updated       bool
}

// Download installs (or updates) the catalog's translation of lang: its
// file checked against the catalog's checksum, its strings checked, in
// place of the language's previous pack; the site's translator rebuilt.
func (s *InterfaceStrings) Download(ctx context.Context, lang string) (PackResult, error) {
	if s.packs == nil || slices.Contains(s.builtIn, lang) {
		return PackResult{}, ErrUnknownPack
	}
	index, _, err := s.Catalog(ctx, false)
	if err != nil {
		return PackResult{}, err
	}
	l, ok := index.Language(lang)
	if !ok {
		if index, _, err = s.Catalog(ctx, true); err != nil {
			return PackResult{}, err
		}
		if l, ok = index.Language(lang); !ok {
			return PackResult{}, ErrUnknownPack
		}
	}
	b, err := s.catalog.read(ctx, l.File, maxPackBytes, false)
	if err != nil {
		return PackResult{}, fmt.Errorf("downloading the %s translation: %w", lang, err)
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != l.SHA256 {
		return PackResult{}, ErrPackChecksum
	}
	entries, err := i18n.ParseCatalog(lang, b)
	if err != nil {
		return PackResult{}, err
	}
	known := s.knownKeys()
	forms := len(i18n.PluralCategories(lang))
	res := PackResult{Lang: lang}
	var keep []entity.InterfaceString
	for _, e := range entries {
		if !known[e.Context+"\x04"+e.Source] || !packEntryValid(e, forms) {
			res.Dropped++
			continue
		}
		keep = append(keep, entity.InterfaceString{Lang: lang, Context: e.Context, Source: e.Source, Text: e.Text, Forms: e.Forms})
	}
	res.Kept = len(keep)
	installed, err := s.packs.Packs(ctx)
	if err != nil {
		return PackResult{}, err
	}
	res.Updated = slices.ContainsFunc(installed, func(p entity.InterfacePack) bool { return p.Lang == lang })
	p := entity.InterfacePack{Lang: lang, Version: l.Version, SHA256: l.SHA256, Strings: res.Kept, Draft: l.Draft, InstalledAt: s.now()}
	if err := s.packs.Install(ctx, p, keep); err != nil {
		return PackResult{}, err
	}
	return res, s.reloaded(ctx)
}

// packEntryValid - whether a pack's string may stand for its source: a
// text, or the language's number of plural forms, each passing
// i18n.CheckTranslation.
func packEntryValid(e i18n.Entry, forms int) bool {
	texts := []string{e.Text}
	if len(e.Forms) > 0 {
		if len(e.Forms) != forms || e.Text != "" {
			return false
		}
		texts = e.Forms
	}
	for _, t := range texts {
		if t == "" || i18n.CheckTranslation(e.Source, t) != nil {
			return false
		}
	}
	return true
}

// knownKeys - the strings a pack may translate (context "\x04" source):
// the code's, of every module and theme, and the active theme's
// catalogs'.
func (s *InterfaceStrings) knownKeys() map[string]bool {
	out := make(map[string]bool, len(s.sources))
	for _, src := range s.sources {
		out[src.Context+"\x04"+src.Source] = true
	}
	if s.themeSources != nil {
		for _, src := range s.themeSources() {
			out[src.Context+"\x04"+src.Source] = true
		}
	}
	return out
}

// DeletePack removes lang's downloaded translation (the site's own
// changes of its strings stay); the site's language can't be removed,
// nor a language that isn't downloaded (ErrPackNotInstalled).
func (s *InterfaceStrings) DeletePack(ctx context.Context, lang string) error {
	if s.packs == nil {
		return ErrUnknownPack
	}
	packs, err := s.packs.Packs(ctx)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(packs, func(p entity.InterfacePack) bool { return p.Lang == lang }) {
		return ErrPackNotInstalled
	}
	if lang == i18n.Default() {
		return ErrPackInUse
	}
	if err := s.packs.Delete(ctx, lang); err != nil {
		return err
	}
	return s.reloaded(ctx)
}

// PackState - where a language stands on the Languages tab.
type PackState string

// The states of a language.
const (
	PackSource    PackState = "source"
	PackBuiltIn   PackState = "builtin"
	PackInstalled PackState = "installed"
	PackUpdate    PackState = "update"
	PackAvailable PackState = "available"
)

// InterfaceLanguagesPage - the Languages tab: the languages (the source
// language, the built-in ones, the downloaded ones, the catalog's others),
// the catalog's address and when it was read, why it couldn't be read
// (CatalogErr; ErrNoTranslationsCatalog - none configured).
type InterfaceLanguagesPage struct {
	Languages      []InterfaceLanguage
	CatalogURL     string
	CatalogChecked time.Time
	CatalogErr     error
}

// LanguagesPage - the Languages tab; refresh reads the catalog again.
func (s *InterfaceStrings) LanguagesPage(ctx context.Context, refresh bool) (InterfaceLanguagesPage, error) {
	page := InterfaceLanguagesPage{CatalogURL: s.CatalogURL()}
	var packs []entity.InterfacePack
	if s.packs != nil {
		var err error
		if packs, err = s.packs.Packs(ctx); err != nil {
			return page, err
		}
	}
	index, checked, err := s.Catalog(ctx, refresh)
	page.CatalogChecked, page.CatalogErr = checked, err
	langs := s.Languages()
	have := map[string]bool{}
	for i := range langs {
		l := &langs[i]
		have[l.Code] = true
		switch {
		case l.Source:
			l.State = PackSource
		case slices.Contains(s.builtIn, l.Code):
			l.State = PackBuiltIn
		default:
			l.State = PackInstalled
			for j := range packs {
				if packs[j].Lang == l.Code {
					l.Pack = &packs[j]
				}
			}
			if c, ok := index.Language(l.Code); ok {
				l.Catalog = &c
				if l.Pack != nil && c.SHA256 != l.Pack.SHA256 {
					l.State = PackUpdate
				}
			}
		}
	}
	total := len(s.visible())
	for _, c := range index.Languages {
		if have[c.Code] || slices.Contains(s.builtIn, c.Code) {
			continue
		}
		langs = append(langs, InterfaceLanguage{Code: c.Code, Name: i18n.NativeName(c.Code), Total: total,
			Translated: min(c.Strings, total), State: PackAvailable, Catalog: &c})
	}
	order := map[PackState]int{PackSource: 0, PackBuiltIn: 1, PackInstalled: 2, PackUpdate: 2, PackAvailable: 3}
	slices.SortStableFunc(langs, func(a, b InterfaceLanguage) int {
		if d := order[a.State] - order[b.State]; d != 0 {
			return d
		}
		return strings.Compare(a.Code, b.Code)
	})
	page.Languages = langs
	return page, nil
}
