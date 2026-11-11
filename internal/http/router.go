package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/paths"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// RegisterRoutes registers the site's own routes: the CSRF token for a
// page's script, the home page, the catch-all route serving content by
// its address (with the fallbacks for an address no node has: a View's
// page, /node/<id>) and the lazy blocks' and View blocks' pages.
func RegisterRoutes(
	r *http.ServeMux,
	store *sessions.CookieStore,
	nodeDispatcher *interfaces.NodeDispatcher,
	nodeRepo entity.NodesRepository,
	userRepo entity.UserRepository,
	authService service.AuthService,
	ic *interfaces.IndexController,
	notFoundHTML http.HandlerFunc,
	dynamicListFallback func(w http.ResponseWriter, r *http.Request) bool,
) {
	// The session's CSRF token for a page's script (csrf_head): reading a
	// page creates no session, so a form or a POST gets the token here.
	r.HandleFunc("GET /api/csrf-token", func(w http.ResponseWriter, req *http.Request) {
		token := csrf.EnsureToken(store, w, req)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	})

	// Home page
	r.HandleFunc("GET /{$}", ic.Index)

	// An address no node has: a View's page, else /node/<id> - to the
	// node's own address.
	nodeIDRedirect := middleware.NodeIDRedirect(nodeRepo.ByID, userRepo.ByID, authService)
	fallback := func(w http.ResponseWriter, r *http.Request) bool {
		return (dynamicListFallback != nil && dynamicListFallback(w, r)) || nodeIDRedirect(w, r)
	}
	nodeViewAccessBySlug := middleware.NodeAccessBySlugMiddleware(
		nodeRepo.BySlug,
		userRepo.ByID,
		authService,
		middleware.PermissionView,
		notFoundHTML,
		fallback,
	)

	optionalAuth := middleware.OptionalAuthMiddleware(store)

	// All routes by slug
	r.Handle("GET /",
		optionalAuth(
			nodeViewAccessBySlug(
				http.HandlerFunc(nodeDispatcher.View))))

	// Other routes
	r.HandleFunc("GET /api/block-placement/{id}", ic.BlockPlacementHTML)
	r.HandleFunc("GET /api/view-block/{id}", ic.ViewBlockPage)
}

// RegisterSearchRoutes - the live search and the results page; a query
// scans the content, so rl limits them per client.
func RegisterSearchRoutes(r Mux, sc *interfaces.SearchController, rl *ratelimit.RateLimiter) {
	limit := ratelimit.RateLimitMiddleware(rl, httputil.ClientIP)
	r.Handle("GET /api/search", limit(http.HandlerFunc(sc.LiveSearch)))
	r.Handle("GET /search", limit(http.HandlerFunc(sc.SearchPage)))
}

// UserRateLimits - the limiters of the account pages: per client
// (UserSearch, AuthForm, Password) and per account (LoginAccount - the
// sign-in tries at one account, whoever makes them; ResetAddress - the
// reset mails to one address).
type UserRateLimits struct {
	UserSearch, AuthForm, Password *ratelimit.RateLimiter
	LoginAccount, ResetAddress     *ratelimit.RateLimiter
}

// formAccount - the account a form names (its first field of names that
// is set, lowercased), the key of a per-account limiter.
func formAccount(names ...string) func(*http.Request) string {
	return func(r *http.Request) string {
		for _, n := range names {
			if v := strings.TrimSpace(r.FormValue(n)); v != "" {
				return "account:" + strings.ToLower(v)
			}
		}
		return ""
	}
}

// RegisterUserRoutes registers the account routes: sign-in (with its
// two-factor step), sign-out, sign-up, password reset, invitations, the
// social sign-in (when oc is set), the profiles and their tabs, and
// editing and cancelling an account, rate limited by limits.
func RegisterUserRoutes(r *http.ServeMux, store *sessions.CookieStore, uc *interfaces.UserController, oc *interfaces.OAuthController, limits UserRateLimits) {
	// API endpoints (auth + rate limit)
	auth := middleware.AuthMiddleware(store)
	rl := ratelimit.RateLimitMiddleware(limits.UserSearch, httputil.ClientIP)
	r.Handle("GET /api/users/search", auth(rl(http.HandlerFunc(uc.SearchUsers))))

	authFormMW := ratelimit.RateLimitMiddleware(limits.AuthForm, httputil.ClientIP)
	loginAccountMW := ratelimit.RateLimitMiddleware(limits.LoginAccount, formAccount("login", "email"))
	// Login / profile
	r.Handle("POST /login", authFormMW(loginAccountMW(http.HandlerFunc(uc.Login))))
	r.HandleFunc("GET /login", uc.Login)
	// Logout
	r.HandleFunc("GET /logout", uc.LogoutConfirm)
	r.HandleFunc("POST /logout", uc.Logout)
	// Register
	r.HandleFunc("GET /user/register", uc.Signup)
	r.Handle("POST /user/register", authFormMW(http.HandlerFunc(uc.Signup)))
	r.HandleFunc("GET /user/confirm-email", uc.ConfirmEmail)
	r.HandleFunc("GET /user/login-once", uc.LoginOnce)
	// The second step of a sign-in (two-factor), and the user's own page
	// turning it on and off.
	r.HandleFunc("GET /login/code", uc.LoginCode)
	r.Handle("POST /login/code", authFormMW(http.HandlerFunc(uc.LoginCode)))
	r.Handle("GET "+interfaces.TwoFactorPath, auth(http.HandlerFunc(uc.TwoFactor)))
	r.Handle("POST "+interfaces.TwoFactorPath+"/enable", auth(authFormMW(http.HandlerFunc(uc.TwoFactorEnable))))
	r.Handle("POST "+interfaces.TwoFactorPath+"/codes", auth(authFormMW(http.HandlerFunc(uc.TwoFactorCodes))))
	r.Handle("POST "+interfaces.TwoFactorPath+"/disable", auth(authFormMW(http.HandlerFunc(uc.TwoFactorDisable))))

	if oc != nil {
		r.Handle("GET /auth/{provider}", authFormMW(http.HandlerFunc(oc.OAuthStart)))
		r.Handle("GET /auth/{provider}/callback", authFormMW(http.HandlerFunc(oc.OAuthCallback)))
		r.Handle("POST /auth/{provider}/disconnect", auth(http.HandlerFunc(oc.Disconnect)))
		r.Handle("GET /user/complete-profile", auth(http.HandlerFunc(oc.CompleteProfile)))
		r.Handle("POST /user/complete-profile", auth(authFormMW(http.HandlerFunc(oc.CompleteProfile))))
	}

	pwRL := ratelimit.RateLimitMiddleware(limits.Password, httputil.ClientIP)
	resetAddressRL := ratelimit.RateLimitMiddleware(limits.ResetAddress, formAccount("email"))
	r.HandleFunc("GET /user/password", uc.PasswordRequest)
	r.Handle("POST /user/password", pwRL(resetAddressRL(http.HandlerFunc(uc.PasswordRequest))))
	r.HandleFunc("GET /user/reset-password", uc.ResetPassword)
	r.Handle("POST /user/reset-password", pwRL(http.HandlerFunc(uc.ResetPassword)))
	r.HandleFunc("GET /user/invite", uc.AcceptInvite)
	r.Handle("POST /user/invite", pwRL(http.HandlerFunc(uc.AcceptInvite)))

	// Profile routes
	r.HandleFunc("GET /me", uc.Profile)
	// /me/{tab}, /users/{alias}/{tab} - the profile's tabs of the modules
	// (interfaces.AddProfileTab) and of the Views (user_tab displays);
	// built-in tabs keep their own more specific routes.
	r.HandleFunc("GET /me/files", uc.Profile)
	r.HandleFunc("GET /me/{tab}", uc.Profile)
	profiles := "/" + entity.UserPathPrefix + "/{alias}"
	r.HandleFunc("GET "+profiles, uc.Profile)
	r.HandleFunc("GET "+profiles+"/{tab}", uc.Profile)

	// Profile edit
	r.Handle("GET /me/edit", auth(http.HandlerFunc(uc.EditForm)))
	r.Handle("POST /me/edit", auth(http.HandlerFunc(uc.EditSave)))
	// Deleting one's own account (the permission "cancel own account").
	r.Handle("POST /me/cancel", auth(http.HandlerFunc(uc.CancelAccount)))
	r.Handle("GET /user/{id}/edit", auth(http.HandlerFunc(uc.EditForm)))
	r.Handle("POST /user/{id}/edit", auth(http.HandlerFunc(uc.EditSave)))
}

// RegisterUserFilesRoutes registers JSON API for per-user uploads (session + CSRF on mutating requests).
func RegisterUserFilesRoutes(r *http.ServeMux, store *sessions.CookieStore, ufc *interfaces.UserFilesController) {
	authJSON := middleware.AuthJSONMiddleware(store)
	r.Handle("GET /api/user/files", authJSON(http.HandlerFunc(ufc.List)))
	r.Handle("GET /api/user/files/tree", authJSON(http.HandlerFunc(ufc.Tree)))
	r.Handle("GET /api/user/files/usage", authJSON(http.HandlerFunc(ufc.Usage)))
	r.Handle("POST /api/user/files/upload", authJSON(http.HandlerFunc(ufc.Upload)))
	r.Handle("POST /api/user/files/mkdir", authJSON(http.HandlerFunc(ufc.Mkdir)))
	r.Handle("POST /api/user/files/rmdir", authJSON(http.HandlerFunc(ufc.Rmdir)))
	r.Handle("POST /api/user/files/unlink", authJSON(http.HandlerFunc(ufc.Unlink)))
	r.Handle("POST /api/user/files/rename", authJSON(http.HandlerFunc(ufc.Rename)))
}

// RegisterNodeSearchRoutes - the cross-type node title search (see
// interfaces.NodeSearchController) behind the node pickers of edit forms
// (menu links, reference fields); signed-in users only.
func RegisterNodeSearchRoutes(r *http.ServeMux, store *sessions.CookieStore, nsc *interfaces.NodeSearchController, rl *ratelimit.RateLimiter) {
	limit := ratelimit.RateLimitMiddleware(rl, httputil.ClientIP)
	r.Handle("GET /api/nodes/search", middleware.AuthJSONMiddleware(store)(limit(http.HandlerFunc(nsc.Search))))
}

// RegisterEmbedsRoutes - the editor's "Insert" window and the cards over
// the embed tags (interfaces.EmbedsController); signed-in users only.
func RegisterEmbedsRoutes(r *http.ServeMux, store *sessions.CookieStore, ec *interfaces.EmbedsController) {
	auth := middleware.AuthJSONMiddleware(store)
	r.Handle("GET /api/embeds/kinds", auth(http.HandlerFunc(ec.Kinds)))
	r.Handle("GET /api/embeds/search", auth(http.HandlerFunc(ec.Search)))
	r.Handle("GET /api/embeds/describe", auth(http.HandlerFunc(ec.Describe)))
}

// RegisterPrivateFilesRoutes - the private files of file fields, served
// after the access check (interfaces.PrivateFilesController).
func RegisterPrivateFilesRoutes(r *http.ServeMux, pc *interfaces.PrivateFilesController) {
	r.HandleFunc("GET "+paths.PrivateFilesURL+"{filepath...}", pc.Serve)
}

// RegisterStylesRoutes registers <public files>/styles/{style}/…: a
// public image in an image style (StylesController.ServeStyledImage).
func RegisterStylesRoutes(r *http.ServeMux, sc *interfaces.StylesController) {
	r.HandleFunc("GET "+paths.PublicFilesURL()+"styles/{style}/{filepath...}", sc.ServeStyledImage)
}

// RegisterTaxonomyRoutes registers the taxonomy API. Term pages
// themselves are served by the catch-all route's fallback
// (TaxonomyController.TermPageFallback).
func RegisterTaxonomyRoutes(r *http.ServeMux, store *sessions.CookieStore, tc *interfaces.TaxonomyController, rl *ratelimit.RateLimiter) {
	// The content forms' tag pickers: signed in, rate limited.
	r.Handle("GET /api/taxonomy/autocomplete", middleware.AuthMiddleware(store)(ratelimit.RateLimitMiddleware(rl, httputil.ClientIP)(http.HandlerFunc(tc.Autocomplete))))
}

// RegisterViewCountRoutes registers POST /api/node/view, the page's
// report of a view of its node, limited per client by rl.
func RegisterViewCountRoutes(r *http.ServeMux, vc *interfaces.ViewCountController, rl *ratelimit.RateLimiter) {
	r.Handle("POST /api/node/view", ratelimit.RateLimitMiddleware(rl, httputil.ClientIP)(http.HandlerFunc(vc.PostView)))
}

// RegisterStaticRoutes serves /static (versioned assets) and the public
// files (uploads: paths.PublicStore at paths.PublicFilesURL).
func RegisterStaticRoutes(r *http.ServeMux) {
	r.Handle("GET /static/{filepath...}", http.StripPrefix("/static/", FileServerWithCache(paths.StaticDir(), CacheStatic)))
	// A module's own files (internal/modules/<name>/static) at the same
	// address its files had under web/static/modules/<name>/.
	r.Handle("GET /static/modules/{module}/{filepath...}", moduleStatic(FileServerWithCache(paths.StaticDir(), CacheStatic)))
	// A theme's own files (themes/<theme>/static) at the address its files
	// had under web/static/themes/<theme>/.
	r.Handle("GET /static/themes/{theme}/{filepath...}", themeStatic(FileServerWithCache(paths.StaticDir(), CacheStatic)))
	r.Handle("GET "+paths.PublicFilesURL()+"{filepath...}", http.StripPrefix(paths.PublicFilesURL(), UploadsFileServer(paths.PublicStore(), paths.PublicStoreURL, CacheUploads)))
	r.HandleFunc("GET /i18n/{file}", serveJSTranslations)
	r.HandleFunc("GET /favicon.ico", serveFavicon)
}

// serveFavicon - GET /favicon.ico, which browsers ask for themselves: a
// redirect to the pages' icon in the request's theme (service.SiteIconFor,
// whatever its format), 404 without one.
func serveFavicon(w http.ResponseWriter, r *http.Request) {
	icon := service.SiteIconFor(paths.ThemeFor(r.Context()))
	if icon.URL == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.Redirect(w, r, icon.URL, http.StatusFound)
}

// serveJSTranslations - GET /i18n/<lang>.js: the JavaScript translations
// and runtime (i18n.JSScript). The URL carries ?v=<hash> (template func
// jsTranslationsURL), so it is cached for a year.
func serveJSTranslations(w http.ResponseWriter, r *http.Request) {
	lang, ok := strings.CutSuffix(r.PathValue("file"), ".js")
	if !ok || !i18n.Current().Available(lang) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(i18n.JSScript(lang))
}

// RegisterSitemapRoutes registers robots.txt and the sitemap (rendered as
// for a guest): its index, the static pages and the content in chunks.
func RegisterSitemapRoutes(r *http.ServeMux, sc *interfaces.SitemapController) {
	r.HandleFunc("GET /robots.txt", sc.Robots)
	r.HandleFunc("HEAD /robots.txt", sc.Robots)
	// Public by nature, kept by any cache: as for a guest, whoever asks.
	r.Handle("GET /sitemap.xml", interfaces.AsGuestHandler(http.HandlerFunc(sc.Index)))
	r.Handle("GET /sitemap/static.xml", interfaces.AsGuestHandler(http.HandlerFunc(sc.Static)))
	r.Handle("GET /sitemap/nodes/{n}", interfaces.AsGuestHandler(http.HandlerFunc(sc.NodesChunk)))
}

// RegisterRssRoutes registers the site's RSS feed (/rss.xml), rendered as
// for a guest.
func RegisterRssRoutes(r *http.ServeMux, rc *interfaces.RssController) {
	r.Handle("GET /rss.xml", interfaces.AsGuestHandler(http.HandlerFunc(rc.Feed)))
}

// RegisterCommentRoutes registers the comments' routes: the recent ones,
// a comment's link, a node's comments (limited by commentsRL), posting
// one (signed in, limited by commentPostRL), editing and deleting.
func RegisterCommentRoutes(r *http.ServeMux, store *sessions.CookieStore, cc *interfaces.CommentController, commentsRL, commentPostRL *ratelimit.RateLimiter) {
	// Public routes
	r.HandleFunc("GET /comments/recent", cc.Recent)
	r.HandleFunc("GET /comment/{id}", cc.Redirect)
	rl := ratelimit.RateLimitMiddleware(commentsRL, httputil.ClientIP)
	r.Handle("GET /api/comments/{nodeId}", rl(http.HandlerFunc(cc.LoadComments)))

	// Authenticated routes
	auth := middleware.AuthMiddleware(store)
	// Posting a comment: its own limit, tighter than reading them.
	r.Handle("POST /comment/create", auth(ratelimit.RateLimitMiddleware(commentPostRL, httputil.ClientIP)(http.HandlerFunc(cc.Create))))

	// EditForm and delete routes (permission checks inside handlers)
	r.HandleFunc("GET /comment/{id}/edit", cc.Edit)
	r.HandleFunc("POST /comment/{id}/edit", cc.Edit)
	r.HandleFunc("GET /comment/{id}/delete", cc.DeleteConfirm)
	r.HandleFunc("POST /comment/{id}/delete", cc.Delete)
}

// RegisterNodeRoutes registers creating, editing, deleting, cloning,
// publishing and unpublishing content (each behind the node access check
// its action needs), its preview links and history, the media picker of
// its forms and the recent changes (RegisterRecentChangesRoute).
func RegisterNodeRoutes(
	r *http.ServeMux,
	store *sessions.CookieStore,
	nodeController *interfaces.NodeController,
	nodeRepo entity.NodesRepository,
	userRepo entity.UserRepository,
	authService service.AuthService,
	nodeDispatcher *interfaces.NodeDispatcher,
	notFoundHTML http.HandlerFunc,
) {
	nodeEditAccessByID := middleware.NodeAccessByIDMiddleware(
		nodeRepo.ByID,
		userRepo.ByID,
		authService,
		middleware.PermissionEdit,
		notFoundHTML,
	)

	nodeDeleteAccessByID := middleware.NodeAccessByIDMiddleware(
		nodeRepo.ByID,
		userRepo.ByID,
		authService,
		middleware.PermissionDelete,
		notFoundHTML,
	)

	nodeCreateAccess := middleware.NodeCreateAccessByTypeMiddleware(
		userRepo.ByID,
		authService,
	)

	nodePublishAccessByID := middleware.NodeAccessByIDMiddleware(
		nodeRepo.ByID,
		userRepo.ByID,
		authService,
		middleware.PermissionPublish,
		notFoundHTML,
	)

	authRequired := middleware.OptionalAuthMiddleware(store)

	r.Handle("GET /create/{type}", authRequired(nodeCreateAccess(http.HandlerFunc(nodeDispatcher.CreateForm))))
	r.Handle("POST /create/{type}", authRequired(nodeCreateAccess(http.HandlerFunc(nodeDispatcher.Create))))
	// The form previewed (its Preview button): who may submit it.
	r.Handle("POST /create/{type}/preview", authRequired(nodeCreateAccess(http.HandlerFunc(nodeDispatcher.CreatePreview))))

	r.Handle("GET /node/{id}/edit", authRequired(nodeEditAccessByID(http.HandlerFunc(nodeDispatcher.Edit))))
	r.Handle("POST /node/{id}/edit", authRequired(nodeEditAccessByID(http.HandlerFunc(nodeDispatcher.Save))))
	r.Handle("POST /node/{id}/edit/preview", authRequired(nodeEditAccessByID(http.HandlerFunc(nodeDispatcher.EditPreview))))

	r.Handle("GET /node/{id}/delete", authRequired(nodeDeleteAccessByID(http.HandlerFunc(nodeController.DeleteConfirm))))
	r.Handle("POST /node/{id}/delete", authRequired(nodeDeleteAccessByID(http.HandlerFunc(nodeDispatcher.Delete))))
	// A copy: who may edit the node (and copy content, create its type's
	// content: the handler checks).
	r.Handle("POST /node/{id}/clone", authRequired(nodeEditAccessByID(http.HandlerFunc(nodeDispatcher.Clone))))
	// Preview links: made and revoked by who may edit the node, opened by
	// whoever has one.
	r.Handle("POST /node/{id}/preview-link", authRequired(nodeEditAccessByID(http.HandlerFunc(nodeDispatcher.PreviewLink))))
	r.Handle("GET /preview/{token}", authRequired(http.HandlerFunc(nodeDispatcher.Preview)))
	// The media library's files an image or file field may take (its
	// picker); signed-in users who may fill the field.
	r.Handle("GET /api/media/pick", middleware.AuthJSONMiddleware(store)(http.HandlerFunc(nodeDispatcher.MediaPick)))

	r.Handle("POST /node/{id}/unpublish", authRequired(nodePublishAccessByID(http.HandlerFunc(nodeController.Unpublish))))
	r.Handle("POST /node/{id}/publish", authRequired(nodePublishAccessByID(http.HandlerFunc(nodeController.Publish))))

	// The node's history: for who may edit it (the handlers check the
	// history's permissions).
	r.Handle("GET /node/{id}/revisions", authRequired(nodeEditAccessByID(http.HandlerFunc(nodeController.Revisions))))
	r.Handle("GET /node/{id}/revisions/compare", authRequired(nodeEditAccessByID(http.HandlerFunc(nodeController.RevisionsCompare))))
	r.Handle("POST /node/{id}/revisions/{rev}/revert", authRequired(nodeEditAccessByID(http.HandlerFunc(nodeController.RevisionRevert))))
	RegisterRecentChangesRoute(r, store, userRepo, authService, nodeController)
}

// RegisterRecentChangesRoute registers the recent changes of all the
// content (Content → Changes): an admin page; the handler checks the
// history's permission and which types' content to show.
func RegisterRecentChangesRoute(r *http.ServeMux, store *sessions.CookieStore, userRepo entity.UserRepository, authService service.AuthService, nodeController *interfaces.NodeController) {
	adminAccess := middleware.PermissionMiddleware(userRepo, authService, service.PermAccessAdmin)
	r.Handle("GET "+interfaces.RecentChangesPath, middleware.OptionalAuthMiddleware(store)(adminAccess(http.HandlerFunc(nodeController.RecentChanges))))
}
