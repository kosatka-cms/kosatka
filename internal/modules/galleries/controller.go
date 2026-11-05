package galleries

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/schema"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// GalleryController serves node galleries: management pages, the JSON API and public gallery pages.
type GalleryController struct {
	*modulekit.BaseController
	fileUploadService modulekit.FileUploader
	nodeService       modulekit.NodesService
	galleryService    GalleryService
	fileRepo          modulekit.FileRepository
	// contentTypes resolves the node type's main content image field
	// (GalleryDeps).
	contentTypes modulekit.ContentTypeService
	// imageStyleService - the image styles a gallery's previews choose
	// from (GalleryDeps).
	imageStyleService modulekit.ImageStyleService
	// settings - the module's settings of content types and nodes
	// (GalleryDeps).
	settings SettingsRepository
}

// nodeEnabled - whether the node can have galleries (a failed check - no).
func (c *GalleryController) nodeEnabled(ctx context.Context, node *modulekit.Node) bool {
	if c.settings == nil {
		return false
	}
	on, err := c.settings.NodeEnabled(ctx, node.ID)
	if err != nil {
		c.Logger().Warn("galleries switch of a node", "error", err, "nodeID", node.ID)
	}
	return on
}

// nodeOpen - whether the node's galleries open for user: a node they may
// see (published, or theirs, or "view unpublished content") that can have
// galleries.
func (c *GalleryController) nodeOpen(ctx context.Context, node *modulekit.Node, user *modulekit.User) bool {
	return c.AuthService().CanViewNode(ctx, node, user) && c.nodeEnabled(ctx, node)
}

// GalleryDeps - what NewGalleryController needs besides the BaseDeps.
type GalleryDeps struct {
	FileUploadService modulekit.FileUploader
	FileRepo          modulekit.FileRepository
	NodeService       modulekit.NodesService
	GalleryService    GalleryService
	ContentTypes      modulekit.ContentTypeService
	Settings          SettingsRepository
	ImageStyles       modulekit.ImageStyleService
}

// NewGalleryController creates a GalleryController.
func NewGalleryController(deps modulekit.BaseDeps, d GalleryDeps) *GalleryController {
	base := modulekit.NewBaseController(deps)
	base.SetLogger(deps.Logger.With("component", "gallery.controller"))

	return &GalleryController{
		BaseController:    base,
		fileUploadService: d.FileUploadService,
		nodeService:       d.NodeService,
		galleryService:    d.GalleryService,
		fileRepo:          d.FileRepo,
		contentTypes:      d.ContentTypes,
		settings:          d.Settings,
		imageStyleService: d.ImageStyles,
	}
}

// GalleriesList displays the overview page with all galleries of a node.
func (c *GalleryController) GalleriesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	nodeIDStr := r.PathValue("id")
	nodeID, err := strconv.Atoi(nodeIDStr)
	if err != nil {
		c.Logger().Warn("invalid node ID", "id", nodeIDStr)
		c.RenderNotFound(w, r)
		return
	}

	node, err := c.nodeService.ByID(ctx, nodeID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error getting node by ID",
			"nodeID", nodeID,
		)
		return
	}

	user, _ := c.GetCurrentUser(r)
	// A node the visitor may not see doesn't show its galleries either.
	if !c.AuthService().CanViewNode(ctx, node, user) {
		c.RenderNotFound(w, r)
		return
	}
	// Enforce the node's galleries switch server-side, regardless of what
	// the UI shows.
	if !c.nodeEnabled(ctx, node) {
		c.RenderForbidden(w, r)
		return
	}

	var canEdit, canDelete bool
	if user != nil {
		canEdit = c.AuthService().CanEditNode(ctx, node, user)
		canDelete = c.AuthService().CanDeleteNode(ctx, node, user)
	}

	galleries, err := c.galleryService.ByNodeID(ctx, nodeID)
	if err != nil {
		c.LogFailure(err, "error loading galleries", "nodeID", nodeID)
		httputil.InternalServerError(w)
		return
	}

	imageStyles, err := c.imageStyleService.ImageStyles(ctx)
	if err != nil {
		c.LogFailure(err, "error loading image styles")
		httputil.InternalServerError(w)
		return
	}

	data := c.GetCommonData(r)
	data["node"] = node
	data["galleries"] = galleries
	data["canEdit"] = canEdit
	data["canDelete"] = canDelete
	data["imageStyles"] = imageStyles
	data["coverStyle"] = coverStyle(imageStyles)
	data["defaultSmallStyle"], data["defaultMediumStyle"] = schema.DefaultPreviewSmallStyle, schema.DefaultPreviewMediumStyle
	nodeTitle := c.NodeRepo().ProfileMaterialDisplayTitle(ctx, *node)
	data["title"] = i18n.T(ctx, "Galleries - @title", "@title", nodeTitle)
	c.SetNodeSubpageTrail(ctx, data, nodeWithTitle(*node, nodeTitle), i18n.T(ctx, "Galleries"))
	data["pageSEO"] = pageSEOForNodeGalleriesList(ctx, *node, nodeTitle, len(galleries), r.URL.Path)

	c.RenderOrFail(w, r, "node/galleries_list.html", data)
}

// EditGallery displays the edit page for a gallery.
func (c *GalleryController) EditGallery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	nodeIDStr := r.PathValue("id")
	galleryIDStr := r.PathValue("gallery_id")

	nodeID, err := strconv.Atoi(nodeIDStr)
	if err != nil {
		c.Logger().Warn("invalid node ID", "id", nodeIDStr)
		c.RenderNotFound(w, r)
		return
	}

	galleryID, err := strconv.Atoi(galleryIDStr)
	if err != nil {
		c.Logger().Warn("invalid gallery ID", "id", galleryIDStr)
		c.RenderNotFound(w, r)
		return
	}

	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil {
		c.Logger().Warn("user not authenticated")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	node, err := c.nodeService.ByID(ctx, nodeID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error getting node by ID",
			"nodeID", nodeID,
		)
		return
	}

	if !c.AuthService().CanEditNode(ctx, node, user) || !c.nodeEnabled(ctx, node) {
		c.Logger().Warn("user does not have permission to edit galleries",
			"userID", user.ID,
			"nodeID", nodeID,
		)
		c.RenderForbidden(w, r)
		return
	}

	gallery, err := c.galleryService.ByID(ctx, galleryID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error loading gallery",
			"galleryID", galleryID,
		)
		return
	}

	c.Logger().Info("loaded gallery for editing",
		"galleryID", galleryID,
		"title", gallery.Title,
		"description", gallery.Description,
		"itemsCount", len(gallery.Items),
	)

	if gallery.NodeID != nodeID {
		c.Logger().Warn("gallery does not belong to node",
			"galleryID", galleryID,
			"nodeID", nodeID,
		)
		c.RenderNotFound(w, r)
		return
	}

	imageStyles, err := c.imageStyleService.ImageStyles(ctx)
	if err != nil {
		c.LogFailure(err, "error loading image styles")
		httputil.InternalServerError(w)
		return
	}

	data := c.GetCommonData(r)
	data["node"] = node
	data["gallery"] = gallery
	data["imageStyles"] = imageStyles
	// The upload policy's image types: the file picker's and the hint's.
	if types, err := c.fileUploadService.ImageTypes(ctx); err != nil {
		modulekit.LogFallback(c.Logger(), err, "the upload policy's image types (any image offered)")
	} else {
		data["uploadAccept"] = strings.Join(types, ",")
		data["uploadFormats"] = modulekit.ImageFormatsLabel(types)
	}
	data["title"] = i18n.T(ctx, "Edit gallery: @title", "@title", gallery.GetTitle())
	// canSetPicture - the type has a "set as main image" field.
	if f, err := c.galleriesPictureField(ctx, node); err != nil {
		c.LogFailure(err, "error loading galleries picture field", "nodeID", node.ID)
	} else {
		data["canSetPicture"] = f != nil
	}
	c.SetNodeSubpageTrail(ctx, data, *node, i18n.T(ctx, "Editing"), galleriesTrailItem(ctx, node.ID))

	// Add no-cache headers to prevent browser caching of the edit form
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	c.ApplyEditorPageSEOFromTitle(data, r)
	c.RenderOrFail(w, r, "node/edit_gallery.html", data)
}

// GalleryCreate creates a new gallery for a node (API endpoint).
func (c *GalleryController) GalleryCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	nodeIDStr := r.PathValue("id")
	nodeID, err := strconv.Atoi(nodeIDStr)
	if err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid node ID",
		})
		return
	}

	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil {
		httputil.SendJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "User not authenticated",
		})
		return
	}

	node, err := c.nodeService.ByID(ctx, nodeID)
	if err != nil {
		c.LogFailure(err, "loading the content", "nodeID", nodeID)
		httputil.SendJSON(w, http.StatusNotFound, map[string]any{
			"error": "Node not found",
		})
		return
	}

	if !c.AuthService().CanEditNode(ctx, node, user) {
		httputil.SendJSON(w, http.StatusForbidden, map[string]any{
			"error": "Permission denied",
		})
		return
	}

	// Enforce the node's galleries switch server-side, as in GalleriesList.
	if !c.nodeEnabled(ctx, node) {
		httputil.SendJSON(w, http.StatusForbidden, map[string]any{
			"error": "Galleries are not enabled for this material",
		})
		return
	}

	if err := r.ParseForm(); err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid request body",
		})
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")

	// Empty styles - the module's defaults (GalleryService.Create).
	input := GalleryCreateInput{
		NodeID:             nodeID,
		PreviewSmallStyle:  r.FormValue("preview_small_style"),
		PreviewMediumStyle: r.FormValue("preview_medium_style"),
		Title:              &title,
		Description:        &description,
		Weight:             0,
		Status:             true,
	}

	gallery, err := c.galleryService.Create(ctx, input)
	if err != nil {
		if errors.Is(err, ErrGalleryFieldTooLong) {
			httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
				"error": i18n.T(r.Context(), "Value too long: title up to 255 characters"),
			})
			return
		}
		if errors.Is(err, ErrGalleryStyle) {
			httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
				"error": i18n.T(r.Context(), "Select both preview styles."),
			})
			return
		}
		c.LogFailure(err, "error creating gallery", "nodeID", nodeID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to create gallery",
		})
		return
	}

	httputil.SendJSON(w, http.StatusCreated, map[string]any{
		"gallery": gallery,
	})
}

// GalleryUpdate updates gallery metadata (API endpoint).
func (c *GalleryController) GalleryUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	galleryIDStr := r.PathValue("id")
	galleryID, err := strconv.Atoi(galleryIDStr)
	if err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid gallery ID",
		})
		return
	}

	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil {
		httputil.SendJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "User not authenticated",
		})
		return
	}

	gallery, err := c.galleryService.ByID(ctx, galleryID)
	if err != nil {
		c.LogFailure(err, "loading the gallery", "galleryID", galleryID)
		httputil.SendJSON(w, http.StatusNotFound, map[string]any{
			"error": "Gallery not found",
		})
		return
	}

	node, err := c.nodeService.ByID(ctx, gallery.NodeID)
	if err != nil {
		c.LogFailure(err, "loading the content", "nodeID", gallery.NodeID)
		httputil.SendJSON(w, http.StatusNotFound, map[string]any{
			"error": "Node not found",
		})
		return
	}

	if !c.AuthService().CanEditNode(ctx, node, user) {
		httputil.SendJSON(w, http.StatusForbidden, map[string]any{
			"error": "Permission denied",
		})
		return
	}

	if err := r.ParseForm(); err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid request body",
		})
		return
	}

	input := GalleryUpdateInput{}

	// Update fields present in the form even if empty.
	if r.Form.Has("preview_small_style") {
		v := r.FormValue("preview_small_style")
		input.PreviewSmallStyle = &v
	}
	if r.Form.Has("preview_medium_style") {
		v := r.FormValue("preview_medium_style")
		input.PreviewMediumStyle = &v
	}
	if r.Form.Has("title") {
		title := r.FormValue("title")
		input.Title = &title
	}
	if r.Form.Has("description") {
		description := r.FormValue("description")
		input.Description = &description
	}

	c.Logger().Info("updating gallery",
		"galleryID", galleryID,
		"title", input.Title,
		"description", input.Description,
	)

	updatedGallery, err := c.galleryService.Update(ctx, galleryID, input)
	if err != nil {
		if errors.Is(err, ErrGalleryFieldTooLong) {
			httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
				"error": i18n.T(r.Context(), "Value too long: title up to 255 characters"),
			})
			return
		}
		if errors.Is(err, ErrGalleryStyle) {
			httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
				"error": i18n.T(r.Context(), "Select both preview styles."),
			})
			return
		}
		c.LogFailure(err, "error updating gallery", "galleryID", galleryID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to update gallery",
		})
		return
	}

	c.Logger().Info("gallery updated successfully",
		"galleryID", galleryID,
		"title", updatedGallery.Title,
		"description", updatedGallery.Description,
	)

	// Reload to return the gallery with all relations.
	reloadedGallery, err := c.galleryService.ByID(ctx, galleryID)
	if err != nil {
		c.Logger().Warn("error reloading gallery after update",
			"error", err,
			"galleryID", galleryID,
		)
		// Still return the updated gallery even if reload fails
		reloadedGallery = updatedGallery
	}

	httputil.SendJSON(w, http.StatusOK, map[string]any{
		"gallery": reloadedGallery,
	})
}

// ViewGallery displays the public gallery page.
func (c *GalleryController) ViewGallery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	galleryIDStr := r.PathValue("id")
	galleryID, err := strconv.Atoi(galleryIDStr)
	if err != nil {
		c.Logger().Warn("invalid gallery ID", "id", galleryIDStr)
		c.RenderNotFound(w, r)
		return
	}

	gallery, err := c.galleryService.ByID(ctx, galleryID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error loading gallery",
			"galleryID", galleryID,
		)
		return
	}

	if !gallery.Status {
		c.Logger().Warn("gallery not published",
			"galleryID", galleryID,
		)
		c.RenderNotFound(w, r)
		return
	}

	// The gallery opens as its node's galleries do (nodeOpen); the node is
	// also the page's breadcrumbs and context.
	node, err := c.nodeService.ByID(ctx, gallery.NodeID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error loading node for gallery", "nodeID", gallery.NodeID)
		return
	}
	user, _ := c.GetCurrentUser(r)
	if !c.nodeOpen(ctx, node, user) {
		c.RenderNotFound(w, r)
		return
	}

	var canEdit, canDelete bool
	if user != nil {
		canEdit = c.AuthService().CanEditNode(ctx, node, user)
		canDelete = c.AuthService().CanDeleteNode(ctx, node, user)
	}

	data := c.GetCommonData(r)
	data["gallery"] = gallery
	data["node"] = node
	data["canEdit"] = canEdit
	data["canDelete"] = canDelete

	title := gallery.GetTitle()
	if title == "" {
		title = i18n.T(ctx, "Image gallery")
	}
	data["title"] = title
	nodeTitle := c.NodeRepo().ProfileMaterialDisplayTitle(ctx, *node)
	data["pageSEO"] = pageSEOForGalleryView(ctx, *gallery, node, nodeTitle, r.URL.Path)
	c.SetViewTrackNID(data, *node)
	c.SetNodeSubpageTrail(ctx, data, nodeWithTitle(*node, nodeTitle), title, galleriesTrailItem(ctx, node.ID))

	c.RenderOrFail(w, r, "gallery/view.html", data)
}

// GalleryDeleteConfirm displays the gallery delete confirmation page.
func (c *GalleryController) GalleryDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	galleryIDStr := r.PathValue("gallery_id")
	galleryID, err := strconv.Atoi(galleryIDStr)
	if err != nil {
		c.Logger().Warn("invalid gallery ID", "id", galleryIDStr)
		c.RenderNotFound(w, r)
		return
	}

	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil {
		c.Logger().Warn("user not authenticated")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	gallery, err := c.galleryService.ByID(ctx, galleryID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error loading gallery",
			"galleryID", galleryID,
		)
		return
	}

	node, err := c.nodeService.ByID(ctx, gallery.NodeID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error loading node",
			"nodeID", gallery.NodeID,
		)
		return
	}

	if !c.AuthService().CanDeleteNode(ctx, node, user) {
		c.Logger().Warn("user does not have permission to delete this gallery",
			"userID", user.ID,
			"galleryID", galleryID,
		)
		c.RenderForbidden(w, r)
		return
	}

	data := c.GetCommonData(r)
	data["gallery"] = gallery
	data["node"] = node
	data["title"] = i18n.T(ctx, "Confirm gallery deletion")
	c.SetNodeSubpageTrail(ctx, data, *node, i18n.T(ctx, "Gallery deletion"), galleriesTrailItem(ctx, node.ID))

	c.RenderOrFail(w, r, "gallery/delete_confirm.html", data)
}

// GalleryDeletePage deletes the gallery (form submission).
func (c *GalleryController) GalleryDeletePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	galleryIDStr := r.PathValue("gallery_id")
	galleryID, err := strconv.Atoi(galleryIDStr)
	if err != nil {
		c.Logger().Warn("invalid gallery ID", "id", galleryIDStr)
		c.RenderNotFound(w, r)
		return
	}

	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil {
		c.Logger().Warn("user not authenticated")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	gallery, err := c.galleryService.ByID(ctx, galleryID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error loading gallery",
			"galleryID", galleryID,
		)
		return
	}

	node, err := c.nodeService.ByID(ctx, gallery.NodeID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error loading node",
			"nodeID", gallery.NodeID,
		)
		return
	}

	if !c.AuthService().CanDeleteNode(ctx, node, user) {
		c.Logger().Warn("user does not have permission to delete this gallery",
			"userID", user.ID,
			"galleryID", galleryID,
		)
		c.RenderForbidden(w, r)
		return
	}

	err = c.galleryService.Delete(ctx, galleryID)
	if err != nil {
		c.LogFailure(err, "error deleting gallery", "galleryID", galleryID)
		httputil.InternalServerError(w)
		return
	}

	c.Logger().Info("gallery deleted successfully",
		"galleryID", galleryID,
		"nodeID", gallery.NodeID,
		"userID", user.ID,
	)

	http.Redirect(w, r, fmt.Sprintf("/node/%d/galleries", gallery.NodeID), http.StatusFound)
}

// galleriesTrailItem - a node's galleries page as a trail link.
func galleriesTrailItem(ctx context.Context, nodeID int) modulekit.BreadcrumbItem {
	return modulekit.BreadcrumbItem{URL: "/node/" + strconv.Itoa(nodeID) + "/galleries", Label: i18n.T(ctx, "Galleries")}
}

// nodeWithTitle - node with its display title (for the trail link).
func nodeWithTitle(node modulekit.Node, title string) modulekit.Node {
	if title != "" {
		node.Title = title
	}
	return node
}
