package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/sociallogin"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/domain/twofactor"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/viewquery"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	oauthinfra "github.com/kosatka-cms/kosatka/internal/infrastructure/oauth"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/interfaces/admin"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/internal/lib/dbwatch"
	"github.com/kosatka-cms/kosatka/internal/lib/debounce"
	"github.com/kosatka-cms/kosatka/internal/lib/htmlimages"
	"github.com/kosatka-cms/kosatka/internal/modules"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
	"github.com/kosatka-cms/kosatka/translations"
)

// imageStack bundles the pieces used to store, style, and resize uploaded images.
type imageStack struct {
	// caches - the derivatives of the public and the private files.
	caches            derivativeCaches
	imageStyleService *imagestyle.StyleService
	// privateStyleService - the private images' derivatives (served by
	// interfaces.PrivateFilesController).
	privateStyleService *imagestyle.StyleService
	contentProcessor    *htmlimages.ContentProcessor
}

func newImageStack(client *generated.Client) imageStack {
	store := paths.PublicStore()
	storageRepo := imagestyle.NewStorageRepository(store)

	// Admin-editable image styles: DB-backed, adapted to
	// pkg/imagestyle.ImageStyleRepository's no-context interface.
	styleRepo := database.NewImageStyleRepositoryAdapter(database.NewImageStyleRepository(client))
	processor := imagestyle.NewImagingProcessor()

	imageStyleService := imagestyle.NewStyleService(
		styleRepo,
		storageRepo,
		processor,
	)

	privateRepo := imagestyle.NewStorageRepository(paths.PrivateStore())
	privateStyleService := imagestyle.NewStyleService(styleRepo, privateRepo, processor)
	// A style cropping at the focal point finds it in the files' rows.
	fileRepo := database.NewFileRepository(client.File)
	imageStyleService.SetFocalPoints(database.FileFocalPoints{Files: fileRepo, Scheme: entity.PublicScheme})
	privateStyleService.SetFocalPoints(database.FileFocalPoints{Files: fileRepo, Scheme: entity.PrivateScheme})

	resizeService := imagestyle.NewResizeService(processor, store, paths.PublicFilesURL(), paths.PublicStoreURL())
	contentProcessor := htmlimages.NewContentProcessor(resizeService)
	// A text's image opens in the lightbox as its large copy (the
	// lightbox's image style), not the original.
	contentProcessor.SetLightbox(func(src string, width, height int) (string, int, int, bool) {
		p := imageurl.LightboxPicture(entity.File{URI: src, Width: width, Height: height})
		return p.Src, p.Width, p.Height, p.Src != "" && p.Src != src
	})

	return imageStack{
		caches:              derivativeCaches{Repositories: imagestyle.Repositories{storageRepo, privateRepo}, texts: contentProcessor},
		imageStyleService:   imageStyleService,
		privateStyleService: privateStyleService,
		contentProcessor:    contentProcessor,
	}
}

// derivativeCaches - the derivatives of the public and the private files;
// clearing all of them forgets the texts the content processor rewrote
// too (they link to the resize previews).
type derivativeCaches struct {
	imagestyle.Repositories
	texts *htmlimages.ContentProcessor
}

// ClearAllGeneratedImages clears every derivative and the rewritten texts.
func (c derivativeCaches) ClearAllGeneratedImages(ctx context.Context) error {
	defer c.texts.Forget()
	return c.Repositories.ClearAllGeneratedImages(ctx)
}

// newMailService - the site's email sender: the server config's way of
// sending when it sets one - a module's transport from the environment
// (MailTransportField.Env), else its SMTP server - otherwise the one
// saved in the admin (Configuration → Mail).
func newMailService(cfg *config.Config, drv *sql.Driver, baseLogger *slog.Logger) *service.MailService {
	configured := entity.MailSettings{
		FromAddress:  cfg.Email.FromAddress,
		SMTPHost:     cfg.Email.SMTPHost,
		SMTPPort:     cfg.Email.SMTPPort,
		SMTPUser:     cfg.Email.SMTPUser,
		SMTPPassword: cfg.Email.SMTPPassword,
	}
	if name, values, ok := service.ConfiguredMailTransport(os.Getenv); ok {
		configured.Transport = name
		configured.TransportSettings = map[string]map[string]string{name: values}
	} else if configured.SMTPHost != "" {
		configured.Transport = entity.MailTransportSMTP
	}
	mailService := service.NewMailService(database.NewMailSettingsRepository(drv.DB()), configured, baseLogger, config.Seconds(cfg.Timeouts.HTTPRequest))
	mailService.SetTemplates(database.NewMailTemplateRepository(drv.DB()))
	mailService.SetLog(database.NewMailLogRepository(drv.DB()))
	mailService.SetTransportEnv(service.MailTransportEnv(os.Getenv))
	if err := mailService.Reload(context.Background()); err != nil {
		baseLogger.Error("error loading the mail settings", "error", err)
	}
	st := mailService.Status()
	if st.Missing != "" {
		baseLogger.Warn("the mail transport saved in the admin isn't in this binary: add its module to the site's binary (app module add), or choose another on the Mail page", "transport", st.Missing)
	}
	if st.Source == service.MailSourceNone || !st.Enabled {
		baseLogger.Warn("email off or not configured: transactional mail disabled (password reset, verification, templated mail)", "enabled", st.Enabled, "source", st.Source)
	} else {
		baseLogger.Info("email", "transport", st.Transport, "source", st.Source)
	}
	return mailService
}

// backgroundServices are settings-backed services that cache their state in memory
// and need an initial RefreshCache before serving traffic.
type backgroundServices struct {
	redirect    service.RedirectService
	blockedBot  service.BlockedBotService
	blockedIP   service.BlockedIPService
	maintenance service.MaintenanceService
}

func newBackgroundServices(drv *sql.Driver, baseLogger *slog.Logger) backgroundServices {
	redirectService := service.NewRedirectService(database.NewRedirectRepository(drv.DB()), database.NewUnitOfWork(database.ClientFor(drv.DB())))
	if err := redirectService.RefreshCache(context.Background()); err != nil {
		baseLogger.Warn("error loading the redirects cache", "error", err)
	}

	blockedBotService := service.NewBlockedBotService(database.NewBlockedBotRepository(drv.DB()))
	if err := blockedBotService.RefreshCache(context.Background()); err != nil {
		baseLogger.Warn("error loading the blocked bots cache", "error", err)
	}

	blockedIPService := service.NewBlockedIPService(database.NewBlockedIPRepository(drv.DB()))
	if err := blockedIPService.RefreshCache(context.Background()); err != nil {
		baseLogger.Warn("error loading the blocked IPs cache", "error", err)
	}

	maintenanceService := service.NewMaintenanceService(database.NewMaintenanceRepository(drv.DB()))
	if err := maintenanceService.RefreshCache(context.Background()); err != nil {
		baseLogger.Warn("error loading the maintenance mode settings", "error", err)
	}

	return backgroundServices{
		redirect:    redirectService,
		blockedBot:  blockedBotService,
		blockedIP:   blockedIPService,
		maintenance: maintenanceService,
	}
}

// buildHandler builds the site: every repository, service and controller,
// the modules, the routes on mux and the middleware around them. Before it
// returns, the process-wide settings are loaded from the database
// (bootSiteSettings…) and the cache has had its first refresh. Its
// background loops (the cache's refresh, the scheduler) run until ctx
// ends; the jobs it returns tells when they have returned.
func buildHandler(ctx context.Context, mux *http.ServeMux, drv *sql.Driver, cfg *config.Config, baseLogger *slog.Logger, observer *middleware.HTTPObserver, watch *dbwatch.Watch) (http.Handler, backgroundJobs) {
	s := &wiring{ctx: ctx, mux: mux, drv: drv, cfg: cfg, baseLogger: baseLogger, observer: observer, dbWatch: watch}
	s.store = newSessionStore(cfg, baseLogger)

	// The modules register their hooks below; none of an earlier wiring
	// stays.
	interfaces.ResetModuleHooks()
	// A module's View criteria apply while it is on.
	views.SetModuleEnabled(service.ModuleEnabled)

	s.client = generated.NewClient(generated.Driver(drv))

	s.wireData()
	s.wirePresentation()
	s.wireAccess()
	s.wireModules()
	s.wireServices()
	s.wireModuleLifecycle()
	s.wireControllers()

	jobs := backgroundJobs{s.cacheService.Done(), s.scheduler.Done()}
	if watch != nil {
		jobs = append(jobs, watch.Done())
	}
	return s.withMiddleware(), jobs
}

// backgroundJobs - the background loops' Done channels.
type backgroundJobs []<-chan struct{}

// Wait - whether every loop returned before ctx ended.
func (b backgroundJobs) Wait(ctx context.Context) bool {
	for _, done := range b {
		select {
		case <-done:
		case <-ctx.Done():
			return false
		}
	}
	return true
}

func (s *wiring) withMiddleware() http.Handler {
	return withMiddleware(s.mux, middlewareDeps{
		Config:        s.cfg,
		Store:         s.store,
		Background:    s.bg,
		Auth:          s.authService,
		Users:         s.userRepo,
		Site:          s.siteInformationRepo,
		UserTimezones: s.userTimezones,
		TooLarge:      http.HandlerFunc(s.indexController.RenderTooLarge),
		Logger:        s.baseLogger,
		TwoFactor:     s.twoFactorService,
		Ping:          s.drv.DB().PingContext,
		DatabaseDown:  s.dbWatch.Down,
		Observer:      s.observer,
	})
}

// wiring - what buildHandler builds, stage by stage (wire…, in that
// order): what a later stage takes from an earlier one is kept here;
// what one stage uses alone stays its local.
type wiring struct {
	// buildHandler
	ctx        context.Context
	mux        *http.ServeMux
	drv        *sql.Driver
	cfg        *config.Config
	baseLogger *slog.Logger
	// observer - the requests' metrics and log lines (nil - none).
	observer *middleware.HTTPObserver
	// dbWatch - whether the database answers (nil - always taken as up).
	dbWatch *dbwatch.Watch
	store   *sessions.CookieStore
	client  *generated.Client

	// wireData
	images             imageStack
	commentsRepo       *database.CommentRepository
	nodeRepo           *database.NodeRepository
	userFieldRepo      *database.UserFieldRepository
	userRepo           *database.UserRepository
	userService        service.UserService
	nodesService       service.NodesService
	taxonomyService    service.TaxonomyService
	uow                *database.EntUnitOfWork
	contentTypeRepo    *database.ContentTypeRepository
	contentTypeService service.ContentTypeService
	// contentAccess - who may view each content type's content: the
	// auth service's checks and what the cache renders for everyone.
	contentAccess             *service.ContentAccess
	dynamicContentRepoFactory func(entity.ContentTypeDef, entity.FieldInstanceDefs) entity.DynamicContentRepository
	viewService               views.Service
	imageStyleAdminService    service.ImageStyleService
	textFormatService         service.TextFormatService
	menuService               service.MenuService
	blockService              service.BlockService
	fileRepo                  *database.FileRepository
	nodeRevisions             *service.NodeRevisionService
	commentsService           service.CommentsService
	uploadSettingsRepo        *database.UploadSettingsRepository
	accountSettingsRepo       *database.AccountSettingsRepository
	userTimezones             *middleware.UserTimezones
	fileUploadService         *files.FileUploadService
	userFilesService          *files.UserFilesService

	// wirePresentation
	templateService     service.TemplateService
	cacheService        *service.CacheService
	scheduler           *service.Scheduler
	teaserBuilder       *service.TeaserBuilder
	siteInformationRepo *database.SiteInformationRepository
	siteURL             *service.SiteURL
	moduleRepo          *database.ModuleRepository
	themeRuntime        *themeRuntime
	// interfaceStrings - the site's own interface translations.
	interfaceStrings     *service.InterfaceStrings
	integrationService   *service.IntegrationSettingsService
	dateFormatService    *service.DateFormatService
	themeSettingsService *themes.SettingsService
	breadcrumbService    *service.BreadcrumbService

	// wireAccess
	emailSender       *service.MailService
	userTokenService  *accounts.UserAuthTokenService
	accountPolicy     *accounts.AccountPolicyService
	roleRepo          *database.RoleRepository
	permissionService *service.PermissionService
	authService       service.AuthService
	twoFactorService  *twofactor.Service

	// wireModules
	viewCountingService  *service.DefaultViewCountingService
	userContentRepo      *database.UserContentRepository
	nodeDeleteService    *service.NodeDeleteService
	baseDeps             interfaces.BaseDeps
	uninstallRepo        *database.ModuleUninstallRepository
	bg                   backgroundServices
	wiredModules         []modules.Wired
	moduleUninstallSteps map[string]func(ctx context.Context) error

	// wireServices
	searchService        *service.SearchService
	seoService           *service.SEOService
	nodePaths            *service.NodePathService
	captchaService       *service.CaptchaService
	commentSettingsRepo  entity.CommentSettingsRepository
	oauthRegistry        *oauthinfra.Registry
	oauthAuthService     *sociallogin.Service
	oauthSettingsService *sociallogin.SettingsService
	adminBulkService     *service.DefaultAdminBulkService
	userDeletionService  *service.UserDeletionService
	userFieldService     *accounts.UserFieldService
	registration         *accounts.RegistrationService
	passwordReset        *accounts.PasswordResetService
	dashboardRepo        *database.DashboardRepository

	// wireModuleLifecycle
	installModule   func(ctx context.Context, module string) error
	moduleUninstall *service.ModuleUninstall

	// wireControllers
	indexController *interfaces.IndexController
}

// wireData - the repositories and the domain services over them:
// content, users, taxonomy, content types, Views, menus, blocks, files,
// revisions, comments, text formats and image styles.
func (s *wiring) wireData() {
	s.images = newImageStack(s.client)

	s.commentsRepo = database.NewCommentRepository(s.client)
	s.nodeRepo = database.NewNodeRepository(s.client)

	s.userFieldRepo = database.NewUserFieldRepository(s.drv.DB())
	s.userRepo = database.NewUserRepository(s.client.User, s.userFieldRepo)
	s.userService = service.NewUserService(s.userRepo, s.baseLogger)
	taxonomyRepo := database.NewTaxonomyRepository(s.client)

	s.nodesService = service.NewNodesService(s.nodeRepo, s.baseLogger)
	s.taxonomyService = service.NewTaxonomyService(
		taxonomyRepo,
		s.baseLogger,
	)
	service.RegisterTagCloudBlock(s.taxonomyService)
	service.RegisterCopyrightBlock()

	s.uow = database.NewUnitOfWork(s.client)

	// Runtime-defined content types.
	s.contentTypeRepo = database.NewContentTypeRepository(s.client)
	s.contentTypeService = service.NewContentTypeService(
		s.contentTypeRepo,
		s.nodeRepo,
		s.uow,
		s.baseLogger,
	)
	s.dynamicContentRepoFactory = database.NewDynamicContentRepositoryFactory(s.client, s.contentTypeRepo, taxonomyRepo)

	// Views (configurable content lists).
	viewRepo := database.NewViewRepository(s.client)
	s.viewService = views.NewService(viewRepo, viewquery.New(s.client), s.contentTypeRepo, s.uow, func(ctx context.Context, path string) (string, error) {
		return interfaces.PathOwner(ctx, path, interfaces.PathOwnerViews)
	}, s.baseLogger)
	// What answers the core's addresses: a View page or a feed takes only
	// an address none of them answers.
	registerPathOwners(s.mux, s.nodeRepo, s.viewService, s.taxonomyService)

	imageStyleRepo := database.NewImageStyleRepository(s.client)
	s.imageStyleAdminService = service.NewImageStyleService(imageStyleRepo, s.baseLogger)
	if err := service.LoadImageStyleFormats(context.Background(), s.imageStyleAdminService); err != nil {
		s.baseLogger.Error("error loading image style formats", "error", err)
	}

	textFormatRepo := database.NewTextFormatRepository(s.client)
	s.textFormatService = service.NewTextFormatService(textFormatRepo, s.baseLogger)
	// The formats' text filters (service.ApplyTextFilters), kept in sync
	// by the service afterwards.
	if formats, err := s.textFormatService.TextFormats(context.Background()); err != nil {
		s.baseLogger.Error("error loading text format filters", "error", err)
	} else {
		service.LoadTextFormatFilters(formats)
	}

	// Menus. The public render (region_blocks.html, any menu as a block)
	// reads CacheService's MenuTrees; see service.DefaultMenuService.ResolvedTree.
	menuRepo := database.NewMenuRepository(s.client)
	s.menuService = service.NewMenuService(menuRepo, s.nodeRepo, taxonomyRepo, viewRepo, s.uow, s.baseLogger)

	// Blocks and regions. The public render reads CacheService's
	// BlockPlacements (see service.DefaultBlockService.PublishedPlacementsByRegion),
	// refreshed on the same cycle as menus.
	blockRepo := database.NewBlockRepository(s.client)
	s.blockService = service.NewBlockService(blockRepo, s.uow, s.baseLogger)

	s.fileRepo = database.NewFileRepository(s.client.File)

	// The history of the nodes of the types that keep one.
	s.nodeRevisions = service.NewNodeRevisionService(service.NodeRevisionDeps{
		Revisions: database.NewNodeRevisionRepository(s.client),
		Nodes:     s.nodeRepo,
		Types:     s.contentTypeRepo,
		Dynamic:   s.dynamicContentRepoFactory,
		Terms:     taxonomyRepo,
		Files:     s.fileRepo,
		Users:     s.userRepo,
		UoW:       s.uow,
		Logger:    s.baseLogger,
	})

	s.commentsService = service.NewCommentsService(s.commentsRepo, s.baseLogger)

	s.uploadSettingsRepo = database.NewUploadSettingsRepository(s.drv.DB())
	s.accountSettingsRepo = database.NewAccountSettingsRepository(s.drv.DB())
	accounts.RegisterMails(s.accountSettingsRepo)
	// The request's zone comes from userTimezones (middleware).
	bootUserZones(context.Background(), s.accountSettingsRepo)
	s.userTimezones = middleware.NewUserTimezones(func(ctx context.Context, id int) (string, error) {
		u, err := s.userRepo.ByID(ctx, id)
		if err != nil || u == nil {
			return "", err
		}
		return u.Timezone, nil
	})
	s.fileUploadService = files.NewFileUploadService(paths.PublicStore(), s.uploadSettingsRepo, s.accountSettingsRepo, textFormatRepo, s.baseLogger)
	s.fileUploadService.SetPrivateStore(paths.PrivateStore())
	s.userFilesService = files.NewUserFilesService(paths.PublicStore(), s.fileUploadService, s.baseLogger)
}

// wirePresentation - templates, the blocks' cache and what its first
// refresh needs (teasers, the site's settings, theme and modules), then
// what the templates read (integrations, date formats, theme settings,
// breadcrumbs).
func (s *wiring) wirePresentation() {
	// Before cacheService, which renders View blocks via RenderNamed.
	s.templateService = newTemplateService(s.cfg, s.baseLogger)

	s.cacheService = service.NewCacheService(
		15*time.Minute,
		10,
		s.commentsService,
		s.menuService,
		s.blockService,
		s.viewService,
		s.templateService,
		s.nodeRepo,
		s.contentTypeRepo,
		s.taxonomyService,
		s.baseLogger,
	)
	s.cacheService.SetUpdateTimeout(config.Seconds(s.cfg.Timeouts.CacheUpdate))
	// The handlers' timeouts (DBContext, StepBudget).
	interfaces.SetTimeouts(config.Seconds(s.cfg.Timeouts.DatabaseQuery), config.Seconds(s.cfg.Timeouts.ServerWrite))

	// Before Start: its first refresh already renders View blocks with
	// a theme template (teaser items).
	s.teaserBuilder = service.NewTeaserBuilder(s.contentTypeRepo, s.taxonomyService, s.commentsService)
	s.teaserBuilder.SetFiles(s.fileRepo)
	s.cacheService.SetTeasers(s.teaserBuilder)
	s.cacheService.SetFiles(s.fileRepo)
	s.cacheService.SetUsers(s.userRepo)

	s.siteInformationRepo = database.NewSiteInformationRepository(s.drv.DB())
	// A display renamed: the front page feed choosing it follows.
	if vs, ok := s.viewService.(*views.DefaultService); ok {
		vs.SetRenamed(s.siteInformationRepo.RenameFrontFeedView)
	}
	// siteURL - the site's canonical origin: the config's public_base_url,
	// else the one saved in Site information.
	s.siteURL = service.NewSiteURL(s.cfg.PublicBaseURL)
	// Before cacheService.Start: its first refresh renders View blocks in
	// the site's theme, zone and language.
	bootSiteSettings(context.Background(), s.siteInformationRepo, s.siteURL, s.nodeRepo.BySlug, s.baseLogger)
	s.moduleRepo = database.NewModuleRepository(s.drv.DB())
	if err := service.LoadModules(context.Background(), s.moduleRepo); err != nil {
		s.baseLogger.Error("error loading module states", "error", err)
	}
	if missing, err := missingModules(context.Background(), migrations.Runner{DB: s.drv.DB(), Dialect: sqldialect.Dialect(s.cfg.DB.Dialect())}); err != nil {
		s.baseLogger.Warn("checking for modules missing from the binary", "error", err)
	} else {
		for _, name := range missing {
			s.baseLogger.Warn("a module is "+missingModulesHint, "module", name)
		}
	}
	s.themeRuntime = newThemeRuntime(s.blockService, s.templateService, s.cacheService, s.baseLogger)
	// The site's own interface translations over the catalogs, changed
	// on the Interface translations page, and the languages downloaded
	// there from the translations catalog.
	sources, err := i18n.ParseSources(translations.Sources)
	if err != nil {
		s.baseLogger.Error("error reading the interface strings' list", "error", err)
	}
	s.interfaceStrings = service.NewInterfaceStrings(database.NewInterfaceStringRepository(s.drv.DB()), sources)
	s.interfaceStrings.SetReload(s.themeRuntime.reloadTranslations)
	s.interfaceStrings.SetThemeSources(s.themeRuntime.themeSources)
	s.interfaceStrings.SetPacks(database.NewInterfacePackRepository(s.drv.DB()), s.cfg.TranslationsCatalogURL, builtInLanguages())
	s.themeRuntime.siteStrings = s.interfaceStrings.Entries
	s.themeRuntime.packStrings = s.interfaceStrings.PackEntries
	s.themeRuntime.loadTranslations()
	service.SetThemeTranslators(s.themeRuntime.previewTranslator)
	bootJSKeys(s.baseLogger)
	// Before Start: the cache renders for everyone, as for a guest. The
	// permissions come later (wireAccess); a guest's check needs none.
	s.contentAccess = &service.ContentAccess{
		Types:     s.contentTypeService.ContentTypes,
		ItemRoles: s.nodeRepo.AnyWithViewRoles,
		HasPermission: func(ctx context.Context, user *entity.User, key string) bool {
			return s.permissionService != nil && s.permissionService.HasPermission(ctx, user, key)
		},
	}
	// A type changed: its access read again, the cached blocks rendered
	// again (a type closed now leaves them).
	if ct, ok := s.contentTypeService.(*service.DefaultContentTypeService); ok {
		ct.SetChanged(func() {
			s.contentAccess.Invalidate()
			s.cacheService.ForceUpdate()
		})
	}
	guest := func(ctx context.Context) (entity.ContentVisitor, error) {
		return s.contentAccess.Visitor(ctx, nil)
	}
	s.cacheService.SetGuestVisitor(guest)
	interfaces.SetGuestVisitor(guest)
	// What texts and templates embed ([[block:…]], [[view:…]]); the
	// modules add their kinds.
	service.RegisterEmbedKind(service.BlockEmbed(s.blockService, s.templateService))
	service.RegisterEmbedKind(s.cacheService.ViewEmbed())
	s.cacheService.SetPaused(s.dbWatch.Down)
	if err := s.cacheService.Start(s.ctx); err != nil {
		s.baseLogger.Error("failed to start cache service", "error", err)
		panic(err)
	}
	// integrationService - analytics counters and custom code snippets for the layout.
	s.integrationService = service.NewIntegrationSettingsService(database.NewIntegrationSettingsRepository(s.drv.DB()), s.baseLogger)
	s.dateFormatService = service.NewDateFormatService(database.NewDateFormatRepository(s.drv.DB()), s.baseLogger)
	// themeSettingsService - the themes' settings (theme.yaml) and their values.
	s.themeSettingsService = themes.NewSettingsService(database.NewThemeSettingsRepository(s.drv.DB()), s.baseLogger)
	if ts, ok := s.templateService.(*service.DefaultTemplateService); ok {
		ts.SetContentProcessor(s.images.contentProcessor)
		ts.SetIntegrations(s.integrationService)
		ts.SetDateFormats(s.dateFormatService)
		ts.SetThemeSettings(s.themeSettingsService)
	}

	breadcrumbRuleRepo := database.NewBreadcrumbRuleRepository(s.client)
	s.breadcrumbService = service.NewBreadcrumbService(func() string { return entity.NewSystemSettings().Domain }, breadcrumbRuleRepo, func() map[string][]service.MenuTreeItem {
		return s.cacheService.GetSidebarData().MenuTrees
	}, s.taxonomyService, s.contentTypeService, s.baseLogger)
	s.breadcrumbService.SetUnitOfWork(s.uow)
}

// wireAccess - mail, accounts' tokens and policy, roles and their
// permissions, sign-in and two-factor.
func (s *wiring) wireAccess() {
	s.emailSender = newMailService(s.cfg, s.drv, s.baseLogger)

	userAuthTokenRepo := database.NewUserAuthTokenRepository(s.drv.DB())
	s.userTokenService = accounts.NewUserAuthTokenService(userAuthTokenRepo)
	s.accountPolicy = accounts.NewAccountPolicyService(s.accountSettingsRepo)
	s.accountPolicy.OnUserZonesChanged(s.cacheService.ForceUpdate)

	s.roleRepo = database.NewRoleRepository(s.client.Role)
	// permissionService - roles' permissions.
	s.permissionService = service.NewPermissionService(s.roleRepo, s.contentTypeRepo, s.textFormatService, s.baseLogger)
	s.authService = service.NewAuthService(s.userService, *s.store, s.accountPolicy)
	// Two-factor sign-in: the second step of every way in, the roles
	// that need it (the account settings), the site's name in the app.
	s.twoFactorService = twofactor.NewService(database.NewTwoFactorRepository(s.drv.DB()), s.accountPolicy, func(ctx context.Context) string {
		if site, err := s.siteInformationRepo.Get(ctx); err == nil && site != nil {
			return site.SiteName
		}
		return ""
	})
	// Required for the base role, two-factor sign-in is everyone's.
	s.twoFactorService.SetBaseRole(func(ctx context.Context) (int, error) {
		role, err := s.roleRepo.ByName(ctx, entity.RoleUser)
		if err != nil {
			return 0, err
		}
		return role.ID, nil
	})

	if a, ok := s.authService.(*service.DefaultAuthService); ok {
		a.SetPermissions(s.permissionService)
		a.SetSecondFactor(s.twoFactorService)
		a.SetContentAccess(s.contentAccess)
	}
}

// wireModules - view counting, the node-delete path, what every
// controller's base needs, then the modules (internal/modules): each
// builds its own services, controllers and hooks from the core's
// (modules.Wire); route registration, the node dispatcher, the 404 path
// and the uninstall page take what they give back.
func (s *wiring) wireModules() {
	statisticsSettingsRepo := database.NewStatisticsSettingsRepository(s.drv.DB())
	s.viewCountingService = service.NewViewCountingService(statisticsSettingsRepo, s.nodeRepo, database.NewNodeCounterRepository(s.drv.DB()))
	if err := s.viewCountingService.RefreshCache(context.Background()); err != nil {
		s.baseLogger.Warn("error loading the statistics settings", "error", err)
	}

	// contentChanged - content saved or deleted (a node's, a comment's:
	// every write through the client, OnContentChange; the deletions'
	// own paths call it): the sidebar/View-block cache would otherwise
	// show it as it was until its next refresh. Debounced, so a burst of
	// changes costs one refresh, at most 10 s late in a steady stream.
	contentChanged := debounce.New(2*time.Second, func() {
		s.cacheService.ForceUpdate()
	}).WithMaxWait(10 * time.Second)
	database.OnContentChange(s.client, contentChanged.Trigger)
	s.userContentRepo = database.NewUserContentRepository(s.drv.DB())
	// nodeDeleteService - the single node-delete path for every type
	// (admin bulk, user-with-content deletion, per-node delete buttons);
	// also removes the node's no-longer-referenced files.
	s.nodeDeleteService = service.NewNodeDeleteService(s.nodeRepo, database.NewNodeDeleteRepository(s.drv.DB()), s.userContentRepo, s.fileUploadService, contentChanged.Trigger, s.baseLogger)
	// Deleted content goes to the trash first (a content type's).
	s.nodeDeleteService.SetTrash(s.nodeRepo, s.cfg.Content.TrashDays)
	// What every controller's base needs (interfaces.BaseDeps).
	s.baseDeps = interfaces.BaseDeps{
		Auth:            s.authService,
		Templates:       s.templateService,
		Users:           s.userService,
		Comments:        s.commentsService,
		Cache:           s.cacheService,
		ViewCounting:    s.viewCountingService,
		Store:           s.store,
		Logger:          s.baseLogger,
		SiteInformation: s.siteInformationRepo,
		Nodes:           s.nodeRepo,
		Breadcrumbs:     s.breadcrumbService,
	}
	s.uninstallRepo = database.NewModuleUninstallRepository(s.drv.DB())
	// The redirects, blocks and maintenance mode (the middleware's; the
	// modules get the redirects).
	s.bg = newBackgroundServices(s.drv, s.baseLogger)
	s.wiredModules = modules.Wire(modulekit.Deps{
		Base:            s.baseDeps,
		DB:              s.drv.DB(),
		Logger:          s.baseLogger,
		Auth:            s.authService,
		Templates:       s.templateService,
		SiteInformation: s.siteInformationRepo,
		RefreshBlocks:   s.cacheService.ForceUpdate,
		Nodes:           s.nodesService,
		NodeRepo:        s.nodeRepo,
		NodeDeleter:     s.nodeDeleteService,
		ContentTypes:    s.contentTypeService,
		Taxonomy:        s.taxonomyService,
		Teasers:         s.teaserBuilder,
		ImageStyles:     s.imageStyleAdminService,
		Comments:        s.commentsService,
		FileUpload:      s.fileUploadService,
		Files:           s.fileRepo,
		Users:           s.userRepo,
		Mail:            s.emailSender,
		UploadSettings:  s.uploadSettingsRepo,
		Vocabularies:    s.uninstallRepo,
		Redirects:       s.bg.redirect,
		Roles:           s.permissionService,
	})
	// The first refresh of the blocks' cache ran before the modules
	// registered their View criteria and plugin blocks' data: View blocks
	// sorted or showing by a module's criterion are built again with them.
	s.cacheService.ForceUpdate()
	s.moduleUninstallSteps = map[string]func(ctx context.Context) error{}
	for _, m := range s.wiredModules {
		if m.Uninstall != nil {
			s.moduleUninstallSteps[m.Name] = m.Uninstall
		}
	}
}

// wireServices - search, SEO, node paths, CAPTCHA, social login, bulk
// actions, deleting users, user fields and the accounts' services.
func (s *wiring) wireServices() {
	searchRepo := database.NewSearchRepository(s.client)
	s.searchService = service.NewSearchService(searchRepo, database.NewSearchConfigRepository(s.drv.DB()), func() map[string][]service.MenuTreeItem {
		return s.cacheService.GetSidebarData().MenuTrees
	}, s.baseLogger)
	s.searchService.SetUnitOfWork(s.uow)
	// Searched by default: every content type with a page of its own (not
	// a field set, not one redirecting elsewhere) and the modules' types.
	s.searchService.SetDefaultTypes(func(ctx context.Context) (map[string]string, error) {
		defs, err := s.contentTypeService.ContentTypes(ctx)
		if err != nil {
			return nil, err
		}
		out := map[string]string{}
		for nodeType, label := range entity.ModuleNodeTypeLabels() {
			out[nodeType] = i18n.T(ctx, label)
		}
		for _, d := range defs {
			if d.RedirectField == "" {
				out[d.MachineName] = d.Label
			}
		}
		return out, nil
	})
	seoRepo := database.NewSEORepository(s.drv.DB())
	s.seoService = service.NewSEOService(seoRepo, seoRepo, s.taxonomyService, func(ctx context.Context) string {
		if site, err := s.siteInformationRepo.Get(ctx); err == nil && site != nil {
			return entity.TranslatedText(i18n.FromContext(ctx), entity.TextSiteName, entity.SiteTextRef, site.SiteName)
		}
		return ""
	}, s.baseLogger)
	s.seoService.SetUnitOfWork(s.uow)
	s.seoService.SetFiles(s.fileRepo.ByID)
	s.seoService.SetDateFormats(s.dateFormatService)
	s.seoService.SetSite(s.siteInformationRepo.Get)
	interfaces.SetSiteJSONLD(s.seoService.SiteJSONLD)
	// nodePaths - node addresses by their types' patterns (the content
	// forms, the URL aliases' rebuild).
	s.nodePaths = service.NewNodePathService(s.taxonomyService, s.nodeRepo)
	s.captchaService = newCaptchaService(s.cfg, s.drv, s.baseLogger)
	interfaces.SetCaptcha(s.captchaService)

	// OAuth callbacks go to the site URL (localhost without one); a URL
	// saved in Site information rebuilds them.
	oauthBaseURL := func(u string) string {
		if u == "" {
			return fmt.Sprintf("http://localhost:%d", s.cfg.Port)
		}
		return u
	}
	oauthRepo := database.NewUserOAuthIdentityRepository(s.client)
	s.oauthRegistry = oauthinfra.NewRegistry(oauthBaseURL(entity.NewSystemSettings().Domain), s.cfg.OAuth)
	s.siteURL.OnChange(func(u string) { s.oauthRegistry.SetBaseURL(oauthBaseURL(u)) })
	s.oauthAuthService = sociallogin.NewService(
		s.oauthRegistry,
		oauthRepo,
		s.userService,
		s.roleRepo,
		s.authService,
		s.accountSettingsRepo,
		s.siteInformationRepo,
		s.emailSender,
		s.baseLogger,
	)
	// oauthSettingsService - social login keys from the admin (People →
	// Social login), the config's while none are saved there.
	s.oauthSettingsService = sociallogin.NewSettingsService(database.NewOAuthSettingsRepository(s.drv.DB()), s.oauthRegistry, oauthinfra.ConfigCredentials(s.cfg.OAuth))
	if err := s.oauthSettingsService.Reload(context.Background()); err != nil {
		s.baseLogger.Error("error loading the social login settings", "error", err)
	}
	interfaces.SetOAuthProviders(s.oauthAuthService.EnabledProviders)

	s.adminBulkService = service.NewAdminBulkService(
		s.nodeRepo,
		s.nodeDeleteService,
		s.userRepo,
		s.baseLogger,
	)
	s.adminBulkService.SetComments(s.commentsService)
	s.adminBulkService.SetRevisions(s.nodeRevisions)
	// The modules change nodes' publishing as the bulk actions do.
	interfaces.SetNodeTracker(s.nodeRevisions.Track)

	s.userDeletionService = service.NewUserDeletionService(
		s.userContentRepo,
		s.userRepo,
		s.adminBulkService.DeleteNode,
		s.fileUploadService,
		s.baseLogger,
	)
	s.userDeletionService.SetComments(s.commentsService)
	for _, m := range s.wiredModules {
		if m.UserContent != nil {
			s.userDeletionService.AddModuleContent(m.Name, *m.UserContent)
		}
	}

	s.userFieldService = accounts.NewUserFieldService(s.userFieldRepo)
	s.oauthAuthService.SetUserFields(s.userFieldService)
	// The accounts' services, shared by the users' pages and the admin.
	s.registration = accounts.NewRegistrationService(accounts.RegistrationDeps{Users: s.userService, Roles: s.roleRepo,
		Site: s.siteInformationRepo, Account: s.accountSettingsRepo, Fields: s.userFieldService, Tokens: s.userTokenService,
		Mail: s.emailSender, UoW: s.uow, Logger: s.baseLogger.With("component", "registration")})
	// A social network's sign-up whose address the site is to confirm.
	s.oauthAuthService.SetRegistration(s.registration)
	s.passwordReset = accounts.NewPasswordResetService(accounts.PasswordResetDeps{Users: s.userService, Tokens: s.userTokenService,
		Site: s.siteInformationRepo, Account: s.accountSettingsRepo, Mail: s.emailSender, UoW: s.uow, Logger: s.baseLogger.With("component", "password_reset")})

	// The Overview's figures; the content waiting on moderation counted by
	// its menu item.
	s.dashboardRepo = database.NewDashboardRepository(s.drv.DB())
	service.RegisterAdminMenuCount("/admin/moderation", s.dashboardRepo.OnModeration)

	// The checks of new comments (Configuration → Comments); the waiting
	// ones counted by the comments' menu item.
	s.commentSettingsRepo = database.NewCommentSettingsRepository(s.drv.DB())
	// The forms' bot trap (comments, registration, web forms), signing
	// with the session secret; without one (outside production) with a
	// random key of the process, as the sessions.
	trapKey := s.cfg.Session.SecretKey
	if trapKey == "" {
		trapKey = generateSessionSecret()
	}
	service.SetBotTrap(service.NewBotTrap(trapKey, s.commentSettingsRepo))
	// The comment form's CAPTCHA: for authors without a published comment.
	service.SetCommentCaptcha(func(ctx context.Context, userID int) string {
		key := s.captchaService.SiteKey(entity.CaptchaFormComment)
		if key == "" {
			return ""
		}
		published := entity.CommentPublished
		n, err := s.commentsRepo.TotalCount(ctx, entity.CommentsFilter{CreatedBy: &userID, Status: &published})
		if err != nil {
			service.LogFallback(s.baseLogger, err, "counting a user's comments (the CAPTCHA shown)", "user_id", userID)
			return key
		}
		if n > 0 {
			return ""
		}
		return key
	})
	// In order: the refusals first, then the holds - the most telling
	// reason first.
	s.commentsService.AddCheck(service.DuplicateCheck(s.commentSettingsRepo, s.commentsRepo, time.Now))
	s.commentsService.AddCheck(service.StopWordsCheck(s.commentSettingsRepo))
	s.commentsService.AddCheck(service.LinksCheck(s.commentSettingsRepo))
	s.commentsService.AddCheck(service.NewAuthorCheck(s.commentSettingsRepo, s.commentsRepo, s.userService, s.authService))
	service.RegisterAdminMenuCount("/admin/content/comment", func(ctx context.Context) (int, error) {
		pending := entity.CommentPending
		return s.commentsRepo.TotalCount(ctx, entity.CommentsFilter{Status: &pending})
	}, i18n.N("Comments waiting for approval"))
}

// wireModuleLifecycle - installing and uninstalling modules from the
// Modules page, the background jobs and the status report.
func (s *wiring) wireModuleLifecycle() {
	// A module enabled for the first time on the Modules page is installed
	// (moduleInstaller), its labels in the site's language.
	moduleRunner := migrations.Runner{DB: s.drv.DB(), Dialect: sqldialect.Dialect(s.cfg.DB.Dialect())}
	installSchema := moduleInstaller(moduleRunner, func(ctx context.Context) (string, error) {
		site, err := s.siteInformationRepo.Get(ctx)
		if err != nil || site == nil {
			return "", err
		}
		return site.DefaultLanguage, nil
	}, s.baseLogger)
	// A module's install step may turn its text filter on for formats
	// (manifestkit.EnableTextFilter): the formats' filters are read again.
	s.installModule = func(ctx context.Context, module string) error {
		if err := installSchema(ctx, module); err != nil {
			return err
		}
		formats, err := s.textFormatService.TextFormats(ctx)
		if err != nil {
			return fmt.Errorf("reloading the text formats' filters: %w", err)
		}
		service.LoadTextFormatFilters(formats)
		return nil
	}
	// Uninstalling a disabled module (Modules page): its placements and
	// permissions, its uninstall step, tables and recorded migrations.
	s.moduleUninstall = &service.ModuleUninstall{
		Installed:   moduleRunner.Installed,
		DropSchema:  moduleRunner.Uninstall,
		Modules:     s.moduleRepo,
		Permissions: s.permissionService,
		Store:       s.uninstallRepo,
		DeleteNode:  s.nodeDeleteService.Delete,
		FileColumns: moduleFileColumns,
		Files:       s.userContentRepo,
		Storage:     s.fileUploadService,
		Steps:       s.moduleUninstallSteps,
		Logger:      s.baseLogger,
	}

	// The background jobs (after the modules are wired: their jobs too):
	// the scheduled publishing, every minute; emptying the trash of what
	// stayed there long enough, the spam, the old views by day and the old
	// mail log, every hour.
	s.scheduler = service.NewScheduler(s.baseLogger)
	s.scheduler.Add(service.NewScheduledPublishing(s.nodeRepo, s.nodeRepo, s.nodeRevisions).Job())
	s.scheduler.Add(s.nodeDeleteService.TrashJob())
	s.scheduler.Add(service.SpamPurgeJob(s.commentSettingsRepo, s.commentsRepo, s.commentsService, time.Now))
	s.scheduler.Add(s.viewCountingService.ViewDaysPurgeJob(time.Now))
	s.scheduler.Add(s.emailSender.LogPurgeJob(time.Now))
	s.scheduler.SetPaused(s.dbWatch.Down)
	s.scheduler.Start(s.ctx)

	setupStatusReport(statusDeps{
		Runner:    moduleRunner,
		DB:        s.drv.DB(),
		Config:    s.cfg,
		Mail:      s.emailSender,
		Cache:     s.cacheService,
		Scheduler: s.scheduler,
		Schedule:  s.nodeRepo,
		Modules:   s.wiredModules,
	})
}

// wireControllers - the controllers and their routes on mux.
func (s *wiring) wireControllers() {
	nodeSearchController := interfaces.NewNodeSearchController(s.nodeRepo, s.baseLogger)
	// The users' files manager; the file usage - where a user's file is
	// linked from, checked before it is deleted or renamed.
	userFilesController := interfaces.NewUserFilesController(s.userFilesService, s.store, s.baseLogger, s.authService,
		files.NewUserFileUsageService(database.NewUserFileUsageRepository(s.drv.DB())), s.textFormatService)
	commentController := interfaces.NewCommentController(s.baseDeps)

	nodeController := interfaces.NewNodeController(s.baseDeps, interfaces.NodeDeps{
		NodeService: s.nodesService,
		Revisions:   s.nodeRevisions,
		Deleter:     s.nodeDeleteService,
	})
	searchController := interfaces.NewSearchController(interfaces.NewBaseController(s.baseDeps), s.searchService)
	styleguideController := interfaces.NewStyleguideController(interfaces.NewBaseController(s.baseDeps), s.textFormatService)
	taxonomyController := interfaces.NewTaxonomyController(s.baseDeps, interfaces.TaxonomyDeps{
		TaxonomyService: s.taxonomyService,
		ViewService:     s.viewService,
		Teasers:         s.teaserBuilder,
		SEO:             s.seoService,
	})

	sitemapController := interfaces.NewSitemapController(s.siteInformationRepo, s.nodeRepo, s.viewService, s.menuService, s.baseLogger)
	// The media library: its page, and the content forms' picker.
	media := &service.MediaLibrary{Repo: database.NewMediaRepository(s.drv.DB()), Files: s.fileRepo,
		Cleaner: s.userContentRepo, Remover: s.fileUploadService, Copies: s.fileUploadService, Uploads: s.uploadSettingsRepo}
	dynamicContentController := interfaces.NewDynamicContentController(s.baseDeps, interfaces.DynamicContentDeps{
		Media:              media,
		ContentTypeService: s.contentTypeService,
		TaxonomyService:    s.taxonomyService,
		NodeService:        s.nodesService,
		FileUploadService:  *s.fileUploadService,
		FileRepo:           s.fileRepo,
		UnitOfWork:         s.uow,
		DynamicRepoFactory: s.dynamicContentRepoFactory,
		ViewService:        s.viewService,
		TextFormatService:  s.textFormatService,
		ContentTypeRepo:    s.contentTypeRepo,
		NodeDeleter:        s.nodeDeleteService,
		Redirects:          s.bg.redirect,
		NodePaths:          s.nodePaths,
		Teasers:            s.teaserBuilder,
		SEO:                s.seoService,
		Revisions:          s.nodeRevisions,
		Roles: func(ctx context.Context) (entity.Roles, error) {
			return s.permissionService.Roles(ctx)
		},
		AccessChanged: func() {
			s.contentAccess.Invalidate()
			s.cacheService.ForceUpdate()
		},
		PreviewLinks:    &service.PreviewLinks{Repo: database.NewPreviewLinkRepository(s.client)},
		CommentSettings: s.commentSettingsRepo,
	})
	// The modules copy content through its form's controller (CopyNode).
	interfaces.SetNodeCopier(dynamicContentController.CopyNode)
	rssController := interfaces.NewRssController(s.siteInformationRepo, s.baseLogger, dynamicContentController)
	adminController := admin.NewController(s.baseDeps, admin.Deps{
		Cache:               s.cacheService,
		Registration:        s.registration,
		CommentsRepo:        s.commentsRepo,
		Dashboard:           s.dashboardRepo,
		TwoFactor:           s.twoFactorService,
		Revisions:           s.nodeRevisions,
		Dynamic:             s.dynamicContentRepoFactory,
		UserRepo:            s.userRepo,
		RoleRepo:            s.roleRepo,
		AdminBulkService:    s.adminBulkService,
		NodeDeleter:         s.nodeDeleteService,
		Media:               media,
		RedirectService:     s.bg.redirect,
		PathRebuilder:       service.NewNodePathRebuilder(s.nodePaths, s.nodeRepo, s.contentTypeService, s.bg.redirect),
		BlockedBotService:   s.bg.blockedBot,
		BlockedIPService:    s.bg.blockedIP,
		MaintenanceService:  s.bg.maintenance,
		AccountSettings:     s.accountPolicy,
		ImageStorage:        s.images.caches,
		TaxonomyService:     s.taxonomyService,
		ContentTypeService:  s.contentTypeService,
		ImageStyleService:   s.imageStyleAdminService,
		UploadSettingsRepo:  s.uploadSettingsRepo,
		ViewService:         s.viewService,
		FileUploadService:   *s.fileUploadService,
		FileRepo:            s.fileRepo,
		TextFormatService:   s.textFormatService,
		MenuService:         s.menuService,
		BlockService:        s.blockService,
		UserDeletionService: s.userDeletionService,
		BreadcrumbService:   s.breadcrumbService,
		SeoService:          s.seoService,
		SearchService:       s.searchService,
		Permissions:         s.permissionService,
		Integrations:        s.integrationService,
		ThemeSettings:       s.themeSettingsService,
		DateFormats:         s.dateFormatService,
		UserFields:          s.userFieldService,
		SwitchTheme:         s.themeRuntime.apply,
		ThemeCatalog:        &themes.CatalogService{URL: s.cfg.ThemesCatalogURL},
		SiteURL:             s.siteURL,
		UserFiles:           s.userFilesService,
		CaptchaService:      s.captchaService,
		InterfaceStrings:    s.interfaceStrings,
		CommentSettings:     s.commentSettingsRepo,
		MailService:         s.emailSender,
		OAuthSettings:       s.oauthSettingsService,
		OAuthRedirects:      s.oauthRegistry,
		Modules:             s.moduleRepo,
		ModulesChanged:      s.cacheService.ForceUpdate,
		InstallModule:       s.installModule,
		ModuleUninstall:     s.moduleUninstall,
	})
	stylesController := interfaces.NewStylesController(
		s.images.imageStyleService,
		s.baseLogger,
	)
	privateFilesController := interfaces.NewPrivateFilesController(interfaces.NewBaseController(s.baseDeps), s.fileRepo,
		service.PrivateFileAccess{NodesWithFile: s.userContentRepo.NodesWithFile, NodeByID: s.nodeRepo.ByID, Auth: s.authService},
		paths.PrivateStore, s.images.privateStyleService)

	s.indexController = interfaces.NewIndexController(s.baseDeps, interfaces.IndexDeps{FrontFeed: dynamicContentController})

	userController := interfaces.NewUserController(s.baseDeps, interfaces.UserDeps{
		FileUploadService: s.fileUploadService,
		RoleRepo:          s.roleRepo,
		UserTokenService:  s.userTokenService,
		UserFields:        s.userFieldService,
		ProfileViews:      s.viewService,
		ProfileTabs:       dynamicContentController,
		UserTimezones:     s.userTimezones,
		Registration:      s.registration,
		PasswordReset:     s.passwordReset,
		TwoFactor:         s.twoFactorService,
		SocialLogin:       s.oauthAuthService,
		AccountCancel:     s.userDeletionService,
		AccountPolicy:     s.accountPolicy,
	})

	registerRoutes(s.mux, routeDeps{
		Store:                    s.store,
		BaseLogger:               s.baseLogger,
		NodeRepo:                 s.nodeRepo,
		ViewCountingService:      s.viewCountingService,
		UserRepo:                 s.userRepo,
		AuthService:              s.authService,
		UserService:              s.userService,
		TemplateService:          s.templateService,
		SocialLogin:              s.oauthAuthService,
		SearchController:         searchController,
		UserController:           userController,
		UserFilesController:      userFilesController,
		AdminController:          adminController,
		NodeSearchController:     nodeSearchController,
		EmbedsController:         interfaces.NewEmbedsController(s.baseDeps, s.textFormatService),
		DynamicContentController: dynamicContentController,
		StyleguideController:     styleguideController,
		StylesController:         stylesController,
		PrivateFilesController:   privateFilesController,
		CommentController:        commentController,
		NodeController:           nodeController,
		SitemapController:        sitemapController,
		RssController:            rssController,
		TaxonomyController:       taxonomyController,
		IndexController:          s.indexController,
		Modules:                  s.wiredModules,
	})
}

// newCaptchaService - reCAPTCHA keys and forms from the admin
// (Configuration → Spam protection), the config's keys while none are
// saved there.
func newCaptchaService(cfg *config.Config, drv *sql.Driver, logger *slog.Logger) *service.CaptchaService {
	captcha := service.NewCaptchaService(database.NewCaptchaSettingsRepository(drv.DB()), cfg.Recaptcha.SiteKey, cfg.Recaptcha.SecretKey, config.Seconds(cfg.Timeouts.HTTPRequest))
	if err := captcha.Reload(context.Background()); err != nil {
		logger.Error("error loading the CAPTCHA settings", "error", err)
	}
	return captcha
}

// moduleFileColumns - the module's columns holding file URIs
// (service.ModuleUninstall.FileColumns).
func moduleFileColumns(module string) [][2]string {
	var out [][2]string
	for _, c := range migrations.FileColumns(module) {
		out = append(out, [2]string{c.Table, c.Column})
	}
	return out
}

// middlewareDeps - what the middleware around the routes needs.
type middlewareDeps struct {
	Config        *config.Config
	Store         *sessions.CookieStore
	Background    backgroundServices
	Auth          service.AuthService
	Users         *database.UserRepository
	Site          *database.SiteInformationRepository
	UserTimezones *middleware.UserTimezones
	// TooLarge - the page for a request body over the limit.
	TooLarge http.Handler
	Logger   *slog.Logger
	// TwoFactor - who must turn two-factor sign-in on first (nil - none).
	TwoFactor *twofactor.Service
	// Ping - whether the database answers (/healthz).
	Ping func(context.Context) error
	// DatabaseDown - the database doesn't answer (dbwatch): a 503 at once.
	DatabaseDown func() bool
	// Observer - the requests' metrics and log lines (nil - none).
	Observer *middleware.HTTPObserver
}

// withMiddleware wraps the routes in the site's middleware, from the
// innermost (redirects) out.
func withMiddleware(routes http.Handler, d middlewareDeps) http.Handler {
	bg := d.Background
	// The 404s the modules log (OnNotFound): inside the redirects and the
	// blocks, which answer before the routes.
	handler := middleware.NotFoundHooksMiddleware(middleware.RouteRecorder(routes))
	handler = middleware.RedirectMiddleware(bg.redirect)(handler)
	handler = middleware.BlockedBotMiddleware(bg.blockedBot)(handler)
	handler = middleware.BlockedIPMiddleware(bg.blockedIP)(handler)
	handler = middleware.MaintenanceMiddleware(bg.maintenance, d.Auth, d.Users, d.Store, d.Site)(handler)
	handler = middleware.MaintenanceContextMiddleware(bg.maintenance)(handler)
	handler = middleware.CSRFMiddleware(d.Store, d.TooLarge)(handler)
	// Before the CSRF check, which parses form bodies (uploads included).
	handler = middleware.BodyLimitMiddleware(middleware.MaxRequestBody)(handler)
	handler = middleware.UserLastAccessMiddleware(d.Users, d.Logger)(handler)
	handler = d.UserTimezones.Middleware(handler)
	handler = middleware.ThemePreviewMiddleware(d.Store, d.Users, d.Auth)(handler)
	// A user whose role needs two-factor sign-in turns it on first:
	// inside OptionalAuth (it sees the user), cheap while no role needs it.
	// A failed check sends the user to the setup page too: the page of a
	// role that needs the second factor isn't opened without it.
	if tf := d.TwoFactor; tf != nil {
		handler = middleware.TwoFactorSetupMiddleware(func(ctx context.Context, userID int) bool {
			if len(tf.RequiredRoles(ctx)) == 0 {
				return false
			}
			user, err := d.Users.ByID(ctx, userID)
			if err != nil {
				d.Logger.Warn("checking the two-factor sign-in", "error", err, "user_id", userID)
				return true
			}
			if user == nil {
				return false
			}
			needed, err := tf.SetupNeeded(ctx, *user)
			if err != nil {
				d.Logger.Warn("checking the two-factor sign-in", "error", err, "user_id", userID)
				return true
			}
			return needed
		}, interfaces.TwoFactorPath)(handler)
	}
	// What of the content the visitor may not view, left out of every
	// node and comment query of the request; inside OptionalAuth: it sees
	// the user.
	handler = middleware.ContentAccessMiddleware(func(ctx context.Context, userID int) (entity.ContentVisitor, error) {
		var user *entity.User
		if userID != 0 {
			u, err := d.Users.ByID(ctx, userID)
			if err != nil && !entity.IsNotFound(err) {
				return entity.ContentVisitor{}, err
			}
			user = u
		}
		return d.Auth.ContentVisitor(ctx, user)
	}, d.Logger)(handler)
	// Who acts (the action log), inside OptionalAuth: it sees the user.
	handler = middleware.ActorMiddleware(handler)
	handler = middleware.OptionalAuthMiddleware(d.Store)(handler)
	handler = middleware.InactiveUserSessionMiddleware(d.Store, d.Users, d.Logger)(handler)
	// The language of the address's prefix (/en/…): every layer inside
	// sees the path without it, in that language.
	handler = middleware.LanguageMiddleware(handler)
	// Sees every cookie the layers inside set.
	handler = middleware.PageCacheMiddleware(d.Config.PageCacheMaxAge, csrf.SessionName)(handler)
	// Every layer inside reads a user by ID once a request.
	handler = middleware.UserMemoMiddleware(handler)
	// The database gone: a 503 before any layer inside needs it.
	var site func() *entity.SiteInformation
	if d.Site != nil {
		site = d.Site.Cached
	}
	handler = middleware.DatabaseDownMiddleware(d.DatabaseDown, site)(handler)
	// A panic anywhere inside: a 500 and the stack in the log.
	handler = middleware.RecoverMiddleware(d.Logger)(handler)
	// The requests' metrics and log lines see every answer of the layers
	// inside (RouteRecorder, around the routes, tells them the route);
	// each request has its id first.
	handler = middleware.RequestIDMiddleware(d.Observer.Middleware(handler))
	// /healthz answers ahead of every other layer (not counted, no id);
	// the security headers go on every response, its too.
	return middleware.SecurityHeadersMiddleware(middleware.HealthMiddleware(d.Ping)(handler))
}
