package admin

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/sociallogin"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/domain/twofactor"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

// Controller serves the admin's pages.
type Controller struct {
	*interfaces.BaseController
	// logger, cacheService, store, authService - the admin's own copies of
	// the BaseDeps the base controller keeps unexported.
	logger           *slog.Logger
	cacheService     *service.CacheService
	store            *sessions.CookieStore
	authService      service.AuthService
	nodeRepo         entity.NodesRepository
	commentsRepo     entity.CommentsRepository
	userRepo         entity.UserRepository
	roleRepo         entity.RoleRepository
	adminBulkService service.AdminBulkService
	nodeDeleter      *service.NodeDeleteService
	media            *service.MediaLibrary
	redirectService  service.RedirectService
	// pathRebuilder - the URL aliases' rebuild tab (Deps.PathRebuilder).
	pathRebuilder       *service.NodePathRebuilder
	blockedBotService   service.BlockedBotService
	blockedIPService    service.BlockedIPService
	maintenanceService  service.MaintenanceService
	viewCountingService service.ViewCountingService
	templateService     service.TemplateService
	accountSettings     *accounts.AccountPolicyService
	siteInformationRepo entity.SiteInformationRepository
	// switchTheme applies a newly chosen theme at runtime (Deps).
	switchTheme func(dir string)
	// themeCatalog - the themes' catalog (Deps; its URL "" - none).
	themeCatalog *themes.CatalogService
	// siteURL applies the site URL saved in Site information (Deps).
	siteURL *service.SiteURL
	// modules stores the built-in modules' states; modulesChanged runs
	// after a save (Deps).
	modules        entity.ModuleRepository
	modulesChanged func()
	// installModule creates a module's tables on first enabling
	// (Deps); nil - modules aren't installed from the page.
	installModule func(ctx context.Context, module string) error
	// uninstall removes a disabled module completely (nil - no uninstall).
	uninstall *service.ModuleUninstall
	// userFiles - users' files (Deps).
	userFiles *files.UserFilesService
	// dashboard - the Overview's figures; revisions - its recent changes
	// (Deps; nil - none).
	dashboard entity.DashboardRepository
	revisions *service.NodeRevisionService
	// dynamic - Deps.Dynamic.
	dynamic func(entity.ContentTypeDef, entity.FieldInstanceDefs) entity.DynamicContentRepository
	// twoFactor - two-factor sign-in (Deps; nil - none).
	twoFactor *twofactor.Service
	// captchaService - reCAPTCHA settings (Deps).
	captchaService *service.CaptchaService
	// commentSettings - what is done with suspicious comments (Deps).
	commentSettings entity.CommentSettingsRepository
	// interfaceStrings - the interface's strings and the site's own
	// translations of them (Deps).
	interfaceStrings *service.InterfaceStrings
	// mailService - mail settings (Deps).
	mailService *service.MailService
	// oauthSettings/oauthRedirects - social login settings (Deps).
	oauthSettings      *sociallogin.SettingsService
	oauthRedirects     oauthRedirectURLs
	imageStorage       imagestyle.DerivativeCaches
	taxonomyService    service.TaxonomyService
	contentTypeService service.ContentTypeService
	imageStyleService  service.ImageStyleService
	uploadSettingsRepo entity.UploadSettingsRepository
	viewService        views.Service
	fileUploadService  files.FileUploadService
	fileRepo           entity.FileRepository
	textFormatService  service.TextFormatService
	// permissions - the role × permission matrix (Deps).
	permissions *service.PermissionService
	// integrations - counters and site codes (Deps).
	integrations *service.IntegrationSettingsService
	// themeSettings - the themes' settings (Appearance).
	themeSettings *themes.SettingsService
	// dateFormats - the date format catalog (admin/date_formats.go).
	dateFormats *service.DateFormatService
	// userFields - admin-defined profile fields (Deps).
	userFields *accounts.UserFieldService
	// registration - the accounts an administrator creates and approves
	// (Deps).
	registration        *accounts.RegistrationService
	menuService         service.MenuService
	blockService        service.BlockService
	userDeletionService *service.UserDeletionService
	breadcrumbService   *service.BreadcrumbService
	seoService          *service.SEOService
	searchService       *service.SearchService
}

// Deps - what NewController needs besides the BaseDeps.
type Deps struct {
	CommentsRepo entity.CommentsRepository
	// Dashboard, Revisions - the Overview's figures and recent changes.
	Dashboard entity.DashboardRepository
	Revisions *service.NodeRevisionService
	// Dynamic - a content type's items with their fields (the SEO page's
	// sample of the schema.org markup).
	Dynamic func(entity.ContentTypeDef, entity.FieldInstanceDefs) entity.DynamicContentRepository
	// TwoFactor - two-factor sign-in: the roles that need it, a reset.
	TwoFactor        *twofactor.Service
	UserRepo         entity.UserRepository
	RoleRepo         entity.RoleRepository
	AdminBulkService service.AdminBulkService
	// NodeDeleter - the trash: its page, restoring, deleting for good.
	NodeDeleter *service.NodeDeleteService
	// Media - the media library: the files, their uses, deleting.
	Media               *service.MediaLibrary
	RedirectService     service.RedirectService
	PathRebuilder       *service.NodePathRebuilder
	BlockedBotService   service.BlockedBotService
	BlockedIPService    service.BlockedIPService
	MaintenanceService  service.MaintenanceService
	AccountSettings     *accounts.AccountPolicyService
	ImageStorage        imagestyle.DerivativeCaches
	TaxonomyService     service.TaxonomyService
	ContentTypeService  service.ContentTypeService
	ImageStyleService   service.ImageStyleService
	UploadSettingsRepo  entity.UploadSettingsRepository
	ViewService         views.Service
	FileUploadService   files.FileUploadService
	FileRepo            entity.FileRepository
	TextFormatService   service.TextFormatService
	MenuService         service.MenuService
	BlockService        service.BlockService
	UserDeletionService *service.UserDeletionService
	BreadcrumbService   *service.BreadcrumbService
	SeoService          *service.SEOService
	SearchService       *service.SearchService
	Permissions         *service.PermissionService
	Integrations        *service.IntegrationSettingsService
	ThemeSettings       *themes.SettingsService
	DateFormats         *service.DateFormatService
	UserFields          *accounts.UserFieldService
	Registration        *accounts.RegistrationService
	SwitchTheme         func(dir string)
	ThemeCatalog        *themes.CatalogService
	SiteURL             *service.SiteURL
	UserFiles           *files.UserFilesService
	CaptchaService      *service.CaptchaService
	CommentSettings     entity.CommentSettingsRepository
	MailService         *service.MailService
	OAuthSettings       *sociallogin.SettingsService
	OAuthRedirects      oauthRedirectURLs
	Modules             entity.ModuleRepository
	ModulesChanged      func()
	InstallModule       func(ctx context.Context, module string) error
	ModuleUninstall     *service.ModuleUninstall
	// Cache - the blocks' cache: refreshed after a change, cleared on the
	// Performance page.
	Cache *service.CacheService
	// InterfaceStrings - the interface's strings and the site's own
	// translations of them (Interface translations).
	InterfaceStrings *service.InterfaceStrings
}

// NewController creates the admin's Controller on deps and d.
func NewController(deps interfaces.BaseDeps, d Deps) *Controller {
	base := interfaces.NewBaseController(deps)
	logger := deps.Logger.With("component", "admin.handler")
	base.SetLogger(logger)
	return &Controller{
		BaseController:      base,
		logger:              logger,
		cacheService:        d.Cache,
		store:               deps.Store,
		authService:         deps.Auth,
		nodeRepo:            deps.Nodes,
		commentsRepo:        d.CommentsRepo,
		dashboard:           d.Dashboard,
		revisions:           d.Revisions,
		dynamic:             d.Dynamic,
		twoFactor:           d.TwoFactor,
		userRepo:            d.UserRepo,
		roleRepo:            d.RoleRepo,
		adminBulkService:    d.AdminBulkService,
		nodeDeleter:         d.NodeDeleter,
		media:               d.Media,
		redirectService:     d.RedirectService,
		pathRebuilder:       d.PathRebuilder,
		blockedBotService:   d.BlockedBotService,
		blockedIPService:    d.BlockedIPService,
		maintenanceService:  d.MaintenanceService,
		viewCountingService: deps.ViewCounting,
		templateService:     deps.Templates,
		accountSettings:     d.AccountSettings,
		siteInformationRepo: deps.SiteInformation,
		imageStorage:        d.ImageStorage,
		taxonomyService:     d.TaxonomyService,
		contentTypeService:  d.ContentTypeService,
		imageStyleService:   d.ImageStyleService,
		uploadSettingsRepo:  d.UploadSettingsRepo,
		viewService:         d.ViewService,
		fileUploadService:   d.FileUploadService,
		fileRepo:            d.FileRepo,
		textFormatService:   d.TextFormatService,
		menuService:         d.MenuService,
		blockService:        d.BlockService,
		userDeletionService: d.UserDeletionService,
		breadcrumbService:   d.BreadcrumbService,
		seoService:          d.SeoService,
		searchService:       d.SearchService,
		permissions:         d.Permissions,
		integrations:        d.Integrations,
		themeSettings:       d.ThemeSettings,
		dateFormats:         d.DateFormats,
		userFields:          d.UserFields,
		registration:        d.Registration,
		switchTheme:         d.SwitchTheme,
		themeCatalog:        d.ThemeCatalog,
		siteURL:             d.SiteURL,
		userFiles:           d.UserFiles,
		captchaService:      d.CaptchaService,
		commentSettings:     d.CommentSettings,
		interfaceStrings:    d.InterfaceStrings,
		mailService:         d.MailService,
		oauthSettings:       d.OAuthSettings,
		oauthRedirects:      d.OAuthRedirects,
		modules:             d.Modules,
		modulesChanged:      d.ModulesChanged,
		installModule:       d.InstallModule,
		uninstall:           d.ModuleUninstall,
	}
}

func (c *Controller) getAdminData(r *http.Request) map[string]any {
	return c.AdminPageData(r)
}
