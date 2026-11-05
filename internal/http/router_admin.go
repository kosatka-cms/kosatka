package http

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/interfaces/admin"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
)

// adminRoutes - the admin's routes on r: behind sign-in (auth) and access
// to the admin (admin: a saved settings page goes to the action log,
// AdminSaveAuditMiddleware), and for some a further permission.
type adminRoutes struct {
	r           *http.ServeMux
	auth, admin func(http.Handler) http.Handler
	userRepo    entity.UserRepository
	authService service.AuthService
}

// handle - h at pattern, for who may access the admin.
func (a adminRoutes) handle(pattern string, h http.HandlerFunc) {
	a.r.Handle(pattern, a.auth(a.admin(h)))
}

// with - h at pattern, for who also holds permission.
func (a adminRoutes) with(permission string, pattern string, h http.HandlerFunc) {
	a.r.Handle(pattern, a.auth(a.admin(middleware.PermissionMiddleware(a.userRepo, a.authService, permission)(h))))
}

// people - the accounts, their fields and the account settings
// ("administer users").
func (a adminRoutes) people(pattern string, h http.HandlerFunc) {
	a.with(service.PermAdministerUsers, pattern, h)
}

// permissions - the roles and the permission matrix ("administer
// permissions": a role holding it can grant itself any permission).
func (a adminRoutes) permissions(pattern string, h http.HandlerFunc) {
	a.with(service.PermAdministerPermissions, pattern, h)
}

// RegisterAdminRoutes registers the admin's pages: content, people,
// configuration, the content model, the site structure and the Views,
// for who may access the admin (and, for some, holds a further
// permission).
func RegisterAdminRoutes(
	r *http.ServeMux,
	store *sessions.CookieStore,
	ac *admin.Controller,
	dc *interfaces.DynamicContentController,
	sg *interfaces.StyleguideController,
	userRepo entity.UserRepository,
	authService service.AuthService,
) {
	adminAccess := middleware.PermissionMiddleware(userRepo, authService, service.PermAccessAdmin)
	a := adminRoutes{
		r:           r,
		auth:        middleware.AuthMiddleware(store),
		admin:       func(h http.Handler) http.Handler { return adminAccess(middleware.AdminSaveAuditMiddleware(h)) },
		userRepo:    userRepo,
		authService: authService,
	}
	registerAdminContentRoutes(a, ac)
	registerAdminPeopleRoutes(a, ac)
	registerAdminConfigRoutes(a, ac, sg)
	registerAdminContentModelRoutes(a, ac)
	registerAdminSiteStructureRoutes(a, ac)
	registerAdminViewsRoutes(a, ac, dc)
}

// registerAdminContentRoutes - the admin's first page, content,
// moderation, comments, the users' files and the bulk actions on them.
func registerAdminContentRoutes(a adminRoutes, ac *admin.Controller) {
	// /admin itself - the content list, the admin's first page.
	a.handle("GET /admin", ac.Dashboard)
	// Moderation - for who may moderate, not only the admin's users.
	a.r.Handle("GET /admin/moderation", a.auth(middleware.PermissionMiddleware(a.userRepo, a.authService, service.PermModerateContent)(http.HandlerFunc(ac.ModerationList))))
	a.with(service.PermAdministerUserFiles, "GET /admin/content/files", ac.UserFilesList)
	a.with(service.PermAdministerUserFiles, "GET /admin/content/files/{id}", ac.UserFilesBrowse)
	a.r.Handle("POST /admin/moderation/bulk-confirm", a.auth(middleware.PermissionMiddleware(a.userRepo, a.authService, service.PermModerateContent)(http.HandlerFunc(ac.BulkConfirmModeration))))
	a.handle("GET /admin/content", ac.ContentList)
	a.handle("POST /admin/content/bulk-confirm", ac.BulkConfirmContent)
	a.with(service.PermAdministerContent, "GET /admin/content/media", ac.MediaList)
	a.with(service.PermAdministerContent, "POST /admin/content/media/delete-unused", ac.MediaDeleteUnused)
	a.with(service.PermAdministerContent, "POST /admin/content/media/{id}", ac.MediaUpdate)
	a.with(service.PermAdministerContent, "POST /admin/content/media/{id}/delete", ac.MediaDelete)
	a.with(service.PermAdministerContent, "GET /admin/content/trash", ac.TrashList)
	a.with(service.PermAdministerContent, "POST /admin/content/trash/empty", ac.TrashEmpty)
	a.with(service.PermAdministerContent, "POST /admin/content/trash/{id}/restore", ac.TrashRestore)
	a.with(service.PermAdministerContent, "POST /admin/content/trash/{id}/delete", ac.TrashPurge)
	a.handle("GET /admin/content/comment", ac.CommentList)
	a.handle("POST /admin/content/comment/bulk-confirm", ac.BulkConfirmComment)
	a.handle("POST /admin/content/comment/{id}/{action}", ac.CommentAction)
	a.handle("POST /api/admin/bulk/content", ac.BulkExecuteContent)
	// The moderation queue's bulk actions, for who may moderate: each
	// action is checked as the single one (mayBulkContent, deletableNodes).
	a.r.Handle("POST /api/admin/bulk/moderation", a.auth(middleware.PermissionMiddleware(a.userRepo, a.authService, service.PermModerateContent)(http.HandlerFunc(ac.BulkExecuteContent))))
	a.handle("POST /api/admin/bulk/comments", ac.BulkExecuteComments)
}

// registerAdminPeopleRoutes - accounts, roles and permissions, user
// fields, the account settings.
func registerAdminPeopleRoutes(a adminRoutes, ac *admin.Controller) {
	a.people("GET /admin/people", ac.PeopleList)
	a.people("GET /admin/people/create", ac.PeopleCreateForm)
	a.people("POST /admin/people/create", ac.PeopleCreate)
	a.people("POST /admin/people/{id}/approve", ac.PeopleApprove)
	a.people("POST /admin/people/{id}/invite", ac.PeopleInviteResend)
	a.people("POST /admin/people/{id}/invite/cancel", ac.PeopleInviteCancel)
	a.people("POST /admin/people/bulk-confirm", ac.BulkConfirmPeople)
	a.permissions("GET /admin/people/permissions", ac.PermissionsPage)
	a.permissions("POST /admin/people/permissions", ac.PermissionsSave)
	a.people("GET /admin/people/fields", ac.UserFieldsPage)
	a.people("POST /admin/people/fields", ac.UserFieldCreate)
	a.people("POST /admin/people/fields/{id}/edit", ac.UserFieldUpdate)
	a.people("POST /admin/people/fields/{id}/delete", ac.UserFieldDelete)
	a.permissions("GET /admin/people/roles", ac.RolesPage)
	a.permissions("POST /admin/people/roles", ac.RoleCreate)
	a.permissions("POST /admin/people/roles/{id}/rename", ac.RoleRename)
	a.permissions("POST /admin/people/roles/{id}/delete", ac.RoleDelete)
	a.people("GET /admin/config/account", ac.AccountSettingsPage)
	a.people("POST /admin/config/account", ac.AccountSettingsPage)
	a.people("POST /admin/people/{id}/two-factor/reset", ac.TwoFactorReset)
	a.people("POST /api/admin/bulk/people", ac.BulkExecutePeople)
}

// registerAdminConfigRoutes - the site's settings: site information,
// appearance, CAPTCHA, mail, social login, modules, the status report,
// uploads, integrations, date formats, maintenance, statistics,
// performance, URL aliases, redirects, robots and IP blocking.
func registerAdminConfigRoutes(a adminRoutes, ac *admin.Controller, sg *interfaces.StyleguideController) {
	a.handle("GET /admin/config/system/site-information", ac.SiteInformationSettings)
	a.handle("POST /admin/config/system/site-information", ac.SiteInformationSettings)
	a.handle("GET /admin/config/appearance", ac.AppearancePage)
	a.handle("POST /admin/config/appearance", ac.AppearancePage)
	a.handle("POST /admin/config/appearance/settings", ac.AppearanceSettingsSave)
	a.handle("GET /admin/config/appearance/styleguide", sg.Page)
	a.handle("GET /admin/config/appearance/catalog", ac.AppearanceCatalog)
	a.handle("POST /admin/config/appearance/preview", ac.AppearancePreview)
	a.handle("POST /admin/config/appearance/preview/stop", ac.AppearancePreviewStop)
	a.handle("GET /admin/config/appearance/theme/{theme}", ac.AppearanceThemeSettings)
	a.handle("POST /admin/config/appearance/theme/{theme}", ac.AppearanceThemeSettings)
	a.handle("GET /admin/config/captcha", ac.CaptchaPage)
	a.handle("POST /admin/config/captcha", ac.CaptchaPage)
	a.handle("GET /admin/config/mail", ac.MailPage)
	a.handle("POST /admin/config/mail", ac.MailPage)
	a.handle("POST /admin/config/mail/test", ac.MailTest)
	a.handle("GET /admin/config/mail/templates", ac.MailTemplatesPage)
	a.handle("GET /admin/config/mail/templates/{key}", ac.MailTemplateEdit)
	a.handle("POST /admin/config/mail/templates/{key}", ac.MailTemplateSave)
	a.handle("POST /admin/config/mail/templates/{key}/test", ac.MailTemplateTest)
	a.handle("POST /admin/config/mail/templates/{key}/enabled", ac.MailTemplateSwitch)
	a.handle("GET /admin/config/mail/log", ac.MailLogPage)
	a.handle("GET /admin/config/oauth", ac.OAuthPage)
	a.handle("POST /admin/config/oauth", ac.OAuthPage)
	a.handle("GET /admin/config/modules", ac.ModulesPage)
	a.handle("GET /admin/reports/status", ac.StatusReportPage)
	a.handle("GET /admin/reports/popular", ac.PopularContent)
	a.handle("POST /admin/config/modules", ac.ModulesPage)
	a.handle("GET /admin/config/modules/{name}/uninstall", ac.ModuleUninstallPage)
	a.handle("POST /admin/config/modules/{name}/uninstall", ac.ModuleUninstallPage)
	a.handle("POST /admin/config/modules/{name}/delete-content", ac.ModuleContentDelete)
	// Upload policy - applies to every image upload path.
	a.handle("GET /admin/config/upload-settings", ac.UploadSettingsPage)
	a.handle("POST /admin/config/upload-settings", ac.UploadSettingsPage)
	a.handle("GET /admin/config/integrations", ac.IntegrationsPage)
	a.handle("POST /admin/config/integrations", ac.IntegrationsPage)
	a.with(service.PermFullAccess, "GET /admin/config/translations", ac.InterfaceLanguages)
	a.with(service.PermFullAccess, "POST /admin/config/translations/download", ac.InterfacePackDownload)
	a.with(service.PermFullAccess, "POST /admin/config/translations/delete", ac.InterfacePackDelete)
	a.with(service.PermFullAccess, "POST /admin/config/translations/check", ac.InterfaceCatalogCheck)
	a.with(service.PermFullAccess, "GET /admin/config/translations/strings", ac.InterfaceStringsPage)
	a.with(service.PermFullAccess, "POST /admin/config/translations/strings", ac.InterfaceStringSave)
	a.with(service.PermFullAccess, "POST /admin/config/translations/strings/reset", ac.InterfaceStringReset)
	a.handle("GET /admin/config/date-formats", ac.DateFormatList)
	a.handle("POST /admin/config/date-formats", ac.DateFormatSave)
	a.handle("GET /admin/config/date-formats/new", ac.DateFormatNew)
	a.handle("GET /admin/config/date-formats/{id}", ac.DateFormatEdit)
	a.handle("POST /admin/config/date-formats/{id}", ac.DateFormatSave)
	a.handle("POST /admin/config/date-formats/{id}/delete", ac.DateFormatDelete)
	a.handle("GET /admin/config/maintenance", ac.MaintenanceSettings)
	a.handle("POST /admin/config/maintenance", ac.MaintenanceSettings)
	a.handle("GET /admin/config/comments", ac.CommentSettingsPage)
	a.handle("POST /admin/config/comments", ac.CommentSettingsPage)
	a.handle("GET /admin/config/statistics", ac.StatisticsSettings)
	a.handle("POST /admin/config/statistics", ac.StatisticsSettings)
	a.handle("GET /admin/config/performance", ac.PerformanceSettings)
	a.handle("GET /admin/config/url-aliases", ac.URLAliasList)
	a.handle("GET /admin/config/url-aliases/patterns", ac.URLPatternList)
	a.handle("POST /admin/config/url-aliases/patterns", ac.URLPatternsSave)
	a.handle("GET /admin/config/url-aliases/rebuild", ac.URLRebuildPage)
	a.handle("POST /admin/config/url-aliases/rebuild", ac.URLRebuild)
	a.handle("GET /admin/config/redirects", ac.RedirectList)
	a.handle("POST /admin/config/redirects", ac.RedirectCreate)
	a.handle("POST /admin/config/redirects/import", ac.RedirectImport)
	a.handle("POST /admin/config/redirects/delete/{id}", ac.RedirectDelete)
	a.handle("GET /admin/config/robots", ac.BlockedBotList)
	a.handle("POST /admin/config/robots", ac.BlockedBotCreate)
	a.handle("POST /admin/config/robots/rules", ac.RobotsRulesSave)
	a.handle("POST /admin/config/robots/import", ac.BlockedBotImport)
	a.handle("POST /admin/config/robots/delete/{id}", ac.BlockedBotDelete)
	a.handle("POST /admin/config/robots/ip", ac.BlockedIPCreate)
	a.handle("POST /admin/config/robots/ip/import", ac.BlockedIPImport)
	a.handle("POST /admin/config/robots/ip/delete/{id}", ac.BlockedIPDelete)
	a.handle("POST /admin/performance/clear-thumbnails", ac.PerformanceClearThumbnails)
	a.handle("POST /admin/performance/refresh-memory-cache", ac.PerformanceRefreshMemoryCache)
}

// registerAdminContentModelRoutes - taxonomy, content types and their
// fields, image styles, text formats.
func registerAdminContentModelRoutes(a adminRoutes, ac *admin.Controller) {
	a.handle("GET /admin/config/taxonomy", ac.TaxonomyVocabularyList)
	a.handle("POST /admin/config/taxonomy", ac.TaxonomyVocabularyCreate)
	a.handle("POST /admin/config/taxonomy/reorder", ac.TaxonomyVocabulariesSortOrderUpdate)
	a.handle("GET /admin/config/taxonomy/{id}/edit", ac.TaxonomyVocabularyEdit)
	a.handle("POST /admin/config/taxonomy/{id}", ac.TaxonomyVocabularyUpdate)
	a.handle("GET /admin/config/taxonomy/{id}/delete", ac.TaxonomyVocabularyDeleteConfirm)
	a.handle("POST /admin/config/taxonomy/{id}/delete", ac.TaxonomyVocabularyDelete)
	a.handle("GET /admin/config/taxonomy/{vid}/terms", ac.TaxonomyTermList)
	a.handle("POST /admin/config/taxonomy/{vid}/terms", ac.TaxonomyTermCreate)
	a.handle("POST /admin/config/taxonomy/{vid}/terms/reorder", ac.TaxonomyTermsSortOrderUpdate)
	a.handle("GET /admin/config/taxonomy/{vid}/terms/{id}/edit", ac.TaxonomyTermEdit)
	a.handle("POST /admin/config/taxonomy/{vid}/terms/{id}", ac.TaxonomyTermUpdate)
	a.handle("GET /admin/config/taxonomy/{vid}/terms/{id}/delete", ac.TaxonomyTermDeleteConfirm)
	a.handle("POST /admin/config/taxonomy/{vid}/terms/{id}/delete", ac.TaxonomyTermDelete)
	// Content types and their fields.
	a.handle("GET /admin/config/content-types", ac.ContentTypeList)
	a.handle("POST /admin/config/content-types", ac.ContentTypeCreate)
	a.handle("GET /admin/config/field-sets", ac.FieldSetList)
	a.handle("POST /admin/config/field-sets", ac.FieldSetCreate)
	a.handle("GET /admin/config/content-types/{id}", ac.ContentTypeEdit)
	a.handle("POST /admin/config/content-types/{id}", ac.ContentTypeUpdate)
	a.handle("POST /admin/config/content-types/{id}/delete", ac.ContentTypeDelete)
	a.handle("GET /admin/config/content-types/{id}/fields", ac.ContentTypeFieldList)
	a.handle("POST /admin/config/content-types/{id}/fields", ac.ContentTypeFieldCreate)
	a.handle("POST /admin/config/content-types/{id}/fields/attach", ac.ContentTypeFieldAttach)
	a.handle("POST /admin/config/content-types/{id}/fields/reorder", ac.ContentTypeFieldsSortOrderUpdate)
	a.handle("GET /admin/config/content-types/{id}/fields/display-detail", ac.ContentTypeFieldsDetailDisplay)
	a.handle("POST /admin/config/content-types/{id}/fields/display-detail", ac.ContentTypeFieldsDetailDisplayUpdate)
	a.handle("GET /admin/config/content-types/{id}/fields/{field_id}/edit", ac.ContentTypeFieldEdit)
	a.handle("POST /admin/config/content-types/{id}/fields/{field_id}", ac.ContentTypeFieldUpdate)
	a.handle("GET /admin/config/content-types/{id}/fields/{field_id}/cardinality", ac.ContentTypeFieldCardinality)
	a.handle("POST /admin/config/content-types/{id}/fields/{field_id}/cardinality", ac.ContentTypeFieldCardinalitySave)
	a.handle("GET /admin/config/content-types/{id}/fields/{field_id}/delete", ac.ContentTypeFieldDeleteConfirm)
	a.handle("POST /admin/config/content-types/{id}/fields/{field_id}/delete", ac.ContentTypeFieldDelete)
	// Image styles - admin-editable resize presets.
	a.handle("GET /admin/config/image-styles", ac.ImageStyleList)
	a.handle("POST /admin/config/image-styles", ac.ImageStyleCreate)
	a.handle("GET /admin/config/image-styles/{id}/edit", ac.ImageStyleEdit)
	a.handle("POST /admin/config/image-styles/{id}", ac.ImageStyleUpdate)
	a.handle("GET /admin/config/image-styles/{id}/delete", ac.ImageStyleDeleteConfirm)
	a.handle("POST /admin/config/image-styles/{id}/delete", ac.ImageStyleDelete)

	a.handle("GET /admin/config/text-formats", ac.TextFormatList)
	a.handle("POST /admin/config/text-formats", ac.TextFormatCreate)
	a.handle("POST /admin/config/text-formats/reorder", ac.TextFormatsReorder)
	a.handle("GET /admin/config/text-formats/{id}/edit", ac.TextFormatEdit)
	a.handle("POST /admin/config/text-formats/{id}", ac.TextFormatUpdate)
	a.handle("GET /admin/config/text-formats/{id}/delete", ac.TextFormatDeleteConfirm)
	a.handle("POST /admin/config/text-formats/{id}/delete", ac.TextFormatDelete)
}

// registerAdminSiteStructureRoutes - breadcrumbs, SEO, search, menus and
// blocks.
func registerAdminSiteStructureRoutes(a adminRoutes, ac *admin.Controller) {
	// Breadcrumbs - "type + terms → menu link" rules.
	a.handle("GET /admin/config/breadcrumbs", ac.BreadcrumbRuleList)
	a.handle("POST /admin/config/breadcrumbs", ac.BreadcrumbRuleCreate)
	a.handle("POST /admin/config/breadcrumbs/reorder", ac.BreadcrumbRuleReorder)
	a.handle("GET /admin/config/breadcrumbs/{id}/edit", ac.BreadcrumbRuleEdit)
	a.handle("POST /admin/config/breadcrumbs/{id}", ac.BreadcrumbRuleUpdate)
	a.handle("GET /admin/config/breadcrumbs/{id}/delete", ac.BreadcrumbRuleDeleteConfirm)
	a.handle("POST /admin/config/breadcrumbs/{id}/delete", ac.BreadcrumbRuleDelete)

	// SEO - node title/description templates and term-based rules.
	a.handle("GET /admin/config/seo", ac.SEOPage)
	a.handle("POST /admin/config/seo/default", ac.SEODefaultSave)
	a.handle("GET /admin/config/seo/types/{id}/edit", ac.SEOTypeEdit)
	a.handle("POST /admin/config/seo/types/{id}", ac.SEOTypeSave)
	a.handle("POST /admin/config/seo/term-default", ac.SEOTermDefaultSave)
	a.handle("GET /admin/config/seo/vocabularies/{id}/edit", ac.SEOVocabularyEdit)
	a.handle("POST /admin/config/seo/vocabularies/{id}", ac.SEOVocabularySave)
	a.handle("POST /admin/config/seo/rules", ac.SEORuleCreate)
	a.handle("POST /admin/config/seo/rules/reorder", ac.SEORuleReorder)
	a.handle("GET /admin/config/seo/rules/{id}/edit", ac.SEORuleEdit)
	a.handle("POST /admin/config/seo/rules/{id}", ac.SEORuleUpdate)
	a.handle("GET /admin/config/seo/rules/{id}/delete", ac.SEORuleDeleteConfirm)
	a.handle("POST /admin/config/seo/rules/{id}/delete", ac.SEORuleDelete)

	// Search - which types live search covers and how results are shown.
	a.handle("GET /admin/config/search", ac.SearchConfigPage)
	a.handle("POST /admin/config/search/settings", ac.SearchSettingsSave)
	a.handle("POST /admin/config/search/types", ac.SearchTypeAdd)
	a.handle("POST /admin/config/search/types/reorder", ac.SearchTypesReorder)
	a.handle("GET /admin/config/search/types/{type}/edit", ac.SearchTypeEdit)
	a.handle("POST /admin/config/search/types/{type}", ac.SearchTypeSave)
	a.handle("POST /admin/config/search/types/{type}/delete", ac.SearchTypeDelete)

	// Menus; for public rendering see menuRepo in internal/app/wiring.go.
	a.handle("GET /admin/config/menus", ac.MenuList)
	a.handle("POST /admin/config/menus", ac.MenuCreate)
	a.handle("GET /admin/config/menus/{id}/edit", ac.MenuEdit)
	a.handle("POST /admin/config/menus/{id}", ac.MenuUpdate)
	a.handle("GET /admin/config/menus/{id}/delete", ac.MenuDeleteConfirm)
	a.handle("POST /admin/config/menus/{id}/delete", ac.MenuDelete)
	a.handle("GET /admin/config/menus/{id}/links", ac.MenuLinksList)
	a.handle("POST /admin/config/menus/{id}/links", ac.MenuLinkCreate)
	a.handle("POST /admin/config/menus/{id}/links/reorder", ac.MenuLinksReorder)
	a.handle("GET /admin/config/menus/{id}/links/{link_id}/edit", ac.MenuLinkEdit)
	a.handle("POST /admin/config/menus/{id}/links/{link_id}", ac.MenuLinkUpdate)
	a.handle("GET /admin/config/menus/{id}/links/{link_id}/delete", ac.MenuLinkDeleteConfirm)
	a.handle("POST /admin/config/menus/{id}/links/{link_id}/delete", ac.MenuLinkDelete)

	// Blocks and regions; for public rendering see blockRepo in
	// internal/app/wiring.go.
	a.handle("GET /admin/config/blocks", ac.BlockList)
	a.handle("POST /admin/config/blocks", ac.BlockCreate)
	a.handle("POST /admin/config/blocks/reorder", ac.BlocksReorder)
	a.handle("GET /admin/config/blocks/demo", ac.BlockRegionDemo)
	a.handle("POST /admin/config/blocks/plugin", ac.PluginBlockCreate)
	a.handle("POST /admin/config/blocks/view", ac.ViewBlockCreate)
	a.handle("POST /admin/config/blocks/menu", ac.MenuBlockCreate)
	a.handle("GET /admin/config/blocks/menu/{id}/edit", ac.MenuPlacementEdit)
	// POST .../menu/{id}/edit, not .../menu/{id}: "menu/{id}" and
	// "{id}/delete" below both match "blocks/menu/delete", and ServeMux
	// panics at startup on such ambiguity. GET and POST share the URL.
	a.handle("POST /admin/config/blocks/menu/{id}/edit", ac.MenuPlacementUpdate)
	// Same .../{id}/edit shape as the menu placement, same reason.
	a.handle("GET /admin/config/blocks/plugin/{id}/edit", ac.PluginPlacementEdit)
	a.handle("POST /admin/config/blocks/plugin/{id}/edit", ac.PluginPlacementUpdate)
	// Same shape, same reason.
	a.handle("GET /admin/config/blocks/view/{id}/edit", ac.ViewPlacementEdit)
	a.handle("POST /admin/config/blocks/view/{id}/edit", ac.ViewPlacementUpdate)
	a.handle("POST /admin/config/blocks/placement/{id}/delete", ac.PluginPlacementDelete)
	a.handle("GET /admin/config/blocks/{id}/edit", ac.BlockEdit)
	a.handle("POST /admin/config/blocks/{id}", ac.BlockUpdate)
	a.handle("GET /admin/config/blocks/{id}/delete", ac.BlockDeleteConfirm)
	a.handle("POST /admin/config/blocks/{id}/delete", ac.BlockDelete)
}

// registerAdminViewsRoutes - Views: their displays, filters, sort
// criteria and fields, overriding, publishing.
func registerAdminViewsRoutes(a adminRoutes, ac *admin.Controller, dc *interfaces.DynamicContentController) {
	a.handle("GET /admin/config/views", ac.ViewList)
	a.handle("POST /admin/config/views", ac.ViewCreate)
	a.handle("GET /admin/config/views/{id}", ac.ViewEdit)
	a.handle("POST /admin/config/views/{id}", ac.ViewUpdate)
	a.handle("GET /admin/config/views/{id}/delete", ac.ViewDeleteConfirm)
	a.handle("POST /admin/config/views/{id}/delete", ac.ViewDelete)
	// {id} is the View container's id: deletes the whole group (Default +
	// every display). A separate top-level path on purpose -
	// "views/group/{id}/delete" would conflict with
	// "views/{id}/filters/{filter_id}" and ServeMux panics at startup.
	a.handle("GET /admin/config/view-groups/{id}/delete", ac.ViewGroupDeleteConfirm)
	a.handle("POST /admin/config/view-groups/{id}/delete", ac.ViewGroupDelete)
	a.handle("POST /admin/config/views/{id}/filters", ac.ViewFilterCreate)
	a.handle("POST /admin/config/views/{id}/filters/{filter_id}", ac.ViewFilterUpdate)
	a.handle("GET /admin/config/views/{id}/filters/{filter_id}/delete", ac.ViewFilterDeleteConfirm)
	a.handle("POST /admin/config/views/{id}/filters/{filter_id}/delete", ac.ViewFilterDelete)
	a.handle("POST /admin/config/views/{id}/filters/reorder", ac.ViewFiltersReorder)
	a.handle("POST /admin/config/views/{id}/sorts", ac.ViewSortCreate)
	a.handle("POST /admin/config/views/{id}/sorts/{sort_id}", ac.ViewSortUpdate)
	a.handle("POST /admin/config/views/{id}/sorts/{sort_id}/delete", ac.ViewSortDelete)
	a.handle("POST /admin/config/views/{id}/sorts/reorder", ac.ViewSortsReorder)
	a.handle("POST /admin/config/views/{id}/fields", ac.ViewFieldCreate)
	a.handle("POST /admin/config/views/{id}/fields/{field_id}", ac.ViewFieldUpdate)
	a.handle("GET /admin/config/views/{id}/fields/{field_id}/delete", ac.ViewFieldDeleteConfirm)
	a.handle("POST /admin/config/views/{id}/fields/{field_id}/delete", ac.ViewFieldDelete)
	a.handle("POST /admin/config/views/{id}/fields/reorder", ac.ViewFieldsReorder)
	a.handle("POST /admin/config/views/{id}/preview", dc.ViewPreview)
	// Displays: clone, add, per-section override/revert.
	a.handle("POST /admin/config/views/{id}/clone", ac.ViewClone)
	a.handle("POST /admin/config/views/{id}/add-display", ac.ViewAddDisplay)
	a.handle("POST /admin/config/views/{id}/settings/override", ac.ViewSettingsOverride)
	a.handle("POST /admin/config/views/{id}/settings/revert", ac.ViewSettingsRevert)
	a.handle("POST /admin/config/views/{id}/filters/override", ac.ViewFiltersOverride)
	a.handle("POST /admin/config/views/{id}/filters/revert", ac.ViewFiltersRevert)
	a.handle("POST /admin/config/views/{id}/sorts/override", ac.ViewSortOverride)
	a.handle("POST /admin/config/views/{id}/sorts/revert", ac.ViewSortRevert)
	a.handle("POST /admin/config/views/{id}/fields/override", ac.ViewFieldsOverride)
	a.handle("POST /admin/config/views/{id}/fields/revert", ac.ViewFieldsRevert)
	// Publishing and discarding a display's draft changes.
	a.handle("POST /admin/config/views/{id}/publish", ac.ViewPublishChanges)
	a.handle("POST /admin/config/views/{id}/discard", ac.ViewDiscardChanges)
	a.handle("GET /admin/config/views/{id}/publish-status", ac.ViewPublishStatus)
}
