package galleries

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// errNoGalleriesPictureField - the node's type has no "set as main image"
// field (TypeSettings.PictureField).
var errNoGalleriesPictureField = errors.New("galleries picture field is not set for the content type")

// galleriesPictureField returns the image field that "set as main image"
// writes into for the node's type (TypeSettings.PictureField); nil if none.
func (c *GalleryController) galleriesPictureField(ctx context.Context, node *modulekit.Node) (*modulekit.FieldInstanceDef, error) {
	if c.contentTypes == nil || c.settings == nil {
		return nil, nil
	}
	settings, err := c.settings.TypeSettings(ctx, string(node.Type))
	if err != nil || settings.PictureField == "" {
		return nil, err
	}
	def, err := c.contentTypes.ContentTypeByMachineName(ctx, string(node.Type))
	if err != nil || def == nil {
		return nil, err
	}
	fields, err := c.contentTypes.FieldsByContentTypeID(ctx, def.ID)
	if err != nil {
		return nil, err
	}
	for i := range fields {
		if fields[i].FieldType == modulekit.FieldTypeImage && fields[i].MachineName == settings.PictureField {
			return &fields[i], nil
		}
	}
	return nil, nil
}

// setPictureField puts fileID into the type's "set as main image" field;
// a field with several images gets it first, its others kept after it (as
// many as the field's limit leaves room for).
func (c *GalleryController) setPictureField(ctx context.Context, node *modulekit.Node, fileID int) error {
	target, err := c.galleriesPictureField(ctx, node)
	if err != nil {
		return err
	}
	if target == nil {
		return errNoGalleriesPictureField
	}
	fields := modulekit.FieldInstanceDefs{*target}
	var value any = fileID
	if target.Multiple() {
		current, err := c.contentTypes.FieldValues(ctx, node.ID, fields)
		if err != nil {
			return fmt.Errorf("loading the main image field (node %d): %w", node.ID, err)
		}
		value = mainImageFirst(fileID, modulekit.FileIDs(current[target.ID]), target.MaxValues())
	}
	return c.contentTypes.SetFieldValues(ctx, node.ID, fields, map[int]any{target.ID: value}, nil)
}

// mainImageFirst - the images of a field with several with fileID first,
// the others after it in their order, at most limit of them (0 - no
// limit).
func mainImageFirst(fileID int, current []int, limit int) []int {
	ids := []int{fileID}
	for _, id := range current {
		if id != fileID && (limit == 0 || len(ids) < limit) {
			ids = append(ids, id)
		}
	}
	return ids
}

// GalleryAddItem adds an image to a gallery (API endpoint).
func (c *GalleryController) GalleryAddItem(w http.ResponseWriter, r *http.Request) {
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

	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid form data",
		})
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "No image file provided",
		})
		return
	}
	defer func() { _ = file.Close() }()

	// Store the file under the gallery's directory, checked by the
	// sitewide upload policy.
	uri, result, err := c.fileUploadService.UploadImage(ctx, file, header, filepath.Join("galleries", strconv.Itoa(galleryID)), modulekit.FieldUploadPolicy{})
	if err != nil {
		status, msg := galleryUploadErrorResponse(r.Context(), err)
		if status == http.StatusInternalServerError {
			c.LogFailure(err, "uploading a gallery image", "filename", header.Filename)
		}
		httputil.SendJSON(w, status, map[string]any{
			"error": msg,
		})
		return
	}

	uploadedFile, err := c.fileRepo.Create(ctx, uri, result.MimeType, result.Width, result.Height, result.SizeBytes, user.ID)
	if err != nil {
		c.LogFailure(err, "error creating file record")
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to create file record",
		})
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")

	// Reload the gallery to compute the next item weight (max + 1).
	galleryWithItems, err := c.galleryService.ByID(ctx, galleryID)
	if err != nil {
		c.LogFailure(err, "error reloading gallery", "galleryID", galleryID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to get gallery",
		})
		return
	}

	nextWeight := 0
	if len(galleryWithItems.Items) > 0 {
		for _, item := range galleryWithItems.Items {
			if item.Weight >= nextWeight {
				nextWeight = item.Weight + 1
			}
		}
	}

	c.Logger().Info("calculated next weight for new image",
		"galleryID", galleryID,
		"nextWeight", nextWeight,
		"currentItemsCount", len(galleryWithItems.Items),
	)

	input := GalleryItemCreateInput{
		GalleryID:   galleryID,
		FileID:      uploadedFile.ID,
		Title:       &title,
		Description: &description,
		Weight:      nextWeight,
		IsCover:     false,
	}

	item, err := c.galleryService.AddItem(ctx, input)
	if err != nil {
		if errors.Is(err, ErrGalleryFieldTooLong) {
			httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
				"error": i18n.T(r.Context(), "Value too long: title up to 255 characters"),
			})
			return
		}
		c.LogFailure(err, "error adding item to gallery", "galleryID", galleryID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to add item to gallery",
		})
		return
	}

	httputil.SendJSON(w, http.StatusCreated, map[string]any{
		"item": item,
	})
}

// authorizeGalleryItem loads the gallery item and checks that user may edit
// the node its gallery belongs to. On failure it writes the JSON error
// (404 or 403) and returns false.
func (c *GalleryController) authorizeGalleryItem(ctx context.Context, w http.ResponseWriter, user *modulekit.User, itemID int) (*GalleryItem, bool) {
	item, err := c.galleryService.GetItemByID(ctx, itemID)
	if err != nil || item == nil {
		c.LogFailure(err, "loading the gallery's image", "itemID", itemID)
		httputil.SendJSON(w, http.StatusNotFound, map[string]any{
			"error": "Gallery item not found",
		})
		return nil, false
	}
	gallery, err := c.galleryService.ByID(ctx, item.GalleryID)
	if err != nil || gallery == nil {
		c.LogFailure(err, "loading the gallery", "galleryID", item.GalleryID)
		httputil.SendJSON(w, http.StatusNotFound, map[string]any{
			"error": "Gallery not found",
		})
		return nil, false
	}
	node, err := c.nodeService.ByID(ctx, gallery.NodeID)
	if err != nil || node == nil {
		c.LogFailure(err, "loading the content", "nodeID", gallery.NodeID)
		httputil.SendJSON(w, http.StatusNotFound, map[string]any{
			"error": "Node not found",
		})
		return nil, false
	}
	if !c.AuthService().CanEditNode(ctx, node, user) {
		httputil.SendJSON(w, http.StatusForbidden, map[string]any{
			"error": "Permission denied",
		})
		return nil, false
	}
	return item, true
}

// GalleryRemoveItem removes an image from a gallery (API endpoint).
func (c *GalleryController) GalleryRemoveItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	itemIDStr := r.PathValue("id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid item ID",
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

	if _, ok := c.authorizeGalleryItem(ctx, w, user, itemID); !ok {
		return
	}

	err = c.galleryService.RemoveItem(ctx, itemID)
	if err != nil {
		c.LogFailure(err, "error removing item from gallery", "itemID", itemID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to remove item",
		})
		return
	}

	httputil.SendJSON(w, http.StatusOK, map[string]any{
		"message": "Item removed successfully",
	})
}

// GalleryReorder reorders images in a gallery (API endpoint).
func (c *GalleryController) GalleryReorder(w http.ResponseWriter, r *http.Request) {
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

	var payload struct {
		Items []GalleryItemReorder `json:"items"`
	}

	if err := httputil.DecodeJSON(r, &payload); err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid JSON body",
		})
		return
	}

	items, err := c.galleryService.GetItems(ctx, galleryID)
	if err != nil {
		c.LogFailure(err, "error loading gallery items", "galleryID", galleryID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to reorder items",
		})
		return
	}
	inGallery := make(map[int]bool, len(items))
	for _, it := range items {
		inGallery[it.ID] = true
	}
	for _, it := range payload.Items {
		if !inGallery[it.ID] {
			httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
				"error": "Item does not belong to this gallery",
			})
			return
		}
	}

	err = c.galleryService.ReorderItems(ctx, payload.Items)
	if err != nil {
		c.LogFailure(err, "error reordering gallery items", "galleryID", galleryID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to reorder items",
		})
		return
	}

	c.Logger().Info("gallery items reordered successfully",
		"galleryID", galleryID,
		"itemsCount", len(payload.Items),
	)

	httputil.SendJSON(w, http.StatusOK, map[string]any{
		"message": "Items reordered successfully",
	})
}

// GalleryUpdateItem updates gallery item metadata (API endpoint).
func (c *GalleryController) GalleryUpdateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	itemIDStr := r.PathValue("id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid item ID",
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

	if _, ok := c.authorizeGalleryItem(ctx, w, user, itemID); !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid request body",
		})
		return
	}

	input := GalleryItemUpdateInput{}

	if title := r.FormValue("title"); title != "" {
		input.Title = &title
	}
	if description := r.FormValue("description"); description != "" {
		input.Description = &description
	}

	item, err := c.galleryService.UpdateItem(ctx, itemID, input)
	if err != nil {
		if errors.Is(err, ErrGalleryFieldTooLong) {
			httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
				"error": i18n.T(r.Context(), "Value too long: title up to 255 characters"),
			})
			return
		}
		c.LogFailure(err, "error updating gallery item", "itemID", itemID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to update item",
		})
		return
	}

	httputil.SendJSON(w, http.StatusOK, map[string]any{
		"item": item,
	})
}

// GallerySetCover sets an image as the gallery cover (API endpoint).
func (c *GalleryController) GallerySetCover(w http.ResponseWriter, r *http.Request) {
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

	itemIDStr := r.FormValue("item_id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid item ID",
		})
		return
	}

	if item, err := c.galleryService.GetItemByID(ctx, itemID); err != nil || item == nil || item.GalleryID != galleryID {
		c.LogFailure(err, "loading the gallery's image", "itemID", itemID)
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Item does not belong to this gallery",
		})
		return
	}

	err = c.galleryService.SetCover(ctx, galleryID, itemID)
	if err != nil {
		c.LogFailure(err, "error setting gallery cover", "galleryID", galleryID, "itemID", itemID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to set cover",
		})
		return
	}

	httputil.SendJSON(w, http.StatusOK, map[string]any{
		"message": "Cover set successfully",
	})
}

// GallerySetNodeMainImage sets a gallery item as the node's main image (API endpoint).
func (c *GalleryController) GallerySetNodeMainImage(w http.ResponseWriter, r *http.Request) {
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

	itemIDStr := r.FormValue("item_id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Invalid item ID",
		})
		return
	}

	item, err := c.galleryService.GetItemByID(ctx, itemID)
	if err != nil {
		c.LogFailure(err, "error loading gallery item", "itemID", itemID)
		httputil.SendJSON(w, http.StatusNotFound, map[string]any{
			"error": "Gallery item not found",
		})
		return
	}

	if item.GalleryID != galleryID {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Item does not belong to this gallery",
		})
		return
	}

	if err := c.setPictureField(ctx, node, item.FileID); errors.Is(err, errNoGalleriesPictureField) {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": i18n.T(r.Context(), "No field for the \"Main content image\" is selected for this content type"),
		})
		return
	} else if err != nil {
		c.LogFailure(err, "error setting node picture field", "nodeID", gallery.NodeID, "fileID", item.FileID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to set main image",
		})
		return
	}

	c.Logger().Info("node main image set successfully",
		"nodeID", gallery.NodeID,
		"nodeType", node.Type,
		"fileID", item.FileID,
	)

	httputil.SendJSON(w, http.StatusOK, map[string]any{
		"message": "Main image set successfully",
	})
}

// galleryUploadErrorResponse - the status and message of a failed
// upload: a check it failed (a message for the user) is 400, any other
// failure 500 with a message of its own.
func galleryUploadErrorResponse(ctx context.Context, err error) (int, string) {
	if i18n.IsUserError(err) {
		return http.StatusBadRequest, i18n.ErrorText(ctx, err)
	}
	return http.StatusInternalServerError, i18n.T(ctx, "Could not upload the image")
}
