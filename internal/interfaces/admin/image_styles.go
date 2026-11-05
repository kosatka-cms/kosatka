package admin

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

func isValidImageStyleMode(mode string) bool {
	return slices.Contains(entity.ImageStyleResizeModes, mode)
}

func isValidImageStyleAnchor(anchor string) bool {
	return slices.Contains(entity.ImageStyleAnchors, anchor)
}

// ImageStyleList renders GET /admin/config/image-styles - the editable
// resize presets used across the site via imageurl.Styled(image, styleName).
func (c *Controller) ImageStyleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	styles, err := c.imageStyleService.ImageStyles(ctx)
	if err != nil {
		c.LogFailure(err, "error listing image styles")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Image styles")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Image styles"))}
	data["imageStyles"] = styles
	data["modes"] = entity.ImageStyleResizeModes
	data["anchors"] = entity.ImageStyleAnchors
	data["success"] = r.URL.Query().Get("success") == "1"
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/image-styles/index.html", data)
}

// imageStyleFromForm parses the width/height/mode/anchor/quality fields
// shared by the create and edit forms.
func imageStyleFromForm(r *http.Request) (width, height int, mode, anchor string, quality int, format string, ok bool) {
	width, err := strconv.Atoi(r.FormValue("width"))
	if err != nil || width <= 0 {
		return 0, 0, "", "", 0, "", false
	}
	height, _ = strconv.Atoi(r.FormValue("height"))
	if height < 0 {
		height = 0
	}
	mode = r.FormValue("mode")
	if !isValidImageStyleMode(mode) {
		return 0, 0, "", "", 0, "", false
	}
	anchor = r.FormValue("anchor")
	if !isValidImageStyleAnchor(anchor) {
		return 0, 0, "", "", 0, "", false
	}
	quality, _ = strconv.Atoi(r.FormValue("quality"))
	if quality <= 0 || quality > 100 {
		quality = 85
	}
	format = entity.NormalizeImageStyleFormat(r.FormValue("format"))
	return width, height, mode, anchor, quality, format, true
}

// ImageStyleCreate handles POST /admin/config/image-styles.
func (c *Controller) ImageStyleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/image-styles", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	name := strings.ToLower(strings.TrimSpace(r.FormValue("name")))
	label := strings.TrimSpace(r.FormValue("label"))
	if name == "" || label == "" {
		http.Redirect(w, r, "/admin/config/image-styles?error=empty", http.StatusSeeOther)
		return
	}
	width, height, mode, anchor, quality, format, ok := imageStyleFromForm(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/image-styles?error=invalid", http.StatusSeeOther)
		return
	}
	desc := strings.TrimSpace(r.FormValue("description"))

	_, err := c.imageStyleService.CreateImageStyle(ctx, entity.ImageStyleDef{
		Name:        name,
		Label:       label,
		Description: desc,
		Width:       width,
		Height:      height,
		Mode:        mode,
		Anchor:      anchor,
		Quality:     quality,
		Format:      format,
	})
	if err != nil {
		if errors.Is(err, service.ErrImageStyleDuplicate) {
			http.Redirect(w, r, "/admin/config/image-styles?error=duplicate", http.StatusSeeOther)
			return
		}
		c.LogFailure(err, "error creating image style", "name", name)
		http.Redirect(w, r, "/admin/config/image-styles?error=create", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/image-styles?success=1", http.StatusSeeOther)
}

// ImageStyleEdit renders GET /admin/config/image-styles/{id}/edit - one
// style's settings; submits to ImageStyleUpdate.
func (c *Controller) ImageStyleEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	style, err := c.imageStyleService.ImageStyleByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the image style", "styleID", id)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Style \"@name\"", "@name", style.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Style @name - Admin panel", "@name", style.Label)}
	data["style"] = style
	data["modes"] = entity.ImageStyleResizeModes
	data["anchors"] = entity.ImageStyleAnchors
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"

	c.RenderOrFail(w, r, "admin/image-styles/edit.html", data)
}

// ImageStyleUpdate handles POST /admin/config/image-styles/{id}. Name is
// immutable: derivative URLs and Go call sites reference a style by name.
// Redirects back to ImageStyleEdit.
func (c *Controller) ImageStyleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/image-styles", http.StatusSeeOther)
		return
	}
	editRedirect := fmt.Sprintf("/admin/config/image-styles/%d/edit", id)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, editRedirect, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	existing, err := c.imageStyleService.ImageStyleByID(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading the image style", "styleID", id)
		http.Redirect(w, r, "/admin/config/image-styles?error=update", http.StatusSeeOther)
		return
	}

	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		http.Redirect(w, r, editRedirect+"?error=empty", http.StatusSeeOther)
		return
	}
	width, height, mode, anchor, quality, format, formOK := imageStyleFromForm(r)
	if !formOK {
		http.Redirect(w, r, editRedirect+"?error=invalid", http.StatusSeeOther)
		return
	}
	desc := strings.TrimSpace(r.FormValue("description"))

	_, err = c.imageStyleService.UpdateImageStyle(ctx, entity.ImageStyleDef{
		ID:          id,
		Name:        existing.Name,
		Label:       label,
		Description: desc,
		Width:       width,
		Height:      height,
		Mode:        mode,
		Anchor:      anchor,
		Quality:     quality,
		Format:      format,
		HiDPI:       r.FormValue("hidpi") == "1",
	})
	if err != nil {
		c.LogFailure(err, "error updating image style", "id", id)
		http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		return
	}

	// Cached derivatives were generated with the old settings and are
	// stale until flushed. Best-effort: a flush failure must not fail the save.
	if c.imageStorage != nil {
		if err := c.imageStorage.ClearStyleCache(r.Context(), existing.Name); err != nil {
			c.logger.Warn("error flushing image style cache after update", "error", err, "name", existing.Name)
		}
	}

	http.Redirect(w, r, editRedirect+"?success=1", http.StatusSeeOther)
}

// ImageStyleDeleteConfirm renders GET /admin/config/image-styles/{id}/delete -
// a confirmation page that posts to ImageStyleDelete.
func (c *Controller) ImageStyleDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	style, err := c.imageStyleService.ImageStyleByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the image style", "styleID", id)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Delete style \"@name\"", "@name", style.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Delete style @name - Admin panel", "@name", style.Label)}
	data["style"] = style

	c.RenderOrFail(w, r, "admin/image-styles/delete.html", data)
}

// ImageStyleDelete handles POST /admin/config/image-styles/{id}/delete.
// Styles still referenced (display settings, Go call sites) may be deleted:
// imageurl.Styled handles an unknown style name gracefully (404 + logged
// error).
func (c *Controller) ImageStyleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/image-styles", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/image-styles?error=invalid", http.StatusSeeOther)
		return
	}

	if err := c.imageStyleService.DeleteImageStyle(ctx, id); err != nil {
		c.LogFailure(err, "error deleting image style", "id", id)
		http.Redirect(w, r, "/admin/config/image-styles?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/image-styles?deleted=1", http.StatusSeeOther)
}
