package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/sociallogin"
	router "github.com/kosatka-cms/kosatka/internal/http"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/interfaces/admin"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/internal/modules"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// routeDeps holds every controller/service that route registration needs;
// it only keeps registerRoutes's signature manageable.
type routeDeps struct {
	Store                    *sessions.CookieStore
	BaseLogger               *slog.Logger
	NodeRepo                 *database.NodeRepository
	ViewCountingService      service.ViewCountingService
	UserRepo                 *database.UserRepository
	AuthService              service.AuthService
	UserService              service.UserService
	TemplateService          service.TemplateService
	SocialLogin              *sociallogin.Service
	SearchController         *interfaces.SearchController
	UserController           *interfaces.UserController
	UserFilesController      *interfaces.UserFilesController
	AdminController          *admin.Controller
	NodeSearchController     *interfaces.NodeSearchController
	EmbedsController         *interfaces.EmbedsController
	StylesController         *interfaces.StylesController
	PrivateFilesController   *interfaces.PrivateFilesController
	CommentController        *interfaces.CommentController
	NodeController           *interfaces.NodeController
	SitemapController        *interfaces.SitemapController
	RssController            *interfaces.RssController
	TaxonomyController       *interfaces.TaxonomyController
	IndexController          *interfaces.IndexController
	DynamicContentController *interfaces.DynamicContentController
	StyleguideController     *interfaces.StyleguideController
	// Modules - what the modules' Wire gave back (modules.Wire).
	Modules []modules.Wired
}

// registerRoutes wires the node dispatcher/OAuth controller (which only route
// registration needs) and registers every route on mux.
func registerRoutes(mux *http.ServeMux, d routeDeps) {
	viewCountController := interfaces.NewViewCountController(d.BaseLogger.With("component", "view_count"), d.ViewCountingService)

	nodeDispatcher := interfaces.NewNodeDispatcher(d.DynamicContentController)
	// The modules' content types with pages of their own.
	for _, m := range d.Modules {
		for t, c := range m.NodeTypes {
			nodeDispatcher.RegisterType(t, c)
		}
	}
	// Site information → 404/403 pages: rendered through the node page;
	// node access denials use the same 403 page.
	nodeDispatcher.SetNodes(d.NodeRepo)
	interfaces.SetErrorPageRenderer(nodeDispatcher.RenderErrorPage)
	// The middleware's error answers: the site's pages.
	middleware.SetErrorPages(middleware.ErrorPages{
		Forbidden:   d.IndexController.RenderForbidden,
		NotFound:    d.IndexController.RenderNotFound,
		Rejected:    d.IndexController.RenderRejected,
		ExpiredForm: interfaces.ExpiredFormMessage,
	})

	notFoundHTML := func(w http.ResponseWriter, r *http.Request) {
		d.IndexController.RenderNotFound(w, r)
	}

	// dynamicListFallback runs on the 404 path only (never on a successful
	// lookup): the modules' pages (a feed, a forum section - each while its
	// module is on), View pages at /{view.URLPath}, then public taxonomy
	// term pages.
	dynamicListFallback := func(w http.ResponseWriter, r *http.Request) bool {
		for _, m := range d.Modules {
			if m.Fallback != nil && service.ModuleEnabled(m.Name) && m.Fallback(w, r) {
				return true
			}
		}
		return d.DynamicContentController.ViewPageFallback(w, r) ||
			d.TaxonomyController.TermPageFallback(w, r)
	}

	router.RegisterRoutes(mux,
		d.Store,
		nodeDispatcher,
		d.NodeRepo,
		d.UserRepo,
		d.AuthService,
		d.IndexController,
		notFoundHTML,
		dynamicListFallback,
	)

	// The live search asks as one types.
	router.RegisterSearchRoutes(router.ModuleMux(mux, entity.FeatureSearch, notFoundHTML), d.SearchController, ratelimit.NewRateLimiter(60, 20))

	oauthController := interfaces.NewOAuthController(
		d.UserController.BaseController,
		d.SocialLogin,
		d.UserService,
		d.TemplateService,
		d.Store,
		d.BaseLogger,
	)

	router.RegisterUserRoutes(
		mux,
		d.Store,
		d.UserController,
		oauthController,
		router.UserRateLimits{
			UserSearch: ratelimit.NewRateLimiter(20, 5),
			AuthForm:   ratelimit.NewRateLimiter(20, 5),
			Password:   ratelimit.NewRateLimiter(20, 5),
			// Ten tries at an account, then one a minute; three reset
			// mails to an address, then one in ten minutes.
			LoginAccount: ratelimit.NewRateLimiterEvery(time.Minute, 10),
			ResetAddress: ratelimit.NewRateLimiterEvery(10*time.Minute, 3),
		},
	)

	router.RegisterUserFilesRoutes(mux, d.Store, d.UserFilesController)

	// A page sends one view; a client opens a page a second or so.
	router.RegisterViewCountRoutes(mux, viewCountController, ratelimit.NewRateLimiter(60, 20))

	router.RegisterAdminRoutes(
		mux,
		d.Store,
		d.AdminController,
		d.DynamicContentController,
		d.StyleguideController,
		d.UserRepo,
		d.AuthService,
	)

	// The modules' routes, each on a mux answering 404 while its module
	// is off.
	adminOnly := func(h http.Handler) http.Handler {
		return middleware.AuthMiddleware(d.Store)(middleware.PermissionMiddleware(d.UserRepo, d.AuthService, service.PermAccessAdmin)(middleware.AdminSaveAuditMiddleware(h)))
	}
	for _, m := range d.Modules {
		if m.Routes != nil {
			m.Routes(modulekit.Routes{Mux: router.ModuleMux(mux, m.Name, notFoundHTML), Store: d.Store, AdminOnly: adminOnly})
		}
	}

	router.RegisterNodeSearchRoutes(
		mux,
		d.Store,
		d.NodeSearchController,
		ratelimit.NewRateLimiter(60, 20),
	)

	router.RegisterEmbedsRoutes(mux, d.Store, d.EmbedsController)
	router.RegisterStylesRoutes(mux, d.StylesController)
	router.RegisterPrivateFilesRoutes(mux, d.PrivateFilesController)

	// Comment redirect routes
	commentsRateLimiter := ratelimit.NewRateLimiter(60, 10)
	commentPostRateLimiter := ratelimit.NewRateLimiter(10, 5)
	router.RegisterCommentRoutes(
		mux,
		d.Store,
		d.CommentController,
		commentsRateLimiter,
		commentPostRateLimiter,
	)

	// Hub for choosing material type (auth required)
	authForCreateHub := middleware.AuthMiddleware(d.Store)
	mux.Handle("GET /create", authForCreateHub(http.HandlerFunc(d.IndexController.CreateMaterialsHub)))

	// Node editing routes
	router.RegisterNodeRoutes(
		mux,
		d.Store,
		d.NodeController,
		d.NodeRepo,
		d.UserRepo,
		d.AuthService,
		nodeDispatcher,
		notFoundHTML,
	)

	router.RegisterSitemapRoutes(mux, d.SitemapController)
	router.RegisterRssRoutes(mux, d.RssController)

	router.RegisterStaticRoutes(mux)

	// The tag pickers ask as one types, as the user search does.
	router.RegisterTaxonomyRoutes(mux, d.Store, d.TaxonomyController, ratelimit.NewRateLimiter(60, 10))
}
