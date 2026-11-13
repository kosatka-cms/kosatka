package interfaces

import (
	"context"
	"html/template"
	"maps"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// DynamicContentController serves every admin-created content type through one
// shared handler set. It resolves the content type per request (from the
// "type" path value on create, from the node's Type otherwise) and builds the
// form adapter, service and repository for that type's fields on each request.
type DynamicContentController struct {
	// nodeDeleter - the single node-delete path (Delete below).
	nodeDeleter *service.NodeDeleteService
	// redirects - the old address of a re-slugged node (DynamicContentDeps).
	redirects service.RedirectService
	// paths - a node's slug by its type's path pattern (DynamicContentDeps).
	paths *service.NodePathService
	// roles, accessChanged - DynamicContentDeps.Roles, AccessChanged.
	roles         func(ctx context.Context) (entity.Roles, error)
	accessChanged func()
	previewLinks  *service.PreviewLinks
	media         *service.MediaLibrary
	*BaseController
	contentTypeService service.ContentTypeService
	taxonomyService    service.TaxonomyService
	commentsService    service.CommentsService
	nodeService        service.NodesService
	nodeRepo           entity.NodesRepository
	fileUploadService  files.FileUploadService
	fileRepo           entity.FileRepository
	uow                entity.UnitOfWork
	revisions          *service.NodeRevisionService
	// dynamicRepoFactory builds the repository for one content type's fields;
	// injected as a func so this package stays storage-agnostic (see
	// database.NewDynamicContentRepositoryFactory).
	dynamicRepoFactory func(entity.ContentTypeDef, entity.FieldInstanceDefs) entity.DynamicContentRepository
	// viewService backs ViewPageFallback, the catch-all handler for a View's
	// own url_path.
	viewService views.Service
	// textFormatService backs resolveTextFormat (per-value text format
	// picker, see entity.TextFormatDef).
	textFormatService service.TextFormatService
	// contentTypeRepo - what views.BuildContentList reads (the
	// ContentTypeService interface is too narrow for it).
	contentTypeRepo entity.ContentTypeRepository
	// seo - title/description templates (DynamicContentDeps).
	seo *service.SEOService
	// teasers - items for theme-template Views (DynamicContentDeps).
	teasers *service.TeaserBuilder
	// commentSettings - whether a page has its comments in its HTML.
	commentSettings entity.CommentSettingsRepository
}

// DynamicContentDeps - what NewDynamicContentController needs besides the BaseDeps.
type DynamicContentDeps struct {
	ContentTypeService service.ContentTypeService
	TaxonomyService    service.TaxonomyService
	NodeService        service.NodesService
	FileUploadService  files.FileUploadService
	FileRepo           entity.FileRepository
	UnitOfWork         entity.UnitOfWork
	DynamicRepoFactory func(entity.ContentTypeDef, entity.FieldInstanceDefs) entity.DynamicContentRepository
	ViewService        views.Service
	TextFormatService  service.TextFormatService
	ContentTypeRepo    entity.ContentTypeRepository
	NodeDeleter        *service.NodeDeleteService
	Redirects          service.RedirectService
	NodePaths          *service.NodePathService
	Teasers            *service.TeaserBuilder
	SEO                *service.SEOService
	// Revisions - the history of the types that keep one.
	Revisions *service.NodeRevisionService
	// Roles - the site's roles, those a node's view may be given to
	// (entity.Node.ViewRoles); AccessChanged - called when a node's view
	// roles change (who views what read again, the cached blocks
	// rendered again).
	Roles         func(ctx context.Context) (entity.Roles, error)
	AccessChanged func()
	// PreviewLinks - the links showing an item to whoever has them.
	PreviewLinks *service.PreviewLinks
	// Media - the media library an image or file field picks from.
	Media *service.MediaLibrary
	// CommentSettings - whether a page has its comments in its HTML
	// (CommentSettings.LoadWithPage).
	CommentSettings entity.CommentSettingsRepository
}

// NewDynamicContentController creates a DynamicContentController on deps
// and d.
func NewDynamicContentController(deps BaseDeps, d DynamicContentDeps) *DynamicContentController {
	base := NewBaseController(deps)
	base.logger = deps.Logger.With("component", "dynamic_content.handler")
	return &DynamicContentController{
		BaseController:     base,
		roles:              d.Roles,
		accessChanged:      d.AccessChanged,
		previewLinks:       d.PreviewLinks,
		media:              d.Media,
		commentSettings:    d.CommentSettings,
		nodeDeleter:        d.NodeDeleter,
		contentTypeService: d.ContentTypeService,
		taxonomyService:    d.TaxonomyService,
		commentsService:    deps.Comments,
		nodeService:        d.NodeService,
		nodeRepo:           deps.Nodes,
		fileUploadService:  d.FileUploadService,
		fileRepo:           d.FileRepo,
		uow:                d.UnitOfWork,
		revisions:          d.Revisions,
		dynamicRepoFactory: d.DynamicRepoFactory,
		viewService:        d.ViewService,
		textFormatService:  d.TextFormatService,
		contentTypeRepo:    d.ContentTypeRepo,
		redirects:          d.Redirects,
		paths:              d.NodePaths,
		teasers:            d.Teasers,
		seo:                d.SEO,
	}
}

// resolveByMachineName loads a content type and its ordered fields by machine
// name (used on create, before a node exists).
func (c *DynamicContentController) resolveByMachineName(ctx context.Context, machineName string) (*entity.ContentTypeDef, entity.FieldInstanceDefs, error) {
	def, err := c.contentTypeService.ContentTypeByMachineName(ctx, machineName)
	if err != nil {
		return nil, nil, err
	}
	fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, def.ID)
	if err != nil {
		return nil, nil, err
	}
	return def, fields.SortBySortOrder(), nil
}

// dynamicService builds the DynamicContentService for one content type,
// wrapping the repository the injected factory returns.
func (c *DynamicContentController) dynamicService(def entity.ContentTypeDef, fields entity.FieldInstanceDefs) service.DynamicContentService {
	repo := c.dynamicRepoFactory(def, fields)
	return service.NewDynamicContentService(repo)
}

// Detail renders the public detail page for a node of an admin-created type
// (web/templates/dynamic/page.html).
func (c *DynamicContentController) Detail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, ok := middleware.GetNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}

	def, fields, err := c.resolveByMachineName(ctx, node.Type.ToString())
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the content type", "type", node.Type.ToString())
		return
	}

	// A preview link (Preview) shows the item whoever views it: loaded
	// whatever its type's and its own view roles.
	preview := isPreview(ctx)
	loadCtx := ctx
	if preview {
		loadCtx = entity.WithAllNodeTypes(ctx)
	}
	dc, err := c.dynamicService(*def, fields).ByNodeID(loadCtx, node.ID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error getting dynamic content for detail view", "nodeID", node.ID)
		return
	}

	user, _ := c.GetCurrentUser(r)
	if !preview && !c.RequireVisibleOrModerator(w, r, &dc.Node, user) {
		return
	}
	// If the type has a redirect field (ContentTypeDef.RedirectField) with a
	// valid target, redirect instead of rendering.
	if target := redirectTarget(dc.FieldValueByMachineName(def.RedirectField)); def.RedirectField != "" && target != "" {
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	c.renderItem(w, r, def, fields, dc, user, nil)
}

// renderItem renders the page of the content item dc
// (web/templates/dynamic/page.html) for user. preview - the item as its
// form would save it (FormPreview), nil - as saved: a preview has no
// actions, no comments and no view counted, and its strip over the page
// (UnsavedPreview).
func (c *DynamicContentController) renderItem(w http.ResponseWriter, r *http.Request, def *entity.ContentTypeDef, fields entity.FieldInstanceDefs, dc *entity.DynamicContent, user *entity.User, preview *UnsavedPreview) {
	ctx := r.Context()
	var canEdit, canDelete, canViewRevisions, canClone bool
	if preview == nil {
		canEdit, canDelete = c.EditDeletePermissions(ctx, &dc.Node, user)
		// The node's history: a type keeping one, for who may see it.
		canViewRevisions = canEdit && def.Revisions && c.MayViewRevisions(ctx, &dc.Node, user)
		// A copy (Clone): who may copy content, edit it and create its
		// type's content.
		canClone = canEdit && user != nil && !def.Embedded && c.mayCopy(ctx, dc.Node.Type, user)
	}

	// Title and description from SEOService templates, over the built-in
	// fallback.
	pageSEO := PageSEO(dc)
	var schemaJSONLD template.JS
	if c.seo != nil {
		pageSEO = c.seo.Apply(ctx, *dc, pageSEO)
		// The type's schema.org markup (Configuration → SEO).
		schemaJSONLD = c.seo.NodeJSONLD(ctx, *dc, pageSEO)
	}

	// The modules' parts of this node's page (AddNodePage): their
	// templates' data, links (e.g. its quotes) and attached blocks, and
	// a module showing its comments in place of the core's.
	moduleParts := moduleNodePage(ctx, dc.Node)

	// displayFields: fields in DisplayOrder (the edit form uses SortOrder).
	// The fields' labels in the page's language.
	displayFields := entity.LocalizeFieldLabels(i18n.FromContext(ctx), dc.Fields.SortByDisplayOrder())
	imageFieldItems := c.imageFieldItems(ctx, displayFields, dc.Values, func(f entity.FieldInstanceDef) string {
		return f.DetailImageStyle
	})
	imageFieldURLs, imageFieldAlts := map[int]string{}, map[int]string{}
	for id, items := range imageFieldItems {
		imageFieldURLs[id], imageFieldAlts[id] = items[0].URL, items[0].Alt
	}
	// fileFieldItems - every file of a file field, in order;
	// fileFieldLinks - its first.
	fileFieldItems := c.fileFieldItems(ctx, displayFields, dc.Values)
	fileFieldLinks := map[int]fileLinkInfo{}
	for id, items := range fileFieldItems {
		fileFieldLinks[id] = items[0]
	}
	taxonomyFieldTerms := c.taxonomyFieldTerms(ctx, displayFields, dc.Values)
	userFieldUsers := c.userFieldUsers(ctx, displayFields, dc.Values)
	referenceFieldNodes := c.publicReferenceNodes(ctx, displayFields, dc.Values)
	// displaySections: one block per section, in display order (see
	// displaySectionsOf); a reference field's line - its public nodes.
	displaySections := displaySectionsOf(displayFields, dc, taxonomyFieldTerms, imageFieldURLs, referenceFieldNodes)
	// LCP hint - the first image shown in the standard field output.
	for _, sec := range displaySections {
		if sec.Image != nil {
			if items := imageFieldItems[sec.Image.ID]; len(items) > 0 {
				pageSEO = withPreloadImage(pageSEO, items[0].Picture)
			}
			break
		}
	}

	data := c.GetCommonDataWithUser(r, user)
	maps.Copy(data, map[string]any{
		"item":               dc,
		"contentType":        def,
		"fieldImages":        c.fieldImages(ctx, dc),
		"imageFieldURLs":     imageFieldURLs,
		"imageFieldAlts":     imageFieldAlts,
		"imageFieldItems":    imageFieldItems,
		"fileFieldLinks":     fileFieldLinks,
		"fileFieldItems":     fileFieldItems,
		"taxonomyFieldTerms": taxonomyFieldTerms,
		"userFieldUsers":     userFieldUsers,
		// referenceFieldNodes - the content the reference fields name,
		// the public only (field_line_reference links to it).
		"referenceFieldNodes": referenceFieldNodes,
		"displaySections":     displaySections,
		"blockViews":          c.blockViews(ctx, displayFields, dc.Values),
		"pageSEO":             pageSEO,
		"schemaJSONLD":        schemaJSONLD,
		"canEdit":             canEdit,
		"canDelete":           canDelete,
		// workflowManaged - a workflow publishes the item: no
		// publish/unpublish buttons of its own (NodeWorkflow).
		"workflowManaged":  NodeWorkflowFor(ctx, string(dc.Node.Type)) != nil,
		"canViewRevisions": canViewRevisions,
		"canClone":         canClone,
		"moduleNode":       moduleParts.data,
	})
	// A node configured as the 404/403 page reads as that page, not as
	// content: see errorPageType.
	if shown, status := errorPageType(def, dc.Node.ID); status != 0 {
		data["contentType"] = shown
		data["errorPage"] = status
	}
	if len(moduleParts.links) > 0 {
		data["moduleNodeLinks"] = moduleParts.links
	}
	// commentsModule - the module whose "comments_<module>" is shown in
	// place of the core's comments (comments.html).
	if moduleParts.comments != "" {
		data["commentsModule"] = moduleParts.comments
	}
	// Contextual View blocks (e.g. "more by this author"); which types show
	// them is configured via block placements.
	c.attachContextualViewBlocks(ctx, data, dc.Node)
	c.attachViewAttachments(ctx, data, dc.Node)
	// fieldValues/fieldTerms - every field's value and resolved terms by
	// machine_name, for theme templates (page/<type>.html) that lay a
	// type's page out by hand instead of through displaySections.
	// fieldFormats - the text format of each text value saved with one,
	// for processText (the modules' text filters).
	fieldValues := make(map[string]any, len(dc.Fields))
	fieldFormats := make(map[string]int)
	fieldTerms := make(map[string]entity.TaxonomyTerms)
	for _, f := range dc.Fields {
		if v, ok := dc.Values[f.ID]; ok {
			fieldValues[f.MachineName] = v
		}
		if id := dc.TextFormats[f.ID]; id > 0 {
			fieldFormats[f.MachineName] = id
		}
		if ts := taxonomyFieldTerms[f.ID]; len(ts) > 0 {
			fieldTerms[f.MachineName] = ts
		}
	}
	data["fieldValues"] = fieldValues
	data["fieldFormats"] = fieldFormats
	data["fieldTerms"] = fieldTerms
	// readingTime - the minutes reading the item takes, 0 - nothing to
	// read (entity.ReadingMinutesOf); commentCount - its published
	// comments while the core shows them (see nodeCommentCount).
	data["readingTime"] = entity.ReadingMinutesOf(dc.Fields, func(f entity.FieldInstanceDef) any { return dc.Values[f.ID] })
	if n, ok := c.nodeCommentCount(ctx, dc.Node, moduleParts.comments); ok {
		data["commentCount"] = n
	}
	// Breadcrumb trail; the last item is the node's title.
	trailNode := *dc
	trailNode.ContentType = *def
	c.setNodeTrail(ctx, data, trailNode, dc.Node.Title)
	if preview != nil {
		data["unsavedPreview"] = preview
		c.RenderOrFail(w, r, "dynamic/page.html", data)
		return
	}
	c.SetViewTrackNID(data, dc.Node)
	if moduleParts.comments == "" {
		c.attachComments(ctx, r, data, &dc.Node, user)
	}

	c.RenderOrFail(w, r, "dynamic/page.html", data)
}

// nodeCommentCount - the node's published comments; false while the core
// doesn't show them (hidden, a module's comments in their place) or
// they fail to count (logged).
func (c *DynamicContentController) nodeCommentCount(ctx context.Context, node entity.Node, commentsModule string) (int, bool) {
	if node.CommentMode == entity.CommentModeHidden || commentsModule != "" || c.commentsService == nil {
		return 0, false
	}
	n, err := c.commentsService.CountByNodeID(ctx, node.ID, entity.CommentPublished, false)
	if err != nil {
		c.LogFailure(err, "counting the item's comments (no count shown)", "node_id", node.ID)
		return 0, false
	}
	return n, true
}

// attachComments gives the page its comments in its HTML ("commentsList":
// the page ?page= asks for), unless they load by ajax
// (CommentSettings.LoadWithPage) or the node hides them; should they fail
// to load, by ajax too.
func (c *DynamicContentController) attachComments(ctx context.Context, r *http.Request, data map[string]any, node *entity.Node, user *entity.User) {
	if node.CommentMode == entity.CommentModeHidden || c.commentSettings == nil {
		return
	}
	settings, err := c.commentSettings.Get(ctx)
	if err != nil {
		c.LogFailure(err, "error loading the comment settings (the comments load by ajax)")
		return
	}
	if !settings.LoadWithPage {
		return
	}
	list, err := c.commentsList(ctx, r, node, user, c.GetPageNumber(r))
	if err != nil {
		c.LogFailure(err, "error loading the page's comments (they load by ajax)", "node_id", node.ID)
		return
	}
	data["commentsList"] = withRequestContext(ctx, list)
}

// errorPageType - for a node configured as an error page (Site information
// → 404/403 page), its type's settings as the page shows them (no author
// and dates line, whatever the type shows) and the status; for any other
// node def itself and 0. The page's data["errorPage"] (the status) lets a
// theme leave out more. def isn't changed (it may be shared).
func errorPageType(def *entity.ContentTypeDef, nodeID int) (*entity.ContentTypeDef, int) {
	status := service.ErrorPageStatus(nodeID)
	if status == 0 {
		return def, 0
	}
	shown := *def
	shown.ShowSubmitted = false
	return &shown, status
}

// slugTaken - the form's message when slug, typed for a node, is an
// address something else of the site answers (an engine route, a term's
// or a View's page, a module's): the node would hide it, or never be
// reached; "" - it is free (other nodes' are checked on save).
func (c *DynamicContentController) slugTaken(ctx context.Context, slug string) (string, error) {
	owner, err := PathOwner(ctx, slug, PathOwnerContent)
	if err != nil || owner == "" {
		return "", err
	}
	return i18n.T(ctx, "This address is taken by another page of the site."), nil
}

// nodeSlug - the slug a saved node gets (e: the node as it will be saved):
// by the type's pattern while it is automatic (PathAuto: the form's
// checkbox, or the node's own setting when the form had none; a new node
// without a typed slug too), else the typed one (an emptied one keeps the
// current slug). Sets e.Node.PathAuto. The pattern makes a node's address
// once: when it is created, or when its address set by hand is made
// automatic; a saved automatic address stays - renaming the node or
// changing its terms doesn't move it (an address typed instead does).
func (c *DynamicContentController) nodeSlug(ctx context.Context, def entity.ContentTypeDef, f *form.DynamicForm, e *entity.DynamicContent) (string, error) {
	typed := strings.Trim(f.Slug, "/ ")
	wasAuto := e.Node.ID != 0 && e.Node.PathAuto
	auto := e.Node.ID == 0 || e.Node.PathAuto
	if f.PathAutoShown {
		auto = f.PathAuto
	}
	if !auto && typed == "" && e.Node.ID == 0 {
		auto = true
	}
	e.Node.PathAuto = auto
	if !auto {
		if typed == "" {
			return e.Node.Slug, nil
		}
		return typed, nil
	}
	if wasAuto && e.Node.Slug != "" {
		return e.Node.Slug, nil
	}
	return c.paths.Path(ctx, def, *e)
}
