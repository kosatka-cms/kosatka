package app

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/modules"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// themeRuntime - what follows the active theme while the site runs: the
// interface translations (pkg/i18n: the site's own over the theme's
// catalogs, then the engine's and the modules', embedded), a previewed
// theme's translations and the switch to another theme on the Appearance
// page.
type themeRuntime struct {
	blocks    service.BlockService
	templates service.TemplateService
	cache     *service.CacheService
	logger    *slog.Logger
	// siteStrings - the site's own interface translations
	// (service.InterfaceStrings.Entries); nil - none.
	siteStrings func(ctx context.Context) ([]i18n.Entry, error)
	// packStrings - the downloaded translations' strings
	// (service.InterfaceStrings.PackEntries); nil - none.
	packStrings func(ctx context.Context) ([]i18n.Entry, error)

	previewMu sync.Mutex
	// preview - a previewed theme's translations, loaded on its first
	// preview (nil - they failed to load).
	preview map[string]*i18n.Translator
}

func newThemeRuntime(blocks service.BlockService, templates service.TemplateService, cache *service.CacheService, logger *slog.Logger) *themeRuntime {
	return &themeRuntime{blocks: blocks, templates: templates, cache: cache, logger: logger, preview: map[string]*i18n.Translator{}}
}

// translator loads the translations of the theme in dir: the catalogs of
// the theme and its base themes, the theme's first (its strings win), then
// those of the modules from outside the engine, then the downloaded
// languages; the site's own over them all.
func (t *themeRuntime) translator(dir string) (*i18n.Translator, error) {
	var catalogs []fs.FS
	for _, d := range paths.ThemeTranslationsDirsOf(dir) {
		catalogs = append(catalogs, i18n.DirFS(d))
	}
	tr, err := i18n.Load(append(catalogs, modules.Catalogs()...)...)
	if err != nil {
		return nil, err
	}
	if t.packStrings != nil {
		entries, err := t.packStrings(context.Background())
		if err != nil {
			return nil, err
		}
		tr = tr.WithPacks(entries)
	}
	if t.siteStrings != nil {
		entries, err := t.siteStrings(context.Background())
		if err != nil {
			return nil, err
		}
		tr = tr.WithSite(entries)
	}
	return tr, nil
}

// builtInLanguages - the interface's languages built into the binary:
// English, the language of the code's strings; every other is downloaded
// from the translations catalog.
func builtInLanguages() []string {
	return []string{i18n.SourceLanguage}
}

// reloadTranslations builds the site's translations again (the site
// changed one of its own): the active theme's, a previewed theme's on its
// next preview; the cached blocks follow.
func (t *themeRuntime) reloadTranslations(ctx context.Context) error {
	tr, err := t.translator(paths.Theme())
	if err != nil {
		return err
	}
	i18n.SetTranslator(tr)
	t.previewMu.Lock()
	t.preview = map[string]*i18n.Translator{}
	t.previewMu.Unlock()
	t.cache.ForceUpdate()
	return nil
}

// themeSources - the strings the active theme's chain's catalogs
// translate (service.InterfaceStrings: a theme from its own repository
// isn't in the sources list).
func (t *themeRuntime) themeSources() []i18n.Source {
	var out []i18n.Source
	for _, theme := range paths.ThemeChain(paths.Theme()) {
		dir := paths.ThemeDir(theme)
		if dir == "" {
			continue
		}
		sources, err := i18n.CatalogSources(i18n.DirFS(filepath.Join(dir, "translations")))
		if err != nil {
			t.logger.Warn("error reading a theme's catalogs", "error", err, "theme", theme)
			continue
		}
		for _, src := range sources {
			src.Theme = theme
			out = append(out, src)
		}
	}
	return out
}

// loadTranslations makes the active theme's translations the site's.
func (t *themeRuntime) loadTranslations() {
	tr, err := t.translator(paths.Theme())
	if err != nil {
		t.logger.Error("error loading interface translations", "error", err)
		return
	}
	i18n.SetTranslator(tr)
}

// previewTranslator - a previewed theme's translations
// (service.SetThemeTranslators).
func (t *themeRuntime) previewTranslator(dir string) *i18n.Translator {
	t.previewMu.Lock()
	defer t.previewMu.Unlock()
	if tr, ok := t.preview[dir]; ok {
		return tr
	}
	tr, err := t.translator(dir)
	if err != nil {
		t.logger.Error("error loading a previewed theme's translations", "error", err, "theme", dir)
		tr = nil
	}
	t.preview[dir] = tr
	return tr
}

// apply switches the active theme (the Appearance page): templates, the
// theme's translations and the cached blocks follow. A theme without its
// own block layout gets the previous theme's (in the regions it shares).
func (t *themeRuntime) apply(dir string) {
	previous := service.ActiveThemeKey()
	paths.SetTheme(dir)
	if info, err := themes.LoadInfoFor(dir); err == nil {
		if _, err := t.blocks.InitThemePlacements(context.Background(), entity.ThemeKey(dir), previous, info); err != nil {
			t.logger.Error("error initializing theme block placements", "error", err, "theme", dir)
		}
	}
	if ts, ok := t.templates.(*service.DefaultTemplateService); ok {
		ts.ResetCache()
	}
	t.loadTranslations()
	t.cache.ForceUpdate()
}
