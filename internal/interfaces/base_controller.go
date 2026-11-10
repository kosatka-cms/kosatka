package interfaces

import (
	"bytes"
	"context"
	"html"
	"html/template"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// captcha - the reCAPTCHA in effect (SetCaptcha): site keys per form for
// templates and response checks for the forms that show it.
var captcha *service.CaptchaService

// oauthProviders lists the OAuth providers offered on the login and
// registration pages; read on every request, as the admin can change them.
var oauthProviders func() []string

// SetCaptcha sets the reCAPTCHA service (wiring).
func SetCaptcha(s *service.CaptchaService) {
	captcha = s
}

// SetOAuthProviders sets the source of the OAuth providers offered on the
// login and registration pages (wiring).
func SetOAuthProviders(providers func() []string) {
	oauthProviders = providers
}

// BaseController holds dependencies and helpers shared by all controllers.
type BaseController struct {
	userService         service.UserService
	authService         service.AuthService
	commentService      service.CommentsService
	templateService     service.TemplateService
	cacheService        BlockCache
	viewCountingService service.ViewCountingService
	store               *sessions.CookieStore
	logger              *slog.Logger
	siteInformationRepo entity.SiteInformationRepository // optional; for EditorPageSEO (/create, /node/.../edit forms)
	nodeRepo            entity.NodesRepository           // for the sidebar moderation widget
	// breadcrumbs - the pages' trails (nil - no crumbs).
	breadcrumbs *service.BreadcrumbService
}

// BlockCache - what of the blocks' cache (service.CacheService) the pages
// read: their sidebar data, View blocks and a View block's pages.
type BlockCache interface {
	SidebarDataFor(ctx context.Context) service.SidebarData
	ViewBlockContentFor(ctx context.Context, zone *time.Location) map[int]template.HTML
	ViewBlockPage(ctx context.Context, view entity.ViewDisplayDef, placementID, page int, zone *time.Location) (template.HTML, bool, error)
}

// BaseDeps - what every controller's base needs: the wiring builds it
// once and passes it to each controller's constructor.
type BaseDeps struct {
	Auth      service.AuthService
	Templates service.TemplateService
	Users     service.UserService
	Comments  service.CommentsService
	// Cache - the blocks' cache (nil - the pages render without blocks).
	Cache           BlockCache
	ViewCounting    service.ViewCountingService
	Store           *sessions.CookieStore
	Logger          *slog.Logger
	SiteInformation entity.SiteInformationRepository
	Nodes           entity.NodesRepository
	// Breadcrumbs - the pages' trails (nil - the pages render none).
	Breadcrumbs *service.BreadcrumbService
}

// NewBaseController - the base of a controller (embedded by each).
func NewBaseController(d BaseDeps) *BaseController {
	return &BaseController{
		authService:         d.Auth,
		templateService:     d.Templates,
		userService:         d.Users,
		commentService:      d.Comments,
		cacheService:        d.Cache,
		viewCountingService: d.ViewCounting,
		store:               d.Store,
		logger:              d.Logger,
		siteInformationRepo: d.SiteInformation,
		nodeRepo:            d.Nodes,
		breadcrumbs:         d.Breadcrumbs,
	}
}

// SetViewTrackNID sets data["viewTrackNID"] for client-side view POST when counting is enabled and node is public.
func (c *BaseController) SetViewTrackNID(data map[string]any, node entity.Node) {
	if c.viewCountingService == nil || !c.viewCountingService.IsEnabled() {
		return
	}
	if !node.IsPublic() || node.ID <= 0 {
		return
	}
	data["viewTrackNID"] = node.ID
}

// CommentsService returns the comments service for use by embedding controllers
func (c *BaseController) CommentsService() service.CommentsService {
	return c.commentService
}

// GetCurrentUser returns the current user from the session.
func (c *BaseController) GetCurrentUser(r *http.Request) (*entity.User, error) {
	userID, loggedIn := c.authService.GetUserID(r)
	if !loggedIn {
		return nil, nil
	}

	user, err := c.userService.ByID(r.Context(), userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// maxPageNumber - the last page a list serves (?page=): a larger one
// would only make the database skip ever more rows.
const maxPageNumber = 10000

// GetPageNumber reads the page number from the query string: 1 when it's
// missing, not a number, below 1 or above maxPageNumber.
func (c *BaseController) GetPageNumber(r *http.Request) int {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 || page > maxPageNumber {
		return 1
	}
	return page
}

// PathID parses the "id" path value as a positive int; ok is false if missing or invalid
// (callers should respond with RenderNotFound).
func (c *BaseController) PathID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// DBContext derives a request-scoped context bounded by the database query
// timeout (config timeouts.database_query, SetTimeouts), for handlers that
// cap how long their database work may run.
func (c *BaseController) DBContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), dbQueryTimeout)
}

// EditDeletePermissions reports whether user may edit/delete node. An anonymous user
// (nil) can do neither.
func (c *BaseController) EditDeletePermissions(ctx context.Context, node *entity.Node, user *entity.User) (canEdit, canDelete bool) {
	if user == nil {
		return false, false
	}
	return c.authService.CanEditNode(ctx, node, user), c.authService.CanDeleteNode(ctx, node, user)
}

// IsNodeVisible reports whether user may see node - the gate every detail
// page applies (AuthService.CanViewNode): its type lets them view its
// content (or they may edit it), and it is public - published and not on
// moderation - or they may see unpublished content, or it is theirs on
// moderation.
func (c *BaseController) IsNodeVisible(ctx context.Context, node *entity.Node, user *entity.User) bool {
	return c.authService.CanViewNode(ctx, node, user)
}

// RequireVisibleOrModerator renders 404 and returns false if user may not
// see node (IsNodeVisible).
// Callers should return immediately when this returns false.
func (c *BaseController) RequireVisibleOrModerator(w http.ResponseWriter, r *http.Request, node *entity.Node, user *entity.User) bool {
	if c.IsNodeVisible(r.Context(), node, user) {
		return true
	}
	c.RenderNotFound(w, r)
	return false
}

// IsViewVisible mirrors IsNodeVisible for entity.ViewDisplayDef: enabled
// (Status == 1), or the user is an admin or moderator previewing a
// disabled View's page (hidden from everyone else).
func (c *BaseController) IsViewVisible(ctx context.Context, view *entity.ViewDisplayDef, user *entity.User) bool {
	if view.Status == 1 {
		return true
	}
	return c.authService.HasPermission(ctx, user, service.PermViewUnpublished)
}

// mayAdministerContent - whether r's user may administer content (the
// node settings of the edit forms: publishing options, the front page and
// sticky flags).
func (c *BaseController) mayAdministerContent(r *http.Request) bool {
	user, _ := c.GetCurrentUser(r)
	return user != nil && c.authService.HasPermission(r.Context(), user, service.PermAdministerContent)
}

// GetCommonData - the data every page shares (GetCommonDataWithUser) for
// r's signed-in user.
func (c *BaseController) GetCommonData(r *http.Request) map[string]any {
	user, _ := c.GetCurrentUser(r)
	return c.GetCommonDataWithUser(r, user)
}

// GetCommonDataWithUser builds the shared header/sidebar data; user is the
// already-loaded user (or nil), so GetCurrentUser isn't called again.
func (c *BaseController) GetCommonDataWithUser(r *http.Request, user *entity.User) map[string]any {
	sidebarData := c.sidebarDataFor(r.Context())

	lastComments := sidebarData.LatestComments

	ctx := r.Context()
	// canEditNodeSettings - "administer content" (node settings in forms,
	// the moderator bar, "staff" blocks); isAdmin - the admin bar.
	canEditNodeSettings := c.authService.HasPermission(ctx, user, service.PermAdministerContent)
	isAdmin := c.authService.HasPermission(ctx, user, service.PermAccessAdmin)

	pendingPeopleCount := 0
	if c.authService.HasPermission(ctx, user, service.PermAdministerUsers) {
		if n, err := c.userService.CountWithStatus(r.Context(), entity.UserStatusPending); err == nil {
			pendingPeopleCount = n
		} else {
			service.LogFallback(c.logger, err, "counting users awaiting approval (none shown)")
		}
	}

	csrfToken := csrf.Token(c.store, r)

	// siteLogoURI/siteName - data for the "logo" plugin block
	// (entity.PluginBlockLogo, blocks/logo.html) and the themes' headers,
	// resolved per request rather than cached so a newly uploaded logo
	// shows immediately: the uploaded logo, else the request's theme's
	// (service.SiteLogoFor); empty - neither, the block renders nothing.
	siteLogoURI := ""
	siteName := ""
	siteSlogan := ""
	if c.siteInformationRepo != nil {
		if site, err := c.siteInformationRepo.Get(ctx); err == nil {
			// In the page's language (entity.TranslatedText).
			lang := i18n.FromContext(ctx)
			siteName = entity.TranslatedText(lang, entity.TextSiteName, entity.SiteTextRef, site.SiteName)
			siteSlogan = entity.TranslatedText(lang, entity.TextSiteSlogan, entity.SiteTextRef, site.Slogan)
			siteLogoURI = service.SiteLogoFor(paths.ThemeFor(ctx), site.Logo.URI)
		} else {
			service.LogFallback(c.logger, err, "loading site information (no name, slogan or logo)")
		}
	}
	// adminMenu - the admin toolbar's sections for this user.
	var adminMenu []entity.AdminMenuSection
	if isAdmin {
		adminMenu = service.VisibleAdminMenu(func(permission string) bool {
			return c.authService.HasPermission(ctx, user, permission)
		})
		service.CountAdminMenu(ctx, adminMenu, c.logger)
	}
	// creatableMaterials - the "Create content" menu: what user may create.
	var creatableMaterials []entity.CreatableMaterialLink
	if user != nil {
		creatableMaterials = c.authService.CreatableNodeTypes(ctx, user)
		for _, fn := range createLinkHooks {
			creatableMaterials = append(creatableMaterials, fn(ctx, user, c.authService)...)
		}
	}

	maintenanceBanner := false
	var maintenanceMessageHTML template.HTML
	if enabled, msg := middleware.MaintenancePayloadFromRequest(r); enabled && user != nil && c.authService.CanUseSiteInMaintenanceMode(r.Context(), user) {
		maintenanceBanner = true
		if msg == "" {
			msg = "<p>" + html.EscapeString(i18n.T(r.Context(), "The site is under maintenance.")) + "</p>"
		}
		maintenanceMessageHTML = template.HTML(msg)
	}
	maintenanceShowCompactStrip := maintenanceBanner && !isAdmin

	data := map[string]any{
		"user":                user,
		"currentUser":         user, // Alias for consistency with comment templates
		"canEditNodeSettings": canEditNodeSettings,
		// currentRoles - the visitor's roles for block visibility
		// (service.PlacementVisible).
		"currentRoles":                entity.UserRoleNames(user),
		"isAdmin":                     isAdmin,
		"pendingPeopleCount":          pendingPeopleCount,
		"lastComments":                lastComments,
		"CSRFToken":                   csrfToken,
		"siteLogoURI":                 siteLogoURI,
		"siteName":                    siteName,
		"siteSlogan":                  siteSlogan,
		"creatableMaterials":          creatableMaterials,
		"adminMenu":                   adminMenu,
		"maintenanceBanner":           maintenanceBanner,
		"maintenanceMessageHTML":      maintenanceMessageHTML,
		"maintenanceShowCompactStrip": maintenanceShowCompactStrip,
		// menuTreesAnnotated - every cached menu tree (CacheService.SidebarData.
		// MenuTrees) re-annotated with Active/ActiveTrail for the current path;
		// the cached trees are shared and must not be mutated (see
		// service.AnnotateActiveMenuTree). region_blocks.html looks a tree up by
		// Placement.Menu.MachineName.
		"menuTreesAnnotated": annotateMenuTrees(r.Context(), sidebarData.MenuTrees, r.URL.Path),
		// blockPlacements/isHomepage - raw published block placements per region
		// (CacheService.SidebarData.BlockPlacements) plus the homepage flag.
		// Per-placement visibility is filtered in region_blocks.html, which has
		// the page's .item (and its content type) available.
		"blockPlacements": sidebarData.BlockPlacements,
		// viewBlockContent - one pre-rendered HTML fragment per View block
		// placement, keyed by Placement.ID (see CacheService.SidebarData.
		// ViewBlockContent), rendered in the request's time zone.
		"viewBlockContent": c.viewBlockContentFor(r, sidebarData),
		// carouselPlacements - html_start loads the Swiper stylesheet
		// when one of these is visible (hasVisibleCarouselBlock).
		"carouselPlacements": sidebarData.CarouselPlacements,
		"isHomepage":         r.URL.Path == "/" || r.URL.Path == "",
		// siteJSONLD - the front page's schema.org markup: the site and
		// who runs it (SetSiteJSONLD).
		"siteJSONLD": frontPageJSONLD(r),
		// currentPath - the raw request path, for "Pages" visibility matching in
		// region_blocks.html (see service.MatchesVisibilityPages).
		"currentPath": r.URL.Path,
	}
	// adminSection/adminItem - the admin menu's section and item of the
	// page: the open section and the marked item of the admin's sidebar,
	// the page's breadcrumbs.
	if section, item, ok := entity.AdminMenuCurrent(adminMenu, r.URL.Path); ok {
		data["adminSection"] = section
		data["adminItem"] = item
	}
	// The modules' own data of every page (AddPageData), the admin's
	// pages too (AdminPageData): e.g. the number on the user menu's
	// toggle (AddUserMenuCount).
	for _, add := range pageDataHooks {
		add(r, data)
	}
	if buttons := oauthButtons(r); len(buttons) > 0 {
		data["oauth_providers"] = buttons
	}
	// ?blocks_demo=1 - region demo mode (region_blocks.html's blockDemoMode)
	// on any public page, admins only. Blocks limited by content type or path
	// show only on pages where they actually render, so /admin/config/blocks/demo
	// (the homepage) doesn't show them.
	// themePreview - the theme this request is previewed in
	// (middleware.ThemePreviewMiddleware): the admin bar's notice.
	if dir, ok := paths.PreviewTheme(ctx); ok {
		name := dir
		if info, err := themes.LoadInfoFor(dir); err == nil && info.Name != "" {
			name = info.Name
		}
		data["themePreview"] = map[string]string{"Dir": dir, "Name": name}
	}
	if isAdmin && r.URL.Query().Get("blocks_demo") == "1" {
		data["blockDemoMode"] = true
		data["regionLabels"] = TranslatedRegionLabels(r.Context(), RequestThemeRegions(r.Context(), c.logger))
	}
	return data
}

// annotateMenuTrees re-annotates every cached menu tree (see
// CacheService.SidebarData.MenuTrees) in the page's language
// (service.LocalizeMenuTree) with Active/ActiveTrail for the current
// path. There are only a few menus, so no pre-filtering.
func annotateMenuTrees(ctx context.Context, trees map[string][]service.MenuTreeItem, currentPath string) map[string][]service.MenuTreeItem {
	// In the page's language (service.LocalizeMenuTree): the current path
	// with its prefix, as the links then are.
	lang := i18n.FromContext(ctx)
	currentPath = entity.LocalPath(lang, currentPath)
	out := make(map[string][]service.MenuTreeItem, len(trees))
	for machineName, tree := range trees {
		out[machineName] = service.AnnotateActiveMenuTree(service.LocalizeMenuTree(ctx, tree, lang), currentPath)
	}
	return out
}

// RenderOrFail renders tmpl with data to w. On error it logs the failure,
// writes a 500 via httputil.TemplateExecutionError and returns false;
// callers should return immediately since a response has been written.
func (c *BaseController) RenderOrFail(w http.ResponseWriter, r *http.Request, tmpl string, data map[string]any) bool {
	if err := c.templateService.Render(w, tmpl, data, r, nil); err != nil {
		c.logger.Error("error rendering template", "template", tmpl, "error", err)
		httputil.TemplateExecutionError(w, err)
		return false
	}
	return true
}

// RenderNotFound sends HTTP 404 and renders the site not-found HTML page.
func (c *BaseController) RenderNotFound(w http.ResponseWriter, r *http.Request) {
	if renderConfiguredErrorPage(w, r, http.StatusNotFound) {
		return
	}
	c.renderStatusPage(w, r, http.StatusNotFound, "errors/not_found.html", i18n.T(r.Context(), "Page not found"),
		map[string]any{"breadcrumbs": NotFoundBreadcrumbs()})
}

// RenderLoadFailure answers a failed load of what the page shows: a
// missing record is the visitor's 404 (logged at debug level), any other
// error the server's - logged and a 500, not hidden behind a 404.
func (c *BaseController) RenderLoadFailure(w http.ResponseWriter, r *http.Request, err error, msg string, args ...any) {
	if isNotFound(err) {
		c.logger.DebugContext(r.Context(), msg, append(args, "error", err)...)
		c.RenderNotFound(w, r)
		return
	}
	// With the request's context: the line carries its request_id.
	c.logger.ErrorContext(r.Context(), msg, append(args, "error", err)...)
	httputil.InternalServerError(w)
}

// LogFailure logs an error the handler deals with itself (a redirect with
// an error, a JSON answer, a fallback): a missing record at debug level
// (the visitor's mistake or a stale link), anything else as an error.
// Nil - nothing. The services wrap their errors with what they were doing;
// the handler is where an error is logged, once.
func (c *BaseController) LogFailure(err error, msg string, args ...any) {
	if c == nil {
		return
	}
	logFailure(c.logger, err, msg, args...)
}

// logFailure - LogFailure for a handler without a BaseController.
func logFailure(logger *slog.Logger, err error, msg string, args ...any) {
	if err == nil || logger == nil {
		return
	}
	if isNotFound(err) {
		logger.Debug(msg, append(args, "error", err)...)
		return
	}
	logger.Error(msg, append(args, "error", err)...)
}

// isNotFound - err says the record is missing (ent's not-found or
// entity.ErrEntityNotFound, wrapped or not).
func isNotFound(err error) bool {
	return service.IsNotFound(err)
}

// RenderForbidden sends HTTP 403 and renders the site access-denied page:
// the configured 403 page (Site information), else errors/forbidden.html.
func (c *BaseController) RenderForbidden(w http.ResponseWriter, r *http.Request) {
	if renderConfiguredErrorPage(w, r, http.StatusForbidden) {
		return
	}
	c.renderStatusPage(w, r, http.StatusForbidden, "errors/forbidden.html", i18n.T(r.Context(), "Access denied"), nil)
}

// RenderRejected sends status and renders errors/bad_request.html: what
// the visitor sent can't be taken (a form that can't be read, has
// expired or lacks what's required); message, in the visitor's language,
// says why, with a link back to the page they came from.
func (c *BaseController) RenderRejected(w http.ResponseWriter, r *http.Request, status int, message string) {
	c.renderStatusPage(w, r, status, "errors/bad_request.html", i18n.T(r.Context(), "The request was not accepted"),
		map[string]any{"message": message, "back": sameSiteReferer(r)})
}

// RenderBadRequest - RenderRejected with 400.
func (c *BaseController) RenderBadRequest(w http.ResponseWriter, r *http.Request, message string) {
	c.RenderRejected(w, r, http.StatusBadRequest, message)
}

// RenderBadForm - the submitted form couldn't be read (a broken or cut
// body).
func (c *BaseController) RenderBadForm(w http.ResponseWriter, r *http.Request) {
	c.RenderBadRequest(w, r, i18n.T(r.Context(), "The form could not be read. Reload the page and send it again."))
}

// RenderExpiredForm - the form's CSRF token doesn't match the session
// (the page was open too long or the session ended).
func (c *BaseController) RenderExpiredForm(w http.ResponseWriter, r *http.Request) {
	c.RenderRejected(w, r, http.StatusForbidden, ExpiredFormMessage(r))
}

// ExpiredFormMessage - what a visitor whose form has expired is told.
func ExpiredFormMessage(r *http.Request) string {
	return i18n.T(r.Context(), "The form has expired. Reload the page and send it again.")
}

// RenderTooLarge sends HTTP 413 and renders errors/too_large.html: the
// request (a form with its files) is past the size the site reads.
func (c *BaseController) RenderTooLarge(w http.ResponseWriter, r *http.Request) {
	c.renderStatusPage(w, r, http.StatusRequestEntityTooLarge, "errors/too_large.html", i18n.T(r.Context(), "The request is too large"),
		map[string]any{"back": sameSiteReferer(r)})
}

// renderStatusPage renders the error page tmpl with status: a service
// page (no sidebar) titled seoTitle in the browser, with extra's data. It
// renders into a buffer first, so a template error is a 500, not half a
// page under the status.
func (c *BaseController) renderStatusPage(w http.ResponseWriter, r *http.Request, status int, tmpl, seoTitle string, extra map[string]any) {
	data := c.GetCommonData(r)
	data["title"] = ""
	data["pageSEO"] = PageSEOForList(seoTitle, r.URL.Path)
	data["servicePage"] = true
	maps.Copy(data, extra)
	var buf bytes.Buffer
	if err := c.templateService.Render(&buf, tmpl, data, r, nil); err != nil {
		c.logger.Error("error rendering an error page", "template", tmpl, "error", err)
		httputil.TemplateExecutionError(w, err)
		return
	}
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// sameSiteReferer - the page the request came from, as a path, if it is
// this site's ("" otherwise).
func sameSiteReferer(r *http.Request) string {
	u, err := url.Parse(r.Referer())
	if err != nil || u.Host != r.Host {
		return ""
	}
	return service.SafeRedirectPath(u.RequestURI())
}

// CreateMaterialsHub renders GET /create: list of material types the user may create.
func (c *BaseController) CreateMaterialsHub(w http.ResponseWriter, r *http.Request) {
	data := c.GetCommonData(r)
	pageTitle := createMaterialsHubTitle(r.Context())
	data["title"] = pageTitle
	data["pageSEO"] = c.EditorPageSEO(r, pageTitle)
	c.setPathTrail(data, r.URL.Path, pageTitle)
	c.RenderOrFail(w, r, "create/materials_hub.html", data)
}

// sidebarDataFor - the blocks' cache's data for ctx (none without a cache,
// e.g. in a handler's test).
func (c *BaseController) sidebarDataFor(ctx context.Context) service.SidebarData {
	if c.cacheService == nil {
		return service.SidebarData{}
	}
	return c.cacheService.SidebarDataFor(ctx)
}

// viewBlockContentFor - the View blocks' content for r's time zone
// (service.CacheService.ViewBlockContentFor).
func (c *BaseController) viewBlockContentFor(r *http.Request, data service.SidebarData) map[int]template.HTML {
	if c.cacheService == nil {
		return data.ViewBlockContent
	}
	return c.cacheService.ViewBlockContentFor(r.Context(), sitetime.FromContext(r.Context()))
}

// withRequestContext adds to a fragment's template data what a page gets
// from the request (GetCommonData): the time zone, the language and the
// theme the dates, the texts and the theme's markup follow; data itself is
// returned.
func withRequestContext(ctx context.Context, data map[string]any) map[string]any {
	data[service.RequestLocationKey] = sitetime.FromContext(ctx)
	data[service.RequestLanguageKey] = i18n.FromContext(ctx)
	data[service.RequestThemeKey] = service.RequestTheme(ctx)
	return data
}

// siteJSONLD - the front page's schema.org markup (SEOService.SiteJSONLD,
// wiring); nil - none.
var siteJSONLD func(ctx context.Context) template.JS

// SetSiteJSONLD gives the front page its schema.org markup (wiring).
func SetSiteJSONLD(fn func(ctx context.Context) template.JS) { siteJSONLD = fn }

// frontPageJSONLD - the site's markup on the front page only.
func frontPageJSONLD(r *http.Request) template.JS {
	if siteJSONLD == nil || (r.URL.Path != "/" && r.URL.Path != "") {
		return ""
	}
	return siteJSONLD(r.Context())
}
