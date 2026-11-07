package interfaces

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The base controller's parts a module's controller (internal/modules/*)
// builds its pages with: modules live in their own packages, so they reach
// these through exported methods.

// Logger - the controller's logger.
func (c *BaseController) Logger() *slog.Logger { return c.logger }

// SetLogger replaces the controller's logger (e.g. with the module's
// component attribute).
func (c *BaseController) SetLogger(l *slog.Logger) { c.logger = l }

// AuthService - the authentication and permissions service.
func (c *BaseController) AuthService() service.AuthService { return c.authService }

// TemplateService - the template renderer.
func (c *BaseController) TemplateService() service.TemplateService { return c.templateService }

// SetNodeRefTrail - the breadcrumbs of a node page without loaded fields.
func (c *BaseController) SetNodeRefTrail(ctx context.Context, data map[string]any, node entity.Node) {
	c.setNodeRefTrail(ctx, data, node)
}

// SetNodeSubpageTrail - the breadcrumbs of a node's subpage (title under
// the node).
func (c *BaseController) SetNodeSubpageTrail(ctx context.Context, data map[string]any, node entity.Node, title string, extra ...entity.BreadcrumbItem) {
	c.setNodeSubpageTrail(ctx, data, node, title, extra...)
}

// SetPathTrail - the breadcrumbs of a page at path (by the menus), or
// just its title.
func (c *BaseController) SetPathTrail(data map[string]any, path, title string) {
	c.setPathTrail(data, path, title)
}

// SetChildTrail - the breadcrumbs of a page under parents.
func (c *BaseController) SetChildTrail(data map[string]any, parents []entity.BreadcrumbItem, title string) {
	c.setChildTrail(data, parents, title)
}

// SetCreateFormTrail - the breadcrumbs of a create form under /create.
func (c *BaseController) SetCreateFormTrail(data map[string]any, title string) {
	c.setCreateFormTrail(data, title)
}

// Store - the session store (CSRF checks of JSON requests).
func (c *BaseController) Store() *sessions.CookieStore { return c.store }

// AdminPageData - the common data of an admin page (a service page: the
// theme decides what it leaves out, see data["servicePage"]).
func (c *BaseController) AdminPageData(r *http.Request) map[string]any {
	data := c.GetCommonData(r)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Admin panel")}
	data["servicePage"] = true
	AddStatusCounts(r, data)
	return data
}

// pageDataHooks - the modules' additions to every page's data.
var pageDataHooks []func(r *http.Request, data map[string]any)

// AddPageData registers a module's addition to every page's data (e.g.
// what its page_end partial needs); called at startup. A module needing
// the reCAPTCHA script on every page sets data["recaptchaScript"].
func AddPageData(fn func(r *http.Request, data map[string]any)) {
	pageDataHooks = append(pageDataHooks, fn)
}

// AddUserMenuCount adds n to the number on the user menu's toggle
// (data["userMenuCount"]: the core's header and admin header show it when
// it is above 0) - e.g. a module's unread items, from its AddPageData.
func AddUserMenuCount(data map[string]any, n int) {
	if n <= 0 {
		return
	}
	have, _ := data["userMenuCount"].(int)
	data["userMenuCount"] = have + n
}

// ProfileTab - a module's tab of the user profile, at /me/<Path> and
// /<profile>/<Path> (the core's profile routes serve it). Only an enabled
// module's tabs count. A View's user_tab display can't take a module
// tab's path; one that has it anyway (saved before the module was there)
// isn't listed - the module's tab is served.
type ProfileTab struct {
	// Module - the module's name.
	Module string
	// Path - the URL segment of the tab.
	Path string
	// Label - the tab's name in English, translated with i18n.T when
	// shown.
	Label string
	// Allowed - whether viewer (nil - a guest) may open the tab on
	// profile's page; isOwner - it is the viewer's own profile,
	// canAdminister - the viewer may administer users. False: the tab
	// isn't listed and its URL answers 403 (a guest is offered to log
	// in). Nil - everyone may.
	Allowed func(ctx context.Context, viewer, profile *entity.User, isOwner, canAdminister bool) bool
	// Badge - a number next to the label on the viewer's own profile (0 -
	// none), e.g. their unread items. Nil - none.
	Badge func(ctx context.Context, viewer *entity.User) int
	// Load fills data - the profile page's data (profileUser, pageSEO…
	// set) - for the tab's content: the module's template
	// "profile_tab_<module>" renders it (renderOptional). An error is
	// logged and the tab rendered with what data has.
	Load func(ctx context.Context, w http.ResponseWriter, r *http.Request, profile *entity.User, data map[string]any) error
}

var profileTabs []ProfileTab

// AddProfileTab registers a module's profile tab (called at startup, from
// Wire); its path is reserved for the user_tab displays
// (entity.ReserveProfileTabPath).
func AddProfileTab(t ProfileTab) {
	profileTabs = append(profileTabs, t)
	entity.ReserveProfileTabPath(t.Path)
}

// enabledProfileTab - the enabled module's tab at path.
func enabledProfileTab(path string) (ProfileTab, bool) {
	for _, t := range profileTabs {
		if t.Path == path && service.ModuleEnabled(t.Module) {
			return t, true
		}
	}
	return ProfileTab{}, false
}

// NodeLink - a module's link on a node page (e.g. "Quotes (12)" on a
// person's page), shown under the node by node_module_links.
type NodeLink struct {
	URL   string
	Label string
}

// NodePageParts - what a module adds to a node page. Data is what its
// templates read as .moduleNode.<module>: "node_links_<module>" (under
// the node, before Links), "node_actions_<module>" (next to Edit) and its
// plugin blocks (a block on the page, placed in a theme region; see
// entity.ModulePluginBlock.Region). Comments - the module shows the
// node's comments in place of the core's (a comments service): its
// "comments_<module>" with the page's data where the core's comments
// would be; the node's comment mode still applies (hidden - none). The
// first enabled module saying so wins.
type NodePageParts struct {
	Data     any
	Links    []NodeLink
	Comments bool
}

// nodePageHook - a module's part of node pages.
type nodePageHook struct {
	module string
	fn     func(ctx context.Context, node entity.Node) NodePageParts
}

var nodePageHooks []nodePageHook

// AddNodePage registers a module's part of node pages (called at startup;
// it runs only while the module is on).
func AddNodePage(module string, fn func(ctx context.Context, node entity.Node) NodePageParts) {
	nodePageHooks = append(nodePageHooks, nodePageHook{module, fn})
}

// ModuleNodeData - the modules' data for a node's page (.moduleNode: what
// their "node_actions_<module>" and other templates read), for a module's
// own page of its content type (a poll, a form).
func ModuleNodeData(ctx context.Context, node entity.Node) map[string]any {
	return moduleNodePage(ctx, node).data
}

// moduleNodeParts - the enabled modules' parts of a node page.
type moduleNodeParts struct {
	// data - their templates' data by module (.moduleNode).
	data  map[string]any
	links []NodeLink
	// comments - the module showing the node's comments in place of the
	// core's (NodePageParts.Comments); "" - the core's.
	comments string
}

// moduleNodePage - every enabled module's part of the node page.
func moduleNodePage(ctx context.Context, node entity.Node) moduleNodeParts {
	parts := moduleNodeParts{data: map[string]any{}}
	for _, h := range nodePageHooks {
		if !service.ModuleEnabled(h.module) {
			continue
		}
		p := h.fn(ctx, node)
		if p.Data != nil {
			parts.data[h.module] = p.Data
		}
		parts.links = append(parts.links, p.Links...)
		if p.Comments && parts.comments == "" {
			parts.comments = h.module
		}
	}
	return parts
}

// termPageHook - a module's part of term pages.
type termPageHook struct {
	module string
	fn     func(ctx context.Context, term entity.TaxonomyTerm) any
}

var termPageHooks []termPageHook

// AddTermPage registers a module's part of term pages: fn's result is the
// data of its templates there (.moduleTerm.<module>, e.g. its
// "term_actions_<module>"). Called at startup; it runs only while the
// module is on.
func AddTermPage(module string, fn func(ctx context.Context, term entity.TaxonomyTerm) any) {
	termPageHooks = append(termPageHooks, termPageHook{module, fn})
}

// ModuleTermData - the enabled modules' data for a term's page, by module.
func ModuleTermData(ctx context.Context, term entity.TaxonomyTerm) map[string]any {
	data := map[string]any{}
	for _, h := range termPageHooks {
		if !service.ModuleEnabled(h.module) {
			continue
		}
		if v := h.fn(ctx, term); v != nil {
			data[h.module] = v
		}
	}
	return data
}

// ContentTypeSettings - a module's section of the content type form,
// rendered by its template "content_type_settings_<module>" (moduleHook)
// from .moduleSettings.<module>, what Load returns. Save stores the
// submitted form once the type is saved.
type ContentTypeSettings struct {
	Module string
	Load   func(ctx context.Context, ct entity.ContentTypeDef) (any, error)
	Save   func(ctx context.Context, ct entity.ContentTypeDef, r *http.Request) error
}

var contentTypeSettings []ContentTypeSettings

// AddContentTypeSettings registers a module's section of the content type
// form (called at startup; used only while the module is on).
func AddContentTypeSettings(s ContentTypeSettings) {
	contentTypeSettings = append(contentTypeSettings, s)
}

// AccountSettingsSection - a module's section of the "Account settings"
// page (its settings of accounts), rendered by its template
// "account_settings_<module>" (moduleHook) from .moduleSettings.<module>,
// what Load returns. Save stores the submitted form once the page's own
// settings are saved.
type AccountSettingsSection struct {
	Module string
	Load   func(ctx context.Context) (any, error)
	Save   func(ctx context.Context, r *http.Request) error
}

var accountSettingsSections []AccountSettingsSection

// AddAccountSettingsSection registers a module's section of the "Account
// settings" page (called at startup; used only while the module is on).
func AddAccountSettingsSection(s AccountSettingsSection) {
	accountSettingsSections = append(accountSettingsSections, s)
}

// LoadAccountSettingsSections - the enabled modules' data of the "Account
// settings" page's sections (a failing one is left out).
func (c *BaseController) LoadAccountSettingsSections(ctx context.Context) map[string]any {
	out := map[string]any{}
	for _, s := range accountSettingsSections {
		if !service.ModuleEnabled(s.Module) {
			continue
		}
		v, err := s.Load(ctx)
		if err != nil {
			c.logger.Warn("account settings of a module", "module", s.Module, "error", err)
			continue
		}
		out[s.Module] = v
	}
	return out
}

// SaveAccountSettingsSections stores the enabled modules' sections of the
// submitted "Account settings" page.
func SaveAccountSettingsSections(ctx context.Context, r *http.Request) error {
	for _, s := range accountSettingsSections {
		if !service.ModuleEnabled(s.Module) {
			continue
		}
		if err := s.Save(ctx, r); err != nil {
			return fmt.Errorf("%s: %w", s.Module, err)
		}
	}
	return nil
}

// UserSettingsSection - a module's section of a user's profile form
// (/me/edit, /user/{id}/edit: the user's own settings in the module, such
// as its e-mails), rendered by its template "user_settings_<module>"
// (moduleHook) from .moduleSettings.<module>, what Load returns for the
// user edited. Save stores the submitted form once the profile is saved.
type UserSettingsSection struct {
	Module string
	Load   func(ctx context.Context, user entity.User) (any, error)
	Save   func(ctx context.Context, user entity.User, r *http.Request) error
}

var userSettingsSections []UserSettingsSection

// AddUserSettingsSection registers a module's section of the profile form
// (called at startup; used only while the module is on).
func AddUserSettingsSection(s UserSettingsSection) {
	userSettingsSections = append(userSettingsSections, s)
}

// loadUserSettingsSections - the enabled modules' data of the profile
// form's sections for user (a failing one is left out).
func (c *BaseController) loadUserSettingsSections(ctx context.Context, user entity.User) map[string]any {
	out := map[string]any{}
	for _, s := range userSettingsSections {
		if !service.ModuleEnabled(s.Module) {
			continue
		}
		v, err := s.Load(ctx, user)
		if err != nil {
			c.logger.Warn("user settings of a module", "module", s.Module, "userID", user.ID, "error", err)
			continue
		}
		out[s.Module] = v
	}
	return out
}

// saveUserSettingsSections stores the enabled modules' sections of the
// submitted profile form of user.
func saveUserSettingsSections(ctx context.Context, user entity.User, r *http.Request) error {
	for _, s := range userSettingsSections {
		if !service.ModuleEnabled(s.Module) {
			continue
		}
		if err := s.Save(ctx, user, r); err != nil {
			return fmt.Errorf("%s: %w", s.Module, err)
		}
	}
	return nil
}

// NodeSettings - a module's section of a node's form ("Settings"),
// rendered by its template "node_settings_<module>" (moduleHook) from
// .moduleSettings.<module>, what Load returns: for the node (nil - a new
// node of the type), or what was submitted (non-nil on a form shown again
// with errors). The section is shown to "administer content" only: Save
// stores the submitted form once the node is saved by such a user; for a
// new node saved by anyone else, Default (optional) gives it the
// section's defaults, and an existing node keeps its settings.
type NodeSettings struct {
	Module  string
	Load    func(ctx context.Context, contentType string, node *entity.Node, submitted url.Values) (any, error)
	Save    func(ctx context.Context, node entity.Node, r *http.Request) error
	Default func(ctx context.Context, node entity.Node) error
}

var nodeSettings []NodeSettings

// AddNodeSettings registers a module's section of node forms (called at
// startup; used only while the module is on).
func AddNodeSettings(s NodeSettings) {
	nodeSettings = append(nodeSettings, s)
}

// nodeFormSaved - the modules' calls after a node's own form saved it
// (AddNodeFormSaved).
var nodeFormSaved []struct {
	module string
	fn     func(ctx context.Context, node entity.Node, isNew bool) error
}

// AddNodeFormSaved registers fn, called while module is on after a
// node's own form (create or edit) saved it, in the save's transaction:
// its error fails the save. Not called for changes made elsewhere - the
// bulk actions, publishing, the schedule, a copy.
func AddNodeFormSaved(module string, fn func(ctx context.Context, node entity.Node, isNew bool) error) {
	nodeFormSaved = append(nodeFormSaved, struct {
		module string
		fn     func(ctx context.Context, node entity.Node, isNew bool) error
	}{module, fn})
}

// runNodeFormSaved runs the enabled modules' AddNodeFormSaved calls.
func runNodeFormSaved(ctx context.Context, node entity.Node, isNew bool) error {
	for _, h := range nodeFormSaved {
		if !service.ModuleEnabled(h.module) {
			continue
		}
		if err := h.fn(ctx, node, isNew); err != nil {
			return fmt.Errorf("%s: %w", h.module, err)
		}
	}
	return nil
}

// LoadContentTypeSettings - the enabled modules' data of the content type
// form's sections (a failing one is left out).
func (c *BaseController) LoadContentTypeSettings(ctx context.Context, ct entity.ContentTypeDef) map[string]any {
	out := map[string]any{}
	for _, s := range contentTypeSettings {
		if !service.ModuleEnabled(s.Module) {
			continue
		}
		v, err := s.Load(ctx, ct)
		if err != nil {
			c.logger.Warn("content type settings of a module", "module", s.Module, "type", ct.MachineName, "error", err)
			continue
		}
		out[s.Module] = v
	}
	return out
}

// SaveContentTypeSettings stores the enabled modules' sections of the
// submitted content type form.
func SaveContentTypeSettings(ctx context.Context, ct entity.ContentTypeDef, r *http.Request) error {
	for _, s := range contentTypeSettings {
		if !service.ModuleEnabled(s.Module) {
			continue
		}
		if err := s.Save(ctx, ct, r); err != nil {
			return fmt.Errorf("%s: %w", s.Module, err)
		}
	}
	return nil
}

// loadNodeSettings - the enabled modules' data of a node form's sections
// (node nil - a new node; a failing one is left out).
func (c *BaseController) loadNodeSettings(ctx context.Context, contentType string, node *entity.Node, submitted url.Values) map[string]any {
	out := map[string]any{}
	for _, s := range nodeSettings {
		if !service.ModuleEnabled(s.Module) {
			continue
		}
		v, err := s.Load(ctx, contentType, node, submitted)
		if err != nil {
			c.logger.Warn("node settings of a module", "module", s.Module, "type", contentType, "error", err)
			continue
		}
		out[s.Module] = v
	}
	return out
}

// saveNodeSettings stores the enabled modules' sections of a submitted
// node form when the form showed them (shown: the user may administer
// content); otherwise a new node (isNew) gets their defaults
// (NodeSettings.Default) and an existing one keeps its settings.
func saveNodeSettings(ctx context.Context, node entity.Node, r *http.Request, shown, isNew bool) error {
	for _, s := range nodeSettings {
		if !service.ModuleEnabled(s.Module) {
			continue
		}
		var err error
		switch {
		case shown:
			err = s.Save(ctx, node, r)
		case isNew && s.Default != nil:
			err = s.Default(ctx, node)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", s.Module, err)
		}
	}
	return nil
}

// createLinkHooks - the modules' entries of the "Create content" menu.
var createLinkHooks []func(ctx context.Context, user *entity.User, auth service.AuthService) []entity.CreatableMaterialLink

// AddCreateLinks registers a module's entries of the "Create content"
// menu for a signed-in user (called at startup; the hook checks its module
// is on and the user may create).
func AddCreateLinks(fn func(ctx context.Context, user *entity.User, auth service.AuthService) []entity.CreatableMaterialLink) {
	createLinkHooks = append(createLinkHooks, fn)
}

// sitemapPathHooks - the modules' pages for the sitemap.
var sitemapPathHooks []func(ctx context.Context) ([]string, error)

// AddSitemapPaths registers a module's pages for the sitemap (paths from
// the site root, escaped; called at startup - the hook checks its module
// is on).
func AddSitemapPaths(fn func(ctx context.Context) ([]string, error)) {
	sitemapPathHooks = append(sitemapPathHooks, fn)
}

// ResetModuleHooks drops every module hook registered so far (AddPageData,
// AddNodePage, AddTermPage, AddContentTypeSettings,
// AddAccountSettingsSection, AddUserSettingsSection, AddNodeSettings,
// AddNodeFormSaved, AddFormSection, SetNodeWorkflow, AddCreateLinks,
// AddSitemapPaths, AddProfileTab, the modules' View criteria, the action
// listeners, the 404 hooks, the dashboard blocks, the published-node
// hooks). The app's wiring calls it before the modules register theirs,
// so wiring the app again (a test serving another database) doesn't keep
// hooks bound to the previous one.
func ResetModuleHooks() {
	pageDataHooks = nil
	termPageHooks = nil
	audit.ResetListeners()
	service.ResetNotFoundHooks()
	service.ResetDashboardBlocks()
	service.ResetNodePublished()
	profileTabs = nil
	entity.ResetProfileTabPaths()
	nodePageHooks = nil
	contentTypeSettings = nil
	accountSettingsSections = nil
	userSettingsSections = nil
	nodeSettings = nil
	nodeFormSaved = nil
	formSections = nil
	nodeWorkflow = nil
	service.SetPublishingManaged(nil)
	createLinkHooks = nil
	sitemapPathHooks = nil
	resetPathOwners()
	views.ResetModuleCriteria()
}

// EscapePathSegments - a relative path with each segment URL-escaped.
func EscapePathSegments(rel string) string { return escapePathSegments(rel) }

// Timeouts of the request handlers (config "timeouts"; SetTimeouts at
// startup): a page's database work and the server's write timeout.
var (
	dbQueryTimeout     = 30 * time.Second
	serverWriteTimeout = 15 * time.Second
)

// SetTimeouts sets the handlers' timeouts from the config: the database
// work of a page and the server's write timeout (the time a response
// has; StepBudget is derived from it).
func SetTimeouts(dbQuery, serverWrite time.Duration) {
	dbQueryTimeout, serverWriteTimeout = dbQuery, serverWrite
}

// StepBudget - how long one request of a step-by-step job (deleting a
// user's or a module's content) may work before answering "go on": half
// the server's write timeout, so the answer always gets out.
func StepBudget() time.Duration {
	if b := serverWriteTimeout / 2; b >= time.Second {
		return b
	}
	return time.Second
}

// Captcha - the reCAPTCHA in effect (nil - none set).
func Captcha() *service.CaptchaService { return captcha }

// TruncateMeta - s cut for a meta description of at most n characters.
func TruncateMeta(s string, n int) string { return truncateMeta(s, n) }

// PublicBaseURL - the site's canonical origin (the site URL, else as the
// request reached it), for absolute links in feeds.
func PublicBaseURL(r *http.Request) string { return publicBaseURL(r) }

// MailBaseURL - the site's origin for links in e-mails ("" - the site URL
// isn't set), never the request's host (mailBaseURL).
func MailBaseURL() string { return mailBaseURL() }

// ParseExtensionsForm - the file extensions typed in an admin form
// (space separated, without dots, lowercase), forbidden ones dropped.
func ParseExtensionsForm(raw string) []string { return parseExtensionsForm(raw) }

// MaxFileSizeBytesFromMBForm - a size limit typed in megabytes in an admin
// form, in bytes.
func MaxFileSizeBytesFromMBForm(raw string) int { return maxFileSizeBytesFromMBForm(raw) }

// NodeRepo - the nodes repository.
func (c *BaseController) NodeRepo() entity.NodesRepository { return c.nodeRepo }

// UserService - the users service.
func (c *BaseController) UserService() service.UserService { return c.userService }

// nodeCopier - what copies a node (SetNodeCopier, CopyNode).
var nodeCopier func(ctx context.Context, nodeID int, user *entity.User, title string) (int, error)

// SetNodeCopier sets what copies a node for the modules: the content
// form's controller (DynamicContentController.CopyNode), at startup.
func SetNodeCopier(fn func(ctx context.Context, nodeID int, user *entity.User, title string) (int, error)) {
	nodeCopier = fn
}

// CopyNode - a copy of the node nodeID by user (DynamicContentController.
// CopyNode): its id.
func CopyNode(ctx context.Context, nodeID int, user *entity.User, title string) (int, error) {
	if nodeCopier == nil {
		return 0, ErrNodeNotCopyable
	}
	return nodeCopier(ctx, nodeID, user, title)
}

// FormSection - a module's section of one of the core's admin forms
// (Form - its name: FormMenuLink…) for a thing of it (ref - its id),
// rendered by the module's template "form_<Form>_<module>" (moduleHook)
// from .moduleForms.<Form>.<module>, what Load returns (nil - nothing to
// show); Save stores the submitted form once the form saved the thing.
// A page may hold several forms' sections (a custom block's: the
// block's and its placement's).
type FormSection struct {
	Module string
	Form   string
	Load   func(ctx context.Context, ref string) (any, error)
	Save   func(ctx context.Context, ref string, r *http.Request) error
}

// The core's admin forms with module sections (FormSection.Form).
const (
	// FormMenuLink - a menu link's form (ref - the link's id).
	FormMenuLink = "menu_link"
	// FormBlock - a custom block's form (ref - the block's id).
	FormBlock = "block"
	// FormBlockPlacement - a block placement's form (ref - its id): a
	// custom block's page has it too.
	FormBlockPlacement = "block_placement"
	// FormTerm - a taxonomy term's form (ref - the term's id).
	FormTerm = "term"
	// FormSiteInformation - Site information (ref - entity.SiteTextRef).
	FormSiteInformation = "site_information"
	// FormView - a View display's settings (ref - its id): a modal's
	// form, Bootstrap's form layout rather than the settings rows.
	FormView = "view"
	// FormField - a content type's field's form (ref - the field
	// instance's id).
	FormField = "field"
)

var formSections []FormSection

// AddFormSection registers a module's section of a core admin form
// (called at startup; used only while the module is on).
func AddFormSection(s FormSection) {
	formSections = append(formSections, s)
}

// LoadFormSections - the enabled modules' data of form's sections for
// ref (a failing one is left out).
func (c *BaseController) LoadFormSections(ctx context.Context, form, ref string) map[string]any {
	out := map[string]any{}
	for _, s := range formSections {
		if s.Form != form || !service.ModuleEnabled(s.Module) {
			continue
		}
		v, err := s.Load(ctx, ref)
		if err != nil {
			c.logger.Warn("a module's section of a form", "module", s.Module, "form", form, "ref", ref, "error", err)
			continue
		}
		if v != nil {
			out[s.Module] = v
		}
	}
	return out
}

// AddFormSectionsData puts form's sections for ref in a page's data
// (.moduleForms.<form>.<module>).
func (c *BaseController) AddFormSectionsData(ctx context.Context, data map[string]any, form, ref string) {
	forms, _ := data["moduleForms"].(map[string]map[string]any)
	if forms == nil {
		forms = map[string]map[string]any{}
		data["moduleForms"] = forms
	}
	forms[form] = c.LoadFormSections(ctx, form, ref)
}

// SaveFormSections stores the enabled modules' sections of the submitted
// form for ref.
func SaveFormSections(ctx context.Context, form, ref string, r *http.Request) error {
	for _, s := range formSections {
		if s.Form != form || !service.ModuleEnabled(s.Module) {
			continue
		}
		if err := s.Save(ctx, ref, r); err != nil {
			return fmt.Errorf("%s: %w", s.Module, err)
		}
	}
	return nil
}

// NodeWorkflow - a module's editorial workflow: the items of the content
// types it Handles are published by it, not by their form - the form
// shows its "node_workflow_<module>" template (the item's state, the
// transitions the user may make) from .nodeWorkflow, what Load returns
// (node nil - a new item); Validate (optional) - why the transition
// submitted (r) can't be made ("" - it can), shown with the form's
// errors; Apply, after the form saved the item (in the save's
// transaction), makes it and sets the item's publishing - its error
// fails the save. The items' own publish/unpublish buttons are left out.
type NodeWorkflow struct {
	Module   string
	Handles  func(ctx context.Context, contentType string) bool
	Load     func(ctx context.Context, contentType string, node *entity.Node, user *entity.User) (any, error)
	Validate func(ctx context.Context, contentType string, node *entity.Node, user *entity.User, r *http.Request) string
	Apply    func(ctx context.Context, node entity.Node, user *entity.User, r *http.Request, isNew bool) error
}

// nodeTracker - what records a change of a node in its history
// (SetNodeTracker, ChangeNode).
var nodeTracker func(ctx context.Context, nodeID int, user *entity.User, change func(ctx context.Context) error) error

// SetNodeTracker sets what records a change of a node (the node history's
// Track), at startup.
func SetNodeTracker(fn func(ctx context.Context, nodeID int, user *entity.User, change func(ctx context.Context) error) error) {
	nodeTracker = fn
}

// ChangeNode runs change of the node nodeID by user in one transaction,
// recorded in its history (a revision, the action log, the published
// hooks) as the bulk actions' are.
func ChangeNode(ctx context.Context, nodeID int, user *entity.User, change func(ctx context.Context) error) error {
	if nodeTracker == nil {
		return change(ctx)
	}
	return nodeTracker(ctx, nodeID, user, change)
}

var nodeWorkflow *NodeWorkflow

// SetNodeWorkflow sets the site's editorial workflow (one; called at
// startup, used only while its module is on); the core's own publishing
// leaves its types' items alone (service.PublishingManaged).
func SetNodeWorkflow(w NodeWorkflow) {
	nodeWorkflow = &w
	service.SetPublishingManaged(func(ctx context.Context, contentType string) bool { return NodeWorkflowFor(ctx, contentType) != nil })
}

// NodeWorkflowFor - the workflow publishing contentType's items (nil -
// none: their form publishes them).
func NodeWorkflowFor(ctx context.Context, contentType string) *NodeWorkflow {
	w := nodeWorkflow
	if w == nil || !service.ModuleEnabled(w.Module) || !w.Handles(ctx, contentType) {
		return nil
	}
	return w
}
