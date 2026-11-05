package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/i18nextract"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/modules"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// newSessionStore - the session cookies' store. The secret must be set in
// production: a fixed default would be a shared, guessable key for forging
// session cookies. Elsewhere a random ephemeral key is used (sessions
// don't survive a restart).
func newSessionStore(cfg *config.Config, logger *slog.Logger) *sessions.CookieStore {
	secret := cfg.Session.SecretKey
	if secret == "" {
		if cfg.IsProd() {
			logger.Error("session.secret_key (SESSION_SECRET_KEY) is not set; refusing to start in production with no session secret")
			panic("session.secret_key (SESSION_SECRET_KEY) must be set when env=prod")
		}
		secret = generateSessionSecret()
		logger.Warn("Session secret key not configured; generated a random ephemeral key for this process. Existing sessions are now invalid, and every restart will invalidate sessions again. Set session.secret_key / SESSION_SECRET_KEY for a stable key.")
	}
	store := sessions.NewCookieStore([]byte(secret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   cfg.Session.MaxAge,
		HttpOnly: true,
		Secure:   cfg.Session.Secure,
		// Lax: the browser doesn't send the session with cross-site POSTs
		// (defence in depth on top of the CSRF token check).
		SameSite: http.SameSiteLaxMode,
	}
	// The cookie's lifetime is also the server's: the codecs refuse an
	// older cookie (their own default is 30 days, whatever the cookie
	// says).
	store.MaxAge(cfg.Session.MaxAge)
	return store
}

// newTemplateService - the page templates. Parsed templates are cached
// everywhere except local dev, where template edits apply without a
// restart (elsewhere a deploy restarts the process). The modules'
// templates and static files are the binary's own, except in local dev,
// where a module's source directory comes first (edits apply without a
// rebuild).
func newTemplateService(cfg *config.Config, logger *slog.Logger) service.TemplateService {
	local := cfg.IsLocal()
	paths.SetModuleAssets(modules.Assets(), !local)
	templates := service.NewTemplateService(!local)
	if cfg.TemplatesDebug {
		if ts, ok := templates.(*service.DefaultTemplateService); ok {
			ts.SetDebug(true)
		}
		if !local {
			logger.Warn("template debugging is on: the pages name their template files (templates_debug, TEMPLATES_DEBUG)")
		}
	}
	return templates
}

// bootSiteSettings applies the saved site information to the process-wide
// settings before the first page (the cache's first refresh already
// renders View blocks in the site's theme, zone and language): the theme
// chosen on the Appearance page (it overrides the config's), the time
// zone, the default language, the icon, the site URL and the error pages.
// The admin forms update them on save.
func bootSiteSettings(ctx context.Context, site *database.SiteInformationRepository, siteURL *service.SiteURL, nodeBySlug func(context.Context, string) (*entity.Node, error), logger *slog.Logger) {
	info, err := site.Get(ctx)
	if err != nil {
		logger.Error("error loading the site time zone", "error", err)
		return
	}
	if info.Theme != nil {
		paths.SetTheme(*info.Theme)
	}
	if info.Timezone != "" {
		if err := sitetime.Set(info.Timezone); err != nil {
			logger.Error("invalid site time zone", "error", err, "timezone", info.Timezone)
		}
	}
	i18n.SetDefault(info.DefaultLanguage)
	entity.SetSiteLanguages(i18n.Default(), info.Languages)
	service.SetSiteIcon(info.Icon.URI)
	siteURL.Apply(info.BaseURL)
	service.LoadErrorPages(ctx, nodeBySlug, info)
}

// bootUserZones turns the users' own time zones (an account setting,
// pkg/sitetime) and the "Register" links (entity.SetRegistrationMode) on
// or off as saved; the account settings form updates them.
func bootUserZones(ctx context.Context, account *database.AccountSettingsRepository) {
	if acc, err := account.Get(ctx); err == nil && acc != nil {
		sitetime.SetUserZones(acc.UserTimezones)
		entity.SetRegistrationMode(acc.RegistrationMode)
	}
}

// bootJSKeys collects the strings the JavaScript translates (Kosatka.t…),
// served at /i18n/<lang>.js: the engine's and the themes' on disk - in
// the engine directory (ENGINE_DIR, engine_dir) and, beside it, the
// site's own themes in its working directory - the modules' from their
// embedded files.
func bootJSKeys(logger *slog.Logger) {
	roots := []string{"."}
	if engine := paths.EngineDir(); engine != "" {
		roots = append([]string{engine}, roots...)
	}
	i18n.SetJSKeys(jsKeysIn(logger, roots))
}

// jsKeysIn - the JavaScript strings of the roots' engine and theme files
// and of the modules, each once.
func jsKeysIn(logger *slog.Logger, roots []string) []i18n.JSKey {
	var keys []i18n.JSKey
	seen := map[i18n.JSKey]bool{}
	for _, root := range roots {
		found, err := i18nextract.ExtractJS(root, modules.Assets())
		if err != nil {
			logger.Error("error collecting JavaScript interface strings", "error", err, "dir", root)
			continue
		}
		for _, k := range found {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	return keys
}
