package modulekit

import (
	"context"
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/gorilla/sessions"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/filestorage"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/internal/lib/mailsender"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
)

// A module's manifest and own schema use modulekit/manifestkit (the
// module's description, its schema's registration); its code, this.

// The core's data a module reads.
type (
	// User - a site user.
	User = entity.User
	// PageSEO - a page's title and meta tags.
	PageSEO = entity.PageSEO
	// Node - a material (a node).
	Node = entity.Node
	// ContentType - a content type's machine name.
	ContentType = entity.ContentType
	// NodesRepository - the materials (Deps.NodeRepo).
	NodesRepository = entity.NodesRepository
	// BreadcrumbItem - a link of a page's breadcrumbs.
	BreadcrumbItem = entity.BreadcrumbItem
	// CreatableMaterialLink - an item of the "Create" menu (AddCreateLinks).
	CreatableMaterialLink = entity.CreatableMaterialLink
	// AuthService - who the user is and what they may (Deps.Base.Auth).
	AuthService = service.AuthService
	// UserService - the users' service (BaseDeps.Users,
	// BaseController.UserService).
	UserService = service.UserService
	// NodesService - the materials' service (Deps.Nodes).
	NodesService = service.NodesService
	// CommentMode - a material's comments: open, closed, hidden.
	CommentMode = entity.CommentMode
	// PluginBlockTarget - one thing a plugin block can show.
	PluginBlockTarget = entity.PluginBlockTarget
	// PluginBlockID - a plugin block's id (the module's, its manifest's).
	PluginBlockID = entity.PluginBlockID
	// NodeTypeController - the pages of a module's content type
	// (Parts.NodeTypes).
	NodeTypeController = interfaces.NodeTypeController
	// NodeFormPreviewer - a module's content type whose form previews its
	// item unsaved (FormPreviewPath; without it - no Preview button).
	NodeFormPreviewer = interfaces.NodeFormPreviewer
	// UnsavedPreview - the strip over a previewed item's page: the page's
	// data "unsavedPreview".
	UnsavedPreview = interfaces.UnsavedPreview
	// NodeMetaFields - a material form's title and description fields.
	NodeMetaFields = form.NodeMetaFields
	// Nodes - materials.
	Nodes = entity.Nodes
	// AdminNodesFilter - a page of the admin's content list
	// (NodesRepository.ListForAdmin): type, status, address, order.
	AdminNodesFilter = entity.AdminNodesFilter
	// ContentTypeDef - a content type (Deps.ContentTypes).
	ContentTypeDef = entity.ContentTypeDef
	// ContentTypeDefs - content types.
	ContentTypeDefs = entity.ContentTypeDefs
	// ContentTypeFieldType - a field's type (FieldType*).
	ContentTypeFieldType = entity.ContentTypeFieldType
	// FieldInstanceDef - a content type's field.
	FieldInstanceDef = entity.FieldInstanceDef
	// FieldInstanceDefs - a content type's fields.
	FieldInstanceDefs = entity.FieldInstanceDefs
	// File - an uploaded file (an image).
	File = entity.File
	// Picture - an image's <img> (the template function img): its
	// address, other widths and size (StyledPicture).
	Picture = imageurl.Picture
	// SiteInformation - the site's name, e-mail, language, address.
	SiteInformation = entity.SiteInformation
	// TeaserItem - a material's card (Deps.Teasers).
	TeaserItem = entity.TeaserItem
	// ViewDisplayDef - a View's display.
	ViewDisplayDef = entity.ViewDisplayDef
	// FeedNodeFilter - the materials a feed takes (NodesRepository.ListForFeed).
	FeedNodeFilter = entity.FeedNodeFilter
	// FieldUploadPolicy - what an upload may be (extensions, size).
	FieldUploadPolicy = entity.FieldUploadPolicy
	// ContentTypeSettings - a module's section in the content type's form.
	ContentTypeSettings = interfaces.ContentTypeSettings
	// NodeSettings - a module's section in the material's form.
	NodeSettings = interfaces.NodeSettings
	// AccountSettingsSection - a module's section of the "Account
	// settings" page (its settings of accounts).
	AccountSettingsSection = interfaces.AccountSettingsSection
	// UserSettingsSection - a module's section of a user's profile form
	// (the user's own settings in the module).
	UserSettingsSection = interfaces.UserSettingsSection
	// Comment - a comment (Deps.Comments).
	Comment = entity.Comment
	// Comments - comments.
	Comments = entity.Comments
	// TaxonomyTerm - a term of a vocabulary.
	TaxonomyTerm = entity.TaxonomyTerm
	// TaxonomyTerms - terms.
	TaxonomyTerms = entity.TaxonomyTerms
	// TaxonomyVocabulary - a vocabulary.
	TaxonomyVocabulary = entity.TaxonomyVocabulary
	// TemplateService - the templates (BaseController.TemplateService).
	TemplateService = service.TemplateService
	// ImageStyleDef - an image style.
	ImageStyleDef = entity.ImageStyleDef
	// ImageStyleDefs - image styles.
	ImageStyleDefs = entity.ImageStyleDefs
)

// The field types a module looks for (ContentTypeFieldType).
const (
	FieldTypeText     = entity.FieldTypeText
	FieldTypeTaxonomy = entity.FieldTypeTaxonomy
	FieldTypeImage    = entity.FieldTypeImage
	FieldTypeLink     = entity.FieldTypeLink
)

// Link - one value of a link field: its address and title (Text - the
// title, else the address; Host - its site).
type Link = entity.Link

// Links - a link field's value as a FieldFormatter gets it: its links.
func Links(v any) []Link { return entity.Links(v) }

// FieldLayout - how a field with several values shows them: all one after
// another, the first large and the others small (FieldLayoutFeatured), or
// the first only (FieldLayoutFirst - the formatter gets just that one).
type FieldLayout = entity.FieldLayout

// The layouts of FieldLayout.
const (
	FieldLayoutAll      = entity.FieldLayoutAll
	FieldLayoutFeatured = entity.FieldLayoutFeatured
	FieldLayoutFirst    = entity.FieldLayoutFirst
)

// SiteDomain - the site's address (public_base_url), "" when not set.
func SiteDomain() string { return entity.NewSystemSettings().Domain }

// ContentVisitor - what of the content a visitor may not view
// (AuthService.ContentVisitor).
type ContentVisitor = entity.ContentVisitor

// WithContentVisitor - ctx whose content and comment queries leave out
// what v may not view: a background job working for one user (their
// e-mail) sees what they may (AuthService.ContentVisitor).
func WithContentVisitor(ctx context.Context, v ContentVisitor) context.Context {
	return entity.WithContentVisitor(ctx, v)
}

// AsGuest - ctx whose content queries see only what a guest may view
// (the content closed to guests left out): for output that is public by
// nature and kept by any cache, whoever asks (a feed).
func AsGuest(ctx context.Context) (context.Context, error) { return interfaces.AsGuest(ctx) }

// EmbedKind - a kind of thing a text embeds with a tag [[<kind>:<name>]]
// (or a template with {{embed "<kind>:<name>"}}): Render gives what name
// embeds for its ctx's visitor (the request's: access, language, zone),
// ErrEntityNotFound for a missing one; Label, Search and Describe offer
// the kind in the editor's "Insert" window.
type EmbedKind = service.EmbedKind

// EmbedOption - a thing of the module's kind as the editor shows it (the
// "Insert" window's list and preview, the card over its tag): EmbedKind's
// Search and Describe give them.
type EmbedOption = service.EmbedOption

// AddEmbedKind adds the module's kind of embedded thing (its Module set:
// off with the module).
func AddEmbedKind(k EmbedKind) { service.RegisterEmbedKind(k) }

// EmbedTemplate renders an embedded thing: the template define (one of the
// module's templates/blocks/*.html, always parsed along) with data, in
// the zone, language and theme of ctx's request.
func EmbedTemplate(ctx context.Context, templates TemplateService, define string, data map[string]any) (template.HTML, error) {
	return service.EmbedTemplate(ctx, templates, define, data)
}

// DocumentAllowed - whether a file a visitor sends (name, its first
// bytes) may be kept: not of an extension never accepted (pages, scripts,
// programs), not content a browser could run (HTML, SVG, scripts,
// executables) whatever the name - the core's own check of documents.
func DocumentAllowed(name string, head []byte) bool { return files.DocumentAllowed(name, head) }

// PrivateFiles - the store of the private files (on disk or in a bucket),
// never served as they are: a module serves them itself, after its own
// access check.
func PrivateFiles() filestore.Store { return paths.PrivateStore() }

// PublicFilesURL - the address of the public files (uploads).
func PublicFilesURL() string { return paths.PublicFilesURL() }

// PublicFiles - the store of the public files (on disk or in an object
// storage): a module keeps files of its own there under a folder named
// after it; a file's key is its path under PublicFilesURL.
func PublicFiles() filestore.Store { return paths.PublicStore() }

// FileStorage - a storage the site's files can be kept in besides its
// directories (files.storage, files.private_storage name it): Public opens
// the public files' store, Private the private files' (nil - none).
type FileStorage = filestorage.Driver

// FileStorageConfig - what a FileStorage opens its store from: its own
// section of the config's files (Decode) and the environment.
type FileStorageConfig = filestorage.Config

// AddFileStorage adds a file storage, from the package's init: importing
// the package ("_" in the site's main package, app module add) is all it
// takes. A name already taken panics at startup.
func AddFileStorage(d FileStorage) { filestorage.Register(d) }

// OriginalImage - an image's original file's address.
func OriginalImage(f File) File { return imageurl.Original(f) }

// StyledImage - an image's address in the image style styleName.
func StyledImage(f File, styleName string) File { return imageurl.Styled(f, styleName) }

// StyledPicture - an image in the image style styleName, for the
// template function img: its address, the style's other widths (srcset)
// and its size.
func StyledPicture(f File, styleName string) Picture { return imageurl.StyledPicture(f, styleName) }

// OriginalPicture - the image itself, for the template function img.
func OriginalPicture(f File) Picture { return imageurl.OriginalPicture(f) }

// PlainPreview - the start of s as plain text, at most limit characters.
func PlainPreview(s string, limit int) string { return service.GetPlainPreview(s, limit) }

// PublicBaseURL - the site's address for the request (absolute links).
func PublicBaseURL(r *http.Request) string { return interfaces.PublicBaseURL(r) }

// The core's feed documents, for a module's own feeds.
type (
	// RSSDoc - an RSS document (its <rss>).
	RSSDoc = interfaces.RSSDoc
	// RSSChannel - the feed's channel: its title, address, items.
	RSSChannel = interfaces.RSSChannel
	// RSSItem - a feed's item.
	RSSItem = interfaces.RSSItem
	// RSSGUID - an item's guid.
	RSSGUID = interfaces.RSSGUID
	// RSSEnclosure - an item's attached file (an image).
	RSSEnclosure = interfaces.RSSEnclosure
)

// RSSPostURL - a material's absolute address in a feed.
func RSSPostURL(base string, n Node) string { return interfaces.RSSPostURL(base, n) }

// AbsoluteAssetURL - uri (a file's address) absolute on base.
func AbsoluteAssetURL(base, uri string) string { return interfaces.AbsoluteAssetURL(base, uri) }

// ImageMIME - an image's media type by its address.
func ImageMIME(uri string) string { return interfaces.ImageMIME(uri) }

// The core's permissions a module checks.
const (
	PermAdministerContent = service.PermAdministerContent
	PermAccessAdmin       = service.PermAccessAdmin
	PermViewUnpublished   = service.PermViewUnpublished
)

// The comment modes.
const (
	CommentModeOpen   = entity.CommentModeOpen
	CommentModeClosed = entity.CommentModeClosed
	CommentModeHidden = entity.CommentModeHidden
)

// IsValidCommentMode - whether s is a comment mode.
func IsValidCommentMode(s string) bool { return entity.IsValidCommentMode(s) }

// SetPluginBlockTargets sets what the module's plugin block id can show
// (the placement page's choice): called once, from Wire.
func SetPluginBlockTargets(id PluginBlockID, list func(ctx context.Context) ([]PluginBlockTarget, error)) {
	service.SetPluginBlockTargets(id, list)
}

// A plugin block's own settings and data.
type (
	// PluginBlockSetting - one setting of a plugin block on its placement
	// page (SetPluginBlockSettings); its value is the placement's
	// PluginSetting "<Key>" in the block's template.
	PluginBlockSetting = entity.PluginBlockSetting
	// PluginBlockSettingType - a setting's kind: a number, a choice, a line
	// of text.
	PluginBlockSettingType = entity.PluginBlockSettingType
	// BlockPlacement - where a block is placed (a region of a theme) and
	// its settings (PluginSetting).
	BlockPlacement = entity.BlockPlacementDef
	// Region - a theme's region (BlockPlacement.Region).
	Region = entity.Region
	// DynamicContent - a material with its fields' values (the content
	// form's data, .dc).
	DynamicContent = entity.DynamicContent
	// TaxonomyVocabularies - vocabularies.
	TaxonomyVocabularies = entity.TaxonomyVocabularies
	// AccountSettings - the account settings (the "Account settings"
	// page's data, .account).
	AccountSettings = entity.AccountSettings
)

// A block placement's kind and visibility (BlockPlacement.PlacementType,
// VisibilityScope).
const (
	BlockPlacementPlugin = entity.BlockPlacementPlugin
	BlockVisibilityAll   = entity.BlockVisibilityAll
)

// The kinds of a plugin block's setting.
const (
	PluginBlockSettingNumber = entity.PluginBlockSettingNumber
	PluginBlockSettingSelect = entity.PluginBlockSettingSelect
	PluginBlockSettingText   = entity.PluginBlockSettingText
)

// SetPluginBlockSettings sets the settings of the module's plugin block id,
// offered on its placement page and kept with the placement: called once,
// from Wire; list - on every page and save (its options may come from the
// database).
func SetPluginBlockSettings(id PluginBlockID, list func(ctx context.Context) ([]PluginBlockSetting, error)) {
	service.SetPluginBlockSettings(id, list)
}

// SetPluginBlockData sets the loader of the module's plugin block id's
// data: called once, from Wire. Every refresh of the blocks' cache calls
// load for each of the block's placements; its result is the block
// template's .Data (templates don't query the database).
func SetPluginBlockData(id PluginBlockID, load func(ctx context.Context, p BlockPlacement) (any, error)) {
	service.SetPluginBlockData(id, load)
}

// The status report: a module's own checks (Parts.Requirements).
type (
	// StatusCheck - a module's checks: its lines of the status report, in
	// ctx's language.
	StatusCheck = service.StatusCheck
	// Requirement - one line: what was checked (Title), what was found
	// (Value) and, when not OK, what to do (Description, HTML);
	// translated.
	Requirement = entity.Requirement
	// RequirementSeverity - how a check came out.
	RequirementSeverity = entity.RequirementSeverity
)

// The outcomes of a check.
const (
	RequirementOK      = entity.RequirementOK
	RequirementWarning = entity.RequirementWarning
	RequirementError   = entity.RequirementError
)

// CheckWritableDir - a check that dir() is a directory the site can
// write to; title - the directory's name, English, translated with the
// report (i18n.N).
func CheckWritableDir(title string, dir func() string) StatusCheck {
	return service.CheckWritableDir(title, dir)
}

// CheckWritableStore - a check that the store a module keeps files in
// (PublicFiles, PrivateFiles) takes a new file; its line shows where the
// store keeps them.
func CheckWritableStore(title string, store func() filestore.Store) StatusCheck {
	return service.CheckWritableStore(title, store)
}

// ModuleNodeData - the modules' data for a material's page (AddNodePage
// parts and the like), for a module rendering its own material page.
func ModuleNodeData(ctx context.Context, node Node) map[string]any {
	return interfaces.ModuleNodeData(ctx, node)
}

// ModuleNodeTypePathPrefix - the path prefix of a module's content type's
// materials ("<type>/").
func ModuleNodeTypePathPrefix(nodeType string) string {
	return entity.ModuleNodeTypePathPrefix(nodeType)
}

// CommentsPerPage - the comments a material's page shows at a time.
func CommentsPerPage() int { return entity.CommentsPerPage }

// FormPreviewPath - where the form of a new item of contentType (nodeID
// 0) or of the item nodeID sends its preview: the content form's
// "formPreviewURL" (its Preview button).
func FormPreviewPath(contentType string, nodeID int) string {
	return interfaces.FormPreviewPath(contentType, nodeID)
}

// NodeTypesByLabel - the node types of labels
// (ContentTypeService.NodeTypeLabels) sorted by label.
func NodeTypesByLabel(labels map[string]string) []ContentType { return entity.NodeTypesByLabel(labels) }

// NodeRelPath - a material's address (without the leading slash) from
// its type and slug.
func NodeRelPath(nodeType ContentType, slug string) string { return entity.NodeRelPath(nodeType, slug) }

// EscapePathSegments - rel with each of its path segments escaped.
func EscapePathSegments(rel string) string { return interfaces.EscapePathSegments(rel) }

// ImageFormatsLabel - the image formats of a MIME allow-list as people
// know them ("JPEG, PNG"; FileUploadService.ImageTypes).
func ImageFormatsLabel(mimes []string) string { return files.ImageFormatsLabel(mimes) }

// Job - a background job the site's scheduler runs: Name (unique, e.g.
// "<module>_digest"), Every (a minute at least), Run (its error is
// logged; a run may take up to 50 seconds).
type Job = service.Job

// The action log (audit).
type (
	// Action - what was done: its Type, the Object it was done to, the
	// Changes.
	Action = audit.Action
	// ActionObject - what an action was done to.
	ActionObject = audit.Object
	// ActionChange - one changed value of an action.
	ActionChange = audit.Change
	// ActionGroup - a module's kind of actions: its group shows on the
	// log's filter and settings.
	ActionGroup = audit.Group
	// ActionType - an action of a group: its Label, its Tone
	// (ActionTone*).
	ActionType = audit.Type
	// ActionEvent - a recorded action, with who did it and when.
	ActionEvent = audit.Event
	// ActionActor - who did an action: the user, the address.
	ActionActor = audit.Actor
)

// The tones of action types (ActionType.Tone).
const (
	ActionToneCreate = audit.ToneCreate
	ActionToneChange = audit.ToneChange
	ActionToneDelete = audit.ToneDelete
	ActionToneAccess = audit.ToneAccess
	ActionToneLogin  = audit.ToneLogin
	ActionToneFail   = audit.ToneFail
)

// DashboardBlock - a module's block on the admin's Overview (/admin):
// its template dashboard_<Name> (the module's templates/partials), its
// card's Title (i18n.N) and "all" link, its column (Side), its Weight, the
// Permission it needs and its Data for the request.
type DashboardBlock = service.DashboardBlock

// MailDef - one of the site's emails, a module's (AddMail): its key
// ("<module>.<name>"), the Mail page's label, description and recipient
// (i18n.N), whether it can be switched off, its placeholders and their
// sample values, its default text (i18n.N); Load and Save when the module
// keeps the text itself (else the Mail page does).
type MailDef = entity.MailDef

// MailTemplate - an email's subject and body with placeholders.
type MailTemplate = entity.MailTemplate

// MailSitePlaceholders and MailUserPlaceholders - the placeholders of
// every email and of an email about a user, for MailDef.Placeholders.
var (
	MailSitePlaceholders = entity.MailSitePlaceholders
	MailUserPlaceholders = entity.MailUserPlaceholders
)

// MailTemplatePath - the edit page of the email key on the Mail page (a
// module's settings linking to its email's text).
func MailTemplatePath(key string) string { return entity.MailTemplatePath(key) }

// AddMail adds the module's email to the site's (the Mail page, Deps.Mail's
// Send): listed while the module is on. Called once, from Wire.
func AddMail(d MailDef) { service.RegisterMail(d) }

// MailTransport - a way of sending the site's email besides the engine's
// SMTP (an HTTP API of a mail service): its name, its module (it sends
// while the module is enabled), its title and settings on the Mail page,
// and New, building its sender from the saved settings.
type MailTransport = service.MailTransport

// MailTransportField - a setting of a MailTransport: shown on the Mail
// page (a secret never shown back), set from the environment by Env.
type MailTransportField = service.MailTransportField

// MailSender - what a MailTransport's New gives: it sends a message.
type MailSender = mailsender.EmailSender

// AddMailTransport adds a way of sending email, from the module's init. A
// mistake in it (a name not allowed or taken, no New) panics at startup.
func AddMailTransport(t MailTransport) { service.RegisterMailTransport(t) }

// AddDashboardBlock adds the module's block to the admin's Overview
// (shown while the module is enabled, to whom has b.Permission). Called
// once, from Wire.
func AddDashboardBlock(b DashboardBlock) { service.RegisterDashboardBlock(b) }

// OnNotFound adds the module's hook told of every page request answered
// 404 - its path (no query) and referrer; a redirected path or a blocked
// request never gets there. It runs in the request after the answer:
// quick, its errors its own. Called once, from Wire.
func OnNotFound(module string, fn func(ctx context.Context, path, referrer string)) {
	service.OnNotFound(func(ctx context.Context, path, referrer string) {
		if service.ModuleEnabled(module) {
			fn(ctx, path, referrer)
		}
	})
}

// ActionActorFrom - who acts in ctx: the signed-in user and the address
// (zero - nobody known).
func ActionActorFrom(ctx context.Context) ActionActor { return audit.ActorFrom(ctx) }

// RecordAction records what the module did for the action log: who did
// it (the request's user and address) comes from ctx. Without an action
// log it costs nothing.
func RecordAction(ctx context.Context, a Action) { audit.Record(ctx, a) }

// RecordNodeCreated records for the action log that the module's own save
// created the content n ("Content created"). The core's content form
// records it itself; a module type saved by the module's code calls it.
func RecordNodeCreated(ctx context.Context, n Node) { service.AuditNodeCreated(ctx, n) }

// RecordNodeSaved records what the module's own save changed of the
// content (before and after it): published or unpublished, a new author,
// a new address - as the core's content form does.
func RecordNodeSaved(ctx context.Context, before, after Node) {
	service.AuditNodeChange(ctx, before, after)
}

// NodeActionObject - the content n as the object of a module's action
// (RecordAction): the action shows in the content's log too.
func NodeActionObject(n Node) ActionObject { return service.AuditNodeObject(n) }

// AddActionGroup registers a group of the module's actions (Group.Module -
// the module) and AddActionType its actions. Called once, from Wire.
func AddActionGroup(g ActionGroup) { audit.RegisterGroup(g) }

// AddActionType - see AddActionGroup.
func AddActionType(t ActionType) { audit.RegisterType(t) }

// ActionGroups - the groups of actions (the core's first), ActionTypeOf -
// an action type by name: for an action log's pages.
func ActionGroups() []ActionGroup { return audit.Groups() }

// ActionTypeOf - see ActionGroups.
func ActionTypeOf(name string) (ActionType, bool) { return audit.TypeOf(name) }

// OnAction adds the module's listener of the recorded actions - the
// action log's; it runs in the acting request (quick, its errors its
// own). Called once, from Wire; module "" - always (a test's).
func OnAction(module string, fn func(ctx context.Context, e ActionEvent)) {
	audit.OnAction(func(ctx context.Context, e ActionEvent) {
		if service.ModuleEnabled(module) {
			fn(ctx, e)
		}
	})
}

// TextFilter - a module's processing of HTML text when it's shown,
// turned on per text format (the format page's "Processing when shown"):
// Name, Module, Label, Description, Apply(ctx, html) - ctx carries the
// page's language, called on every show, so cheap and without network -
// and an optional TinyMCE button the formats' editors get (EditorButton,
// EditorPlugin - the module's script registering it, static/<name>.js).
type TextFilter = service.TextFilter

// AddTextFilter registers the module's text filter. Called once, from
// Wire.
func AddTextFilter(f TextFilter) { service.RegisterTextFilter(f) }

// FieldFormatter - a module's way of showing a field (a link field's
// videos, say), chosen per field on the content type's "Detail page" tab
// and per View field: Name (unique), Module, Label (i18n.N), FieldType,
// Render(ctx, value, layout) - the field's value (a link field's: Links)
// as HTML, cheap and without network, a part it can't show shown as the
// core does; Hosts - its sites, offered as a link field's allowed ones;
// EditorScript - its script (static/<name>.js) the content form loads for
// the fields it shows. Off with its module: the field shows as links.
type FieldFormatter = service.FieldFormatter

// AddFieldFormatter registers the module's field formatter. Called once,
// from Wire.
func AddFieldFormatter(f FieldFormatter) { service.RegisterFieldFormatter(f) }

// AddAdminMenuCount sets the number the module shows by the admin menu
// item at path (its own, its manifest's AdminMenu, or the core's - shown
// with the module off too, without the number then): what waits there,
// e.g. new reports; counted for each admin page while the item is shown,
// 0 - none; label (i18n.N, optional) - what it counts, where the admin's
// Overview lists it ("" - the item's label). Called once, from Wire.
func AddAdminMenuCount(module, path string, count func(ctx context.Context) (int, error), label ...string) {
	service.RegisterAdminMenuCount(path, func(ctx context.Context) (int, error) {
		if !service.ModuleEnabled(module) {
			return 0, nil
		}
		return count(ctx)
	}, label...)
}

// AddJob registers a background job of module: the scheduler runs it
// every j.Every while the module is enabled. Called once, from Wire.
func AddJob(module string, j Job) {
	j.Module = module
	service.RegisterJob(j)
}

// AddTermPage registers the module's part of term pages: fn's result is
// the data of its templates there (.moduleTerm.<module>, e.g. its
// "term_actions_<module>"). Called once, from Wire; it runs only while
// the module is enabled.
func AddTermPage(module string, fn func(ctx context.Context, term TaxonomyTerm) any) {
	interfaces.AddTermPage(module, fn)
}

// SafeRedirectPath - destination when it is a path of this site (a
// "back" link of a form), else "".
func SafeRedirectPath(destination string) string { return service.SafeRedirectPath(destination) }

// ContextUserID - the signed-in user of the request ctx belongs to (a
// handler's r.Context(), a node or term page's hook: AddNodePage,
// AddTermPage); false - a guest.
func ContextUserID(ctx context.Context) (int, bool) { return middleware.GetUserIDFromContext(ctx) }

// OnNodePublished registers fn, called when a content item becomes public
// (published and not on moderation): created so, or by its form, its
// publishing, the bulk actions or the schedule - while module is enabled.
// It runs in the change's transaction; its error is logged. Called once,
// from Wire.
func OnNodePublished(module string, fn func(ctx context.Context, node Node) error) {
	service.RegisterNodePublished(module, fn)
}

// AddSitemapPaths registers fn adding the module's pages to the sitemap:
// called once, from Wire.
func AddSitemapPaths(module string, fn func(ctx context.Context) ([]string, error)) {
	interfaces.AddSitemapPaths(func(ctx context.Context) ([]string, error) {
		if !service.ModuleEnabled(module) {
			return nil, nil
		}
		return fn(ctx)
	})
}

// AddPathOwner registers answers, telling whether the module answers a
// path of its own (no leading "/"; a feed's address): a View page or
// another module's page doesn't take it (PathOwner). Called once, from
// Wire, under the module's name.
func AddPathOwner(module string, answers func(ctx context.Context, path string) (bool, error)) {
	interfaces.AddPathOwner(module, answers)
}

// PathOwner - what answers path on the site besides the module ("" -
// nothing: the address is free for it): the engine's routes ("routes"),
// content ("content"), a View page ("views"), a term page ("terms") or
// another module's page (that module's name).
func PathOwner(ctx context.Context, path, module string) (string, error) {
	return interfaces.PathOwner(ctx, path, module)
}

// FileIDs - the files of an image or file field's value: one, or several
// in order for a field with several images (FieldInstanceDef.Multiple).
func FileIDs(v any) []int { return entity.FileIDs(v) }

// FirstFileID - the first file of an image or file field's value (0 -
// none): where one picture stands for the content.
func FirstFileID(v any) int { return entity.FirstFileID(v) }

// NodeTypeEnabled - whether content of nodeType is available: false for
// a type of a disabled module.
func NodeTypeEnabled(nodeType string) bool { return service.NodeTypeEnabled(nodeType) }

// DeletedUser - the author shown for content whose account is gone.
func DeletedUser() *User { return dao.DeletedUserPlaceholder() }

// PermAdministerComments - the core's permission to moderate comments.
const PermAdministerComments = service.PermAdministerComments

// The comments' states (Comment.Status) and the reason of one held after
// visitors' complaints (Comments.Hold; CommentReason makes
// "reported:<count>").
const (
	CommentPublished      = entity.CommentPublished
	CommentPending        = entity.CommentPending
	CommentSpam           = entity.CommentSpam
	CommentReasonReported = entity.CommentReasonReported
)

// CommentReason - a hold reason with its detail ("reported:3").
func CommentReason(code, detail string) string { return entity.CommentReason(code, detail) }

// PermEditAny - the permission to edit anyone's material of nodeType.
func PermEditAny(nodeType string) string { return service.PermEditAny(nodeType) }

// LogFallback logs err of a fallback the module goes on with (a default
// value, an empty list): debug for a missing record, else warn.
func LogFallback(logger *slog.Logger, err error, msg string, args ...any) {
	service.LogFallback(logger, err, msg, args...)
}

// Rebind - a hand-written query's ? placeholders in the site's
// database's own (a module's SQL on Deps.DB).
func Rebind(query string) string { return database.Rebind(query) }

// A module's sort criteria, filters and fields in content Views: a
// View keeps one by its Key ("<module>.<name>", at most 32 characters),
// the module computes the values for a page's nodes at once.
type (
	// ViewSort - a sort criterion: each node's number (Values), its
	// settings in a View (Params: a threshold).
	ViewSort = entity.ViewModuleSort
	// ViewFilter - a number a View filters by: equal, at least, at most,
	// has one or not.
	ViewFilter = entity.ViewModuleFilter
	// ViewOption - a value of a choice filter (ViewFilter.Options).
	ViewOption = entity.ViewModuleOption
	// ViewField - a value shown in a View's rows: each node's markup
	// (Render), its settings in a View (Params: how it is shown).
	ViewField = entity.ViewModuleField
	// ViewParam - a setting of a criterion or a field in a View: the
	// kinds of a plugin block's setting (PluginBlockSetting*).
	ViewParam = entity.ViewModuleParam
)

// AddViewSort registers the module's sort criterion: called once, from
// Wire. It is offered and applied while the module is on.
func AddViewSort(s ViewSort) { views.AddModuleSort(s) }

// AddViewFilter registers the module's filter: called once, from Wire.
func AddViewFilter(f ViewFilter) { views.AddModuleFilter(f) }

// AddViewField registers the module's field: called once, from Wire.
func AddViewField(f ViewField) { views.AddModuleField(f) }

// AddContentTypeSettings registers the module's section of the content
// type form: called once, from Wire.
func AddContentTypeSettings(s ContentTypeSettings) { interfaces.AddContentTypeSettings(s) }

// AddAccountSettingsSection registers the module's section of the
// "Account settings" page, rendered by its template
// "account_settings_<module>": called once, from Wire.
func AddAccountSettingsSection(s AccountSettingsSection) { interfaces.AddAccountSettingsSection(s) }

// AddUserSettingsSection registers the module's section of the profile
// form, rendered by its template "user_settings_<module>": called once,
// from Wire.
func AddUserSettingsSection(s UserSettingsSection) { interfaces.AddUserSettingsSection(s) }

// AddNodeSettings registers the module's section of the material form:
// called once, from Wire.
func AddNodeSettings(s NodeSettings) { interfaces.AddNodeSettings(s) }

// IsDuplicateKey - err is a unique key's violation.
func IsDuplicateKey(err error) bool { return service.IsDuplicateKeyErr(err) }

// TruncateMeta - s cut for a meta description of at most n characters.
func TruncateMeta(s string, n int) string { return interfaces.TruncateMeta(s, n) }

// NodeMetaFromPageSEO - the form fields of a material's page SEO.
func NodeMetaFromPageSEO(ps PageSEO) NodeMetaFields { return form.NodeMetaFromPageSEO(ps) }

// InTx runs fn in a transaction, the core's unit of work on db (inside
// one, fn joins it): the core's repositories called with fn's ctx write
// in it, and so does the module's own client opened with Driver(ctx, db).
func InTx(ctx context.Context, db *sql.DB, fn func(ctx context.Context) error) error {
	return database.NewUnitOfWork(database.ClientFor(db)).Do(ctx, fn)
}

// Driver - the module's own ent client's driver for ctx (its generated
// NewClient(Driver(…))): the context's transaction (InTx), else db.
func Driver(ctx context.Context, db *sql.DB) dialect.Driver {
	if tx, ok := database.TxDriver(ctx); ok {
		return tx
	}
	return entsql.OpenDB(database.Dialect(), db)
}

// AdminNodesSortTitle - AdminNodesFilter.SortBy: by title.
const AdminNodesSortTitle = entity.AdminNodesSortTitle

// ErrEntityNotFound - a repository's answer for a missing record
// (IsNotFound; schemakit.ErrEntityNotFound for a module's generated ent).
var ErrEntityNotFound = entity.ErrEntityNotFound

// IsNotFound - err says the record is missing (ErrEntityNotFound, the
// module's generated ent's not-found error, wrapped or not): a handler
// answers 404 for it (RenderLoadFailure does).
func IsNotFound(err error) bool { return service.IsNotFound(err) }

// MaxMetaDesc - the longest meta description, in characters.
const MaxMetaDesc = entity.MaxMetaDesc

// The module's controllers build on the core's base controller: rendering,
// the response helpers, the logging of a failure (docs/contributing/architecture.md).
type (
	// BaseController - what a module's controller embeds.
	BaseController = interfaces.BaseController
	// BaseDeps - what it is built from (Deps.Base).
	BaseDeps = interfaces.BaseDeps
	// CaptchaService - the site's reCAPTCHA (Captcha).
	CaptchaService = service.CaptchaService
)

// PageSEOForList - a list page's title and meta tags (its canonical
// address path).
func PageSEOForList(title, path string) PageSEO { return interfaces.PageSEOForList(title, path) }

// PageSEOForPaginatedList - PageSEOForList for page (of a paginated
// list) with the list's query.
func PageSEOForPaginatedList(title, path string, page int, query url.Values) PageSEO {
	return interfaces.PageSEOForPaginatedList(title, path, page, query)
}

// The parts a module adds to the core's pages.
type (
	// NodeLink - a link a module adds to a material's page.
	NodeLink = interfaces.NodeLink
	// NodePageParts - what a module adds to a material's page
	// (AddNodePage).
	NodePageParts = interfaces.NodePageParts
)

// AddNodePage registers fn adding the module's parts to a material's page
// (links, actions): called once, from Wire.
func AddNodePage(module string, fn func(ctx context.Context, node Node) NodePageParts) {
	interfaces.AddNodePage(module, fn)
}

// AddCreateLinks registers fn adding the module's items to the "Create"
// menu of the user: called once, from Wire.
func AddCreateLinks(module string, fn func(ctx context.Context, user *User, auth AuthService) []CreatableMaterialLink) {
	interfaces.AddCreateLinks(func(ctx context.Context, user *User, auth service.AuthService) []CreatableMaterialLink {
		if !service.ModuleEnabled(module) {
			return nil
		}
		return fn(ctx, user, auth)
	})
}

// RequestThemeKey - the data key of the theme a fragment renders in
// (RenderNamed), RequestTheme its value; RequestLanguageKey and
// RequestLocationKey - of its language and its page's address.
const (
	RequestThemeKey    = service.RequestThemeKey
	RequestLanguageKey = service.RequestLanguageKey
	RequestLocationKey = service.RequestLocationKey
)

// RequestTheme - the theme of the request's render (a previewed one), "" -
// the active theme.
func RequestTheme(ctx context.Context) string { return service.RequestTheme(ctx) }

// AuthMiddleware - the user signed in by the session (Routes.Store), for
// the module's routes that need to know them.
func AuthMiddleware(store *sessions.CookieStore) func(http.Handler) http.Handler {
	return middleware.AuthMiddleware(store)
}

// AuthJSONMiddleware - AuthMiddleware for a JSON endpoint (a guest gets a
// JSON error, not the login page).
func AuthJSONMiddleware(store *sessions.CookieStore) func(http.Handler) http.Handler {
	return middleware.AuthJSONMiddleware(store)
}

// OptionalAuthMiddleware - the user signed in by the session when there
// is one; a guest goes on.
func OptionalAuthMiddleware(store *sessions.CookieStore) func(http.Handler) http.Handler {
	return middleware.OptionalAuthMiddleware(store)
}

// RequestNode - the material the core's node access check put in the
// request (a module's content type pages).
func RequestNode(r *http.Request) (*Node, bool) { return middleware.GetNode(r) }

// RequestUser - the signed-in user the core's access check put in the
// request with the material (RequestNode: a module's content type pages);
// elsewhere - UserID after AuthMiddleware, the user - the controller's
// GetCurrentUser.
func RequestUser(r *http.Request) (*User, bool) { return middleware.GetUser(r) }

// OrderRandom - an ent order option shuffling the rows, in the site's
// database's own way.
func OrderRandom(s *entsql.Selector) { database.OrderRandom(s) }

// NewBaseController - a module's controller's base from d (Deps.Base).
func NewBaseController(d BaseDeps) *BaseController { return interfaces.NewBaseController(d) }

// AddPageData registers fn adding the module's data to every page the
// site renders (e.g. its form on every page): called once, from Wire.
func AddPageData(module string, fn func(r *http.Request, data map[string]any)) {
	interfaces.AddPageData(func(r *http.Request, data map[string]any) {
		if service.ModuleEnabled(module) {
			fn(r, data)
		}
	})
}

// AddUserMenuCount adds n to the number on the user menu's toggle
// (data["userMenuCount"], e.g. the module's unread items), from the
// module's AddPageData.
func AddUserMenuCount(data map[string]any, n int) { interfaces.AddUserMenuCount(data, n) }

// ProfileTab - a module's tab of the user profile (/me/<Path>,
// /<profile>/<Path>): who may open it, its badge, its data; its template
// is "profile_tab_<module>".
type ProfileTab = interfaces.ProfileTab

// AddProfileTab registers the module's profile tab: called once, from
// Wire. It is shown only while the module is enabled.
func AddProfileTab(t ProfileTab) { interfaces.AddProfileTab(t) }

// Captcha - the reCAPTCHA in effect (nil - none set).
func Captcha() *CaptchaService { return interfaces.Captcha() }

// CheckBotTrap - an error when r's form looks sent by a bot: the site's
// bot trap (its fields - the template's {{botTrap}} in the form) caught
// it; nil when it passes or the trap is off.
func CheckBotTrap(r *http.Request) error { return service.CheckBotTrap(r) }

// MailBaseURL - the site's address for links in e-mails.
func MailBaseURL() string { return interfaces.MailBaseURL() }

// UserMailTokens - the tokens of an account e-mail to u ([site:…],
// [user:…], mailtemplate) under the site's address baseURL.
func UserMailTokens(baseURL string, site *SiteInformation, u *User) mailtemplate.TokenData {
	return accounts.MailTokenData(baseURL, site, u, "", "", "")
}

// UserStatusActive - User.Status of an active account (not blocked,
// not awaiting approval).
const UserStatusActive = entity.UserStatusActive

// ModuleEnabled - whether the module name is on (the Modules page).
func ModuleEnabled(name string) bool { return service.ModuleEnabled(name) }

// DBDialect - the database the site runs on (dialect.MySQL,
// dialect.Postgres): a module's own ent client opens its driver on
// Deps.DB with it.
func DBDialect() string { return database.Dialect() }

// SiteLanguage - a language of the site besides its default one: its
// code and its addresses' prefix (SiteInformation.Languages).
type SiteLanguage = entity.SiteLanguage

// SetLanguagesSwitch makes the site's languages besides the default one
// in use - their prefixes, their addresses - while module is enabled:
// called once, from the languages module's Wire; none set (module "") -
// one language.
func SetLanguagesSwitch(module string) {
	if module == "" {
		entity.SetLanguagesSwitch(nil)
		return
	}
	entity.SetLanguagesSwitch(func() bool { return service.ModuleEnabled(module) })
}

// SiteLanguages - the site's default language and the others in use.
func SiteLanguages() (string, []SiteLanguage) { return entity.SiteLanguages() }

// ApplySiteLanguages makes def the running site's default language and
// others its other languages - once they are saved
// (SiteInformationRepository.Update, SetLanguages).
func ApplySiteLanguages(def string, others []SiteLanguage) {
	i18n.SetDefault(def)
	entity.SetSiteLanguages(i18n.Default(), others)
}

// MoveDefaultLanguage - others once newDef becomes the default language
// instead of oldDef (the old one keeps its addresses under its code).
func MoveDefaultLanguage(others []SiteLanguage, oldDef, newDef string) []SiteLanguage {
	return entity.MoveDefaultLanguage(others, oldDef, newDef)
}

// LocalPath - path in code's addresses: with its prefix, as is for the
// default language.
func LocalPath(code, path string) string { return entity.LocalPath(code, path) }

// CopyNode creates a copy of the node nodeID (an admin-created type's) by
// user: its fields' values, flags and view roles, titled title ("" -
// "Copy of …"), unpublished, with an address of its own, in ctx's
// language (i18n.WithLang): its id. ErrNodeNotCopyable - a field set's
// item or a module's content type's.
func CopyNode(ctx context.Context, nodeID int, user *User, title string) (int, error) {
	return interfaces.CopyNode(ctx, nodeID, user, title)
}

// ErrNodeNotCopyable - CopyNode's node can't be copied.
var ErrNodeNotCopyable = interfaces.ErrNodeNotCopyable

// OnNodeFormSaved registers fn, called while module is enabled after a
// content item's own form (create or edit) saved it, in the save's
// transaction - its error fails the save; node has the ID and Type.
// Changes made elsewhere (bulk actions, publishing, the schedule, a
// copy) don't call it. Called once, from Wire.
func OnNodeFormSaved(module string, fn func(ctx context.Context, node Node, isNew bool) error) {
	interfaces.AddNodeFormSaved(module, fn)
}

// SetContentListFilter sets what narrows the content lists' nodes -
// every content View's, the search's - for a request, while module is
// enabled: fn(ctx) returns the condition the nodes' query adds (on its
// selector: s.C("langcode") is the nodes' column), nil - none for ctx.
// Called once, from Wire.
func SetContentListFilter(module string, fn func(ctx context.Context) func(*entsql.Selector)) {
	database.SetContentListFilter(func(ctx context.Context) func(*entsql.Selector) {
		if !service.ModuleEnabled(module) {
			return nil
		}
		return fn(ctx)
	})
}

// FormSection - a module's section of a core admin form: its template is
// "form_<Form>_<module>".
type FormSection = interfaces.FormSection

// The core's admin forms with module sections (FormSection.Form; ref -
// the thing's id): a menu link's, a custom block's, a block placement's.
const (
	FormMenuLink        = interfaces.FormMenuLink
	FormBlock           = interfaces.FormBlock
	FormBlockPlacement  = interfaces.FormBlockPlacement
	FormTerm            = interfaces.FormTerm
	FormSiteInformation = interfaces.FormSiteInformation
	FormView            = interfaces.FormView
	FormField           = interfaces.FormField
)

// AddFormSection registers the module's section of a core admin form:
// called once, from Wire; used only while the module is on.
func AddFormSection(s FormSection) { interfaces.AddFormSection(s) }

// SetTextTranslator sets what gives a text of the site (a menu link's
// title…: kind, ref) in a language (false - none, the text as entered),
// while module is enabled. Called once, from the languages module's Wire.
func SetTextTranslator(module string, fn func(lang, kind, ref string) (string, bool)) {
	entity.SetTextTranslator(func(lang, kind, ref string) (string, bool) {
		if !service.ModuleEnabled(module) {
			return "", false
		}
		return fn(lang, kind, ref)
	})
}

// SetLanguageVisibility sets what says whether a thing (kind, ref: a menu
// link…) shows on a page in a language, while module is enabled (off -
// in every language).
func SetLanguageVisibility(module string, fn func(kind, ref, lang string) bool) {
	entity.SetLanguageVisibility(func(kind, ref, lang string) bool {
		return !service.ModuleEnabled(module) || fn(kind, ref, lang)
	})
}

// SetNodeURLsInLanguage sets what gives content items' addresses in a
// language - their public versions in it - while module is enabled.
func SetNodeURLsInLanguage(module string, fn func(ctx context.Context, ids []int, lang string) map[int]string) {
	entity.SetNodeURLsInLanguage(func(ctx context.Context, ids []int, lang string) map[int]string {
		if !service.ModuleEnabled(module) {
			return nil
		}
		return fn(ctx, ids, lang)
	})
}

// The kinds of the site's texts and things shown by language
// (SetTextTranslator, SetLanguageVisibility).
const (
	TextMenuLinkTitle        = entity.TextMenuLinkTitle
	VisibilityMenuLink       = entity.VisibilityMenuLink
	TextBlockTitle           = entity.TextBlockTitle
	TextPlacementTitle       = entity.TextPlacementTitle
	VisibilityBlockPlacement = entity.VisibilityBlockPlacement
	TextTermName             = entity.TextTermName
	TextTermDescription      = entity.TextTermDescription
	TextSiteName             = entity.TextSiteName
	TextSiteSlogan           = entity.TextSiteSlogan
	TextViewTitle            = entity.TextViewTitle
	TextViewDescription      = entity.TextViewDescription
	TextFieldLabel           = entity.TextFieldLabel
)

// SiteTextRef - the ref of the site's own texts (TextSiteName…).
const SiteTextRef = entity.SiteTextRef

// NodeWorkflow - a module's editorial workflow publishing some content
// types' items.
type NodeWorkflow = interfaces.NodeWorkflow

// SetNodeWorkflow sets the site's editorial workflow: called once, from
// Wire; used only while the module is on.
func SetNodeWorkflow(w NodeWorkflow) { interfaces.SetNodeWorkflow(w) }

// ChangeNode runs change of the node nodeID by user (its publishing, say)
// in one transaction, recorded in its history - a revision, the action
// log, OnNodePublished - as the bulk actions' changes are.
func ChangeNode(ctx context.Context, nodeID int, user *User, change func(ctx context.Context) error) error {
	return interfaces.ChangeNode(ctx, nodeID, user, change)
}

// UserRoleNames - the roles of u (a guest's for nil; every signed-in
// user has RoleUser too).
func UserRoleNames(u *User) []string { return entity.UserRoleNames(u) }

// PermFullAccess - the permission of every permission (the
// administrator role's).
const PermFullAccess = service.PermFullAccess

// The site's roles (Deps.Roles).
type (
	// Role - a role: its name and permissions.
	Role = entity.Role
	// Roles - roles.
	Roles = entity.Roles
)

// RoleAnonymous - the guests' role; RoleUser - every signed-in user's.
const (
	RoleAnonymous = entity.RoleAnonymous
	RoleUser      = entity.RoleUser
)
