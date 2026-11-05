package interfaces

import (
	"context"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// mediaPickerPerPage - the picker's files a request.
const mediaPickerPerPage = 24

// mediaPickerItem - a file in a field's media library picker.
type mediaPickerItem struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Thumb  string `json:"thumb,omitempty"`
	Ext    string `json:"ext"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Size   int    `json:"size"`
	Alt    string `json:"alt"`
	Uses   int    `json:"uses"`
}

// pickerFilter - the media library's files field may take for the
// request's user (service.MediaLibrary.PickerFilter); false when the
// field isn't an image or file field picking from the library, or the
// user may neither create nor edit content of its type.
func (c *DynamicContentController) pickerFilter(ctx context.Context, r *http.Request, field entity.FieldInstanceDef) (entity.MediaFilter, bool) {
	if c.media == nil || (field.FieldType != entity.FieldTypeImage && field.FieldType != entity.FieldTypeFile) || !field.AllowsLibrary() {
		return entity.MediaFilter{}, false
	}
	user, _ := c.GetCurrentUser(r)
	if user == nil {
		return entity.MediaFilter{}, false
	}
	ct, err := c.contentTypeService.ContentTypeByID(ctx, field.ContentTypeID)
	if err != nil {
		c.LogFailure(err, "loading the picker field's content type", "fieldID", field.ID)
		return entity.MediaFilter{}, false
	}
	administers := c.authService.HasPermission(ctx, user, service.PermAdministerContent)
	may := administers
	for _, perm := range []string{service.PermCreate(ct.MachineName), service.PermEditOwn(ct.MachineName), service.PermEditAny(ct.MachineName)} {
		may = may || c.authService.HasPermission(ctx, user, perm)
	}
	if !may {
		return entity.MediaFilter{}, false
	}
	filter, err := c.media.PickerFilter(ctx, field, user.ID, administers)
	if err != nil {
		c.LogFailure(err, "building the media picker's filter", "fieldID", field.ID)
		return entity.MediaFilter{}, false
	}
	return filter, true
}

// MediaPick handles GET /api/media/pick?field=<field id>&q=&page=: the
// media library's files the field may take, the newest first, found by
// name or alt text - JSON {"items": [mediaPickerItem…], "more": bool}.
func (c *DynamicContentController) MediaPick(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.Atoi(r.URL.Query().Get("field"))
	field, err := c.contentTypeService.FieldByID(ctx, id)
	if err != nil {
		httputil.SendJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	filter, ok := c.pickerFilter(ctx, r, *field)
	if !ok {
		httputil.SendJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	page = max(page, 1)
	filter.Search = strings.TrimSpace(r.URL.Query().Get("q"))
	filter.Offset, filter.Limit = (page-1)*mediaPickerPerPage, mediaPickerPerPage
	files, total, err := c.media.Repo.MediaFiles(ctx, filter)
	if err != nil {
		c.LogFailure(err, "listing the media picker's files", "fieldID", field.ID)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed"})
		return
	}
	ids := make([]int, len(files))
	for i, f := range files {
		ids[i] = f.ID
	}
	uses, err := c.media.Repo.MediaUseCounts(ctx, ids)
	if err != nil {
		c.LogFailure(err, "counting the media picker's uses", "fieldID", field.ID)
		uses = map[int]int{}
	}
	items := make([]mediaPickerItem, len(files))
	for i, f := range files {
		items[i] = mediaPickerItemOf(f, uses[f.ID])
	}
	httputil.SendJSON(w, http.StatusOK, map[string]any{"items": items, "more": filter.Offset+len(files) < total})
}

// mediaPickerItemOf - f as the picker shows it: an image in the media
// library's style (gone - the original).
func mediaPickerItemOf(f entity.File, uses int) mediaPickerItem {
	it := mediaPickerItem{ID: f.ID, Name: f.OriginalFilename, Ext: strings.ToUpper(strings.TrimPrefix(path.Ext(f.URI), ".")),
		Width: f.Width, Height: f.Height, Size: f.SizeBytes, Alt: f.Alt, Uses: uses}
	if it.Name == "" {
		it.Name = path.Base(f.URI)
	}
	if strings.HasPrefix(f.MimeType, "image/") {
		if imageurl.HasStyle(entity.MediaThumbnailStyle) {
			it.Thumb = imageurl.StyledPicture(f, entity.MediaThumbnailStyle).Src
		} else {
			it.Thumb = imageurl.Original(f).URI
		}
	}
	return it
}

// pickedFiles - those of the files the form picked from the media
// library (ids) the field may take for the request's user.
func (c *DynamicContentController) pickedFiles(r *http.Request, field entity.FieldInstanceDef, ids []int) map[int]bool {
	if len(ids) == 0 {
		return nil
	}
	ctx := r.Context()
	filter, ok := c.pickerFilter(ctx, r, field)
	if !ok {
		return nil
	}
	out, err := c.media.Pickable(ctx, filter, ids)
	if err != nil {
		c.LogFailure(err, "checking the files picked from the media library", "fieldID", field.ID)
		return nil
	}
	return out
}
