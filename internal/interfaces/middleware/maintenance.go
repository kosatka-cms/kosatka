package middleware

import (
	"context"
	"html"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/paths"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

type maintenanceCtxKey struct{}

type maintenancePayload struct {
	Enabled bool
	Message string
}

// MaintenanceContextMiddleware stores maintenance mode flag and message in request context
// so handlers can show a banner for users allowed to browse during maintenance.
// Must wrap the same routes as MaintenanceMiddleware and run outside it (executed first on incoming requests).
func MaintenanceContextMiddleware(svc service.MaintenanceService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !svc.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}
			p := &maintenancePayload{Enabled: true, Message: svc.GetMessage()}
			ctx := context.WithValue(r.Context(), maintenanceCtxKey{}, p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MaintenancePayloadFromRequest returns maintenance-on flag and admin message HTML (may be empty).
func MaintenancePayloadFromRequest(r *http.Request) (enabled bool, message string) {
	v := r.Context().Value(maintenanceCtxKey{})
	if p, ok := v.(*maintenancePayload); ok && p != nil {
		return p.Enabled, p.Message
	}
	return false, ""
}

// paths that are always allowed during maintenance (login, logout, static assets,
// public files (paths.PublicFilesURL) and read-only HTML/JSON fragments used
// by the public page shell so /login layout is not broken); the enabled
// modules add theirs (entity.Module.MaintenancePaths)
var maintenanceSkipPrefixes = []string{
	"/login",
	"/logout",
	"/static/",
	"/i18n/",
	"/api/block-placement/",
}

func shouldSkipMaintenanceCheck(path string) bool {
	if strings.HasPrefix(path, paths.PublicFilesURL()) {
		return true
	}
	for _, p := range maintenanceSkipPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	for _, m := range entity.Modules {
		if !service.ModuleEnabled(m.Name) {
			continue
		}
		for _, p := range m.MaintenancePaths {
			if strings.HasPrefix(path, p) {
				return true
			}
		}
	}
	return false
}

// maintenanceSEOFromSite - "Name | Slogan" from site information (empty
// parts omitted), with the slogan as description.
func maintenanceSEOFromSite(site *entity.SiteInformation) (title, description string) {
	var siteName, slogan string
	if site != nil {
		siteName, slogan = strings.TrimSpace(site.SiteName), strings.TrimSpace(site.Slogan)
	}
	switch {
	case siteName != "" && slogan != "":
		title = siteName + " | " + slogan
	case siteName != "":
		title = siteName
	default:
		title = slogan
	}
	return title, slogan
}

// unavailableHTML - the page of a 503 (maintenance, the database gone):
// msg (HTML) on a page of its own, titled by the site, needing nothing
// but what is in memory.
func unavailableHTML(msg string, site *entity.SiteInformation) []byte {
	title, desc := maintenanceSEOFromSite(site)
	titleEsc := html.EscapeString(title)
	descEsc := html.EscapeString(desc)
	return []byte(`<!DOCTYPE html>
<html lang="` + html.EscapeString(i18n.Default()) + `">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + titleEsc + `</title>
<meta name="description" content="` + descEsc + `">
<style>
body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center; font-family: system-ui, sans-serif; padding: 20px; }
.content { max-width: 600px; text-align: center; }
.login-link { display: inline-block; margin-top: 1.5rem; padding: 0.5rem 1rem; background: #c0392b; color: #fff; text-decoration: none; border-radius: 4px; }
.login-link:hover { background: #a93226; color: #fff; }
</style>
</head>
<body><div class="content">` + msg + `</div></body>
</html>`)
}

// MaintenanceMiddleware returns 503 with maintenance message for users without permission when maintenance is enabled
func MaintenanceMiddleware(
	svc service.MaintenanceService,
	authSvc service.AuthService,
	userRepo entity.UserRepository,
	store *sessions.CookieStore,
	siteRepo entity.SiteInformationRepository,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			if shouldSkipMaintenanceCheck(path) {
				next.ServeHTTP(w, r)
				return
			}

			if !svc.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			// Maintenance is on: check whether the user may bypass it.
			session, err := store.Get(r, entity.SessionName)
			if err == nil {
				if userID, ok := session.Values["user_id"].(int); ok && userID != 0 {
					user, err := userRepo.ByID(r.Context(), userID)
					if err == nil && user != nil && authSvc.CanUseSiteInMaintenanceMode(r.Context(), user) {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			button := ""

			msg := svc.GetMessage()
			if msg == "" {
				msg = "<p>" + html.EscapeString(i18n.T(r.Context(), "The site is under maintenance.")) + "</p>"
			}
			var site *entity.SiteInformation
			if siteRepo != nil {
				if si, err := siteRepo.Get(r.Context()); err == nil {
					site = si
				}
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write(unavailableHTML(msg+button, site))
		})
	}
}
