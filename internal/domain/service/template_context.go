package service

import (
	"context"
	"net/http"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// requestZone - the time zone of r (sitetime.FromContext) when it is a
// user's own, nil for the site's (the shared template set).
func requestZone(r *http.Request) *time.Location {
	if r == nil {
		return nil
	}
	if loc := sitetime.FromContext(r.Context()); !sitetime.IsSite(loc) {
		return loc
	}
	return nil
}

// RequestLocationKey - the render data key of a fragment's time zone
// (RenderNamed has no request): set it from the request
// (sitetime.FromContext) to render in a user's own zone.
const RequestLocationKey = "requestLocation"

// dataZone - the RequestLocationKey zone of a RenderNamed data map when
// it is a user's own, nil for the site's.
func dataZone(data any) *time.Location {
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	if loc, ok := m[RequestLocationKey].(*time.Location); ok && !sitetime.IsSite(loc) {
		return loc
	}
	return nil
}

// RequestLanguageKey - the render data key of a fragment's language
// (RenderNamed has no request), like RequestLocationKey.
const RequestLanguageKey = "requestLanguage"

// requestLang - r's language (i18n.FromContext) when it isn't the
// site's default, "" otherwise (the shared template set).
func requestLang(r *http.Request) string {
	if r == nil {
		return ""
	}
	if l := i18n.FromContext(r.Context()); l != i18n.Default() {
		return l
	}
	return ""
}

// dataLang - the RequestLanguageKey language of a RenderNamed data map
// when it isn't the site's default.
func dataLang(data any) string {
	m, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	if l, ok := m[RequestLanguageKey].(string); ok && l != "" && l != i18n.Default() {
		return l
	}
	return ""
}

// RequestThemeKey - the render data key of a fragment's theme when it
// isn't the active one (a theme's preview; RenderNamed has no request),
// like RequestLanguageKey: the theme's placement key (entity.ThemeKey),
// set from RequestTheme.
const RequestThemeKey = "requestTheme"

// RequestTheme - the RequestThemeKey value of ctx: its preview theme's
// key (paths.WithTheme), "" - the active theme.
func RequestTheme(ctx context.Context) string {
	if dir, ok := paths.PreviewTheme(ctx); ok {
		return entity.ThemeKey(dir)
	}
	return ""
}

// themeScope - the theme a render resolves its templates in: the active
// one (the zero value), or a previewed one (set; dir "" - the engine's
// templates).
type themeScope struct {
	dir string
	set bool
}

// requestTheme - r's preview theme (paths.WithTheme), if any.
func requestTheme(r *http.Request) themeScope {
	if r == nil {
		return themeScope{}
	}
	dir, ok := paths.PreviewTheme(r.Context())
	return themeScope{dir: dir, set: ok}
}

// dataTheme - the RequestThemeKey theme of a RenderNamed data map, if any.
func dataTheme(data any) themeScope {
	m, ok := data.(map[string]any)
	if !ok {
		return themeScope{}
	}
	if key, ok := m[RequestThemeKey].(string); ok && key != "" {
		return themeScope{dir: entity.ThemeDir(key), set: true}
	}
	return themeScope{}
}

// name - the theme's directory ("" - the engine's templates).
func (s themeScope) name() string {
	if s.set {
		return s.dir
	}
	return paths.Theme()
}

// resolve - paths.ResolveTemplate in the scope's theme.
func (s themeScope) resolve(rel string) (string, error) {
	return paths.ResolveTemplateIn(s.name(), rel)
}

// glob - paths.GlobTemplates in the scope's theme.
func (s themeScope) glob(pattern string) ([]string, error) {
	return paths.GlobTemplatesIn(s.name(), pattern)
}

// moduleGlob - paths.ModuleTemplatesGlobIn the scope's theme: the
// modules' templates, the theme's (or its base's) copies over theirs.
func (s themeScope) moduleGlob(pattern string) ([]string, error) {
	return paths.ModuleTemplatesGlobIn(s.name(), pattern)
}

// translator - the scope's interface translations: a previewed theme's
// own catalogs (SetThemeTranslators), else the active theme's.
func (s themeScope) translator() *i18n.Translator {
	if s.set && themeTranslators != nil {
		if tr := themeTranslators(s.dir); tr != nil {
			return tr
		}
	}
	return i18n.Current()
}

// ThemeText - source translated for theme dir in ctx's language (plain
// text): with that theme's catalogs when it isn't the active one (its
// name and settings' labels on the Appearance pages).
func ThemeText(ctx context.Context, dir, source string) string {
	tr := themeScope{dir: dir, set: dir != paths.Theme()}.translator()
	return tr.Text(i18n.FromContext(ctx), "", source)
}

// themeTranslators - the translations with a given theme's catalogs (a
// previewed theme's strings; the active theme's are i18n.Current).
var themeTranslators func(dir string) *i18n.Translator

// SetThemeTranslators - how a previewed theme's translations load
// (wiring: its catalogs, then the engine's and the modules').
func SetThemeTranslators(f func(dir string) *i18n.Translator) {
	themeTranslators = f
}
