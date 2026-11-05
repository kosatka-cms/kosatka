package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// mediaPath - the media library's page.
const mediaPath = "/admin/content/media"

// mediaPerPage - the media library's tiles a page.
const mediaPerPage = 48

// mediaTile - a file in the media library: its name, thumbnail (none -
// not an image), extension, in how many places it's used.
type mediaTile struct {
	File    entity.File
	Name    string
	Picture imageurl.Picture
	Ext     string
	Uses    int
}

// mediaTileOf - f's tile: an image in the library's style
// (entity.MediaThumbnailStyle; gone - the original).
func mediaTileOf(f entity.File, uses int) mediaTile {
	t := mediaTile{File: f, Uses: uses, Name: f.OriginalFilename, Ext: strings.ToUpper(strings.TrimPrefix(path.Ext(f.URI), "."))}
	if t.Name == "" {
		t.Name = path.Base(f.URI)
	}
	if strings.HasPrefix(f.MimeType, "image/") {
		if imageurl.HasStyle(entity.MediaThumbnailStyle) {
			t.Picture = imageurl.StyledPicture(f, entity.MediaThumbnailStyle)
		} else {
			t.Picture = imageurl.OriginalPicture(f)
		}
	}
	return t
}

// mediaFilter - the list's search from the query: q, kind, by, unused,
// page.
func mediaFilter(q url.Values, page int) entity.MediaFilter {
	by, _ := strconv.Atoi(q.Get("by"))
	kind := entity.MediaKind(q.Get("kind"))
	if kind != entity.MediaImages && kind != entity.MediaDocuments {
		kind = entity.MediaAll
	}
	return entity.MediaFilter{Search: strings.TrimSpace(q.Get("q")), Kind: kind, CreatedBy: by,
		Unused: q.Get("unused") == "1", Offset: (page - 1) * mediaPerPage, Limit: mediaPerPage}
}

// MediaList handles GET /admin/content/media: the files uploaded to the
// site, the newest first, found by name or alt text, kind, uploader or
// as used nowhere; ?file=<id> - one file's card beside them: its alt
// text, where it's used, deleting it when nothing uses it.
func (c *Controller) MediaList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	page := c.GetPageNumber(r)
	filter := mediaFilter(q, page)
	list, total, err := c.media.Repo.MediaFiles(ctx, filter)
	if err != nil {
		c.LogFailure(err, "listing the media library")
		httputil.InternalServerError(w)
		return
	}
	ids := make([]int, len(list))
	for i, f := range list {
		ids[i] = f.ID
	}
	counts, err := c.media.Repo.MediaUseCounts(ctx, ids)
	if err != nil {
		c.LogFailure(err, "counting the media library's uses")
		counts = map[int]int{}
	}
	tiles := make([]mediaTile, len(list))
	for i, f := range list {
		tiles[i] = mediaTileOf(f, counts[f.ID])
	}
	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Media library")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["tiles"] = tiles
	data["filter"] = filter
	data["filtered"] = filter.Search != "" || filter.Kind != entity.MediaAll || filter.CreatedBy > 0 || filter.Unused
	data["pagination"] = pagination.New(page, total, mediaPerPage, mediaPath, q)
	if uploaders, err := c.media.Repo.MediaUploaders(ctx); err == nil {
		data["uploaders"] = uploaders
	} else {
		c.LogFailure(err, "listing the media library's uploaders")
	}
	if totals, err := c.media.Repo.MediaTotals(ctx); err == nil {
		data["totals"] = totals
	} else {
		c.LogFailure(err, "counting the media library")
	}
	// The card's links keep the list as it is.
	listQuery := url.Values{}
	for k, v := range q {
		if k != "file" && k != "success" && k != "error" && k != "count" {
			listQuery[k] = v
		}
	}
	data["listQuery"] = listQuery.Encode()
	if id, err := strconv.Atoi(q.Get("file")); err == nil && id > 0 {
		c.mediaCard(r, data, id)
	}
	for _, flag := range []string{"saved", "deleted", "unused_deleted"} {
		data[flag] = q.Get("success") == flag
	}
	data["deletedCount"], _ = strconv.Atoi(q.Get("count"))
	data["error"] = q.Get("error")
	c.RenderOrFail(w, r, "admin/content/media.html", data)
}

// mediaCard adds file id's card to data: the file, its picture, where
// it's used (none found - no card).
func (c *Controller) mediaCard(r *http.Request, data map[string]any, id int) {
	ctx := r.Context()
	f, err := c.media.Files.ByID(ctx, id)
	if err != nil {
		if !errors.Is(err, entity.ErrEntityNotFound) {
			c.LogFailure(err, "loading a media file", "fileID", id)
		}
		return
	}
	uses, err := c.media.Repo.MediaUses(ctx, id)
	if err != nil {
		c.LogFailure(err, "finding a media file's uses", "fileID", id)
		return
	}
	data["selected"] = mediaTileOf(*f, len(uses))
	data["selectedURL"] = imageurl.Original(*f).URI
	data["selectedOriginal"] = imageurl.OriginalPicture(*f)
	data["focalStyles"] = c.focalStyles(ctx)
	data["uses"] = uses
	data["moduleLabels"] = moduleLabelsByName(ctx)
}

// focalStyles - the image styles that crop at the focal point (fill to
// both a width and a height), for the card's previews of their crops.
func (c *Controller) focalStyles(ctx context.Context) entity.ImageStyleDefs {
	styles, err := c.imageStyleService.ImageStyles(ctx)
	if err != nil {
		c.LogFailure(err, "listing the image styles for the focal point")
		return nil
	}
	var out entity.ImageStyleDefs
	for _, s := range styles {
		if s.Anchor == entity.ImageStyleAnchorFocal && s.Mode == string(imagestyle.ResizeModeFill) && s.Height > 0 {
			out = append(out, s)
		}
	}
	return out
}

// moduleLabelsByName - the modules' names by machine name, translated.
func moduleLabelsByName(ctx context.Context) map[string]string {
	out := map[string]string{}
	for _, m := range entity.Modules {
		out[m.Name] = i18n.T(ctx, m.Label)
	}
	return out
}

// mediaBack - the media library with the list's query the form sent
// (back: a query string, nothing else), file's card open and the
// notice's parameters.
func mediaBack(r *http.Request, file int, notice url.Values) string {
	q, err := url.ParseQuery(r.FormValue("back"))
	if err != nil {
		q = url.Values{}
	}
	if file > 0 {
		q.Set("file", strconv.Itoa(file))
	}
	for k, v := range notice {
		q[k] = v
	}
	return mediaPath + "?" + q.Encode()
}

// MediaUpdate handles POST /admin/content/media/{id}: the file's alt
// text, the same wherever the file is used, and its focal point
// (focal_x, focal_y - percent; both empty - none).
func (c *Controller) MediaUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	ctx := r.Context()
	alt := strings.TrimSpace(r.FormValue("alt"))
	if len([]rune(alt)) > 255 {
		alt = string([]rune(alt)[:255])
	}
	focal, focalOK := focalFromForm(r)
	if !focalOK {
		http.Redirect(w, r, mediaBack(r, id, url.Values{"error": {"save"}}), http.StatusSeeOther)
		return
	}
	if err := c.media.Files.UpdateAlt(ctx, id, alt); err != nil {
		c.LogFailure(err, "saving a media file's alt text", "fileID", id)
		http.Redirect(w, r, mediaBack(r, id, url.Values{"error": {"save"}}), http.StatusSeeOther)
		return
	}
	if r.Form.Has("focal_x") {
		if err := c.media.SetFocal(ctx, id, focal); err != nil {
			c.LogFailure(err, "saving a media file's focal point", "fileID", id)
			http.Redirect(w, r, mediaBack(r, id, url.Values{"error": {"save"}}), http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, mediaBack(r, id, url.Values{"success": {"saved"}}), http.StatusSeeOther)
}

// focalFromForm - the focal point the form sent (focal_x, focal_y,
// percent): both empty - none; false for anything else but two numbers
// 0-100.
func focalFromForm(r *http.Request) (*entity.FocalPoint, bool) {
	fx, fy := strings.TrimSpace(r.FormValue("focal_x")), strings.TrimSpace(r.FormValue("focal_y"))
	if fx == "" && fy == "" {
		return nil, true
	}
	x, errX := strconv.Atoi(fx)
	y, errY := strconv.Atoi(fy)
	p := entity.FocalPoint{X: x, Y: y}
	if errX != nil || errY != nil || !p.Valid() {
		return nil, false
	}
	return &p, true
}

// MediaDelete handles POST /admin/content/media/{id}/delete: the file
// and its stored copies, when nothing uses it.
func (c *Controller) MediaDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	err := c.media.Delete(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrMediaInUse):
		http.Redirect(w, r, mediaBack(r, id, url.Values{"error": {"in_use"}}), http.StatusSeeOther)
	case err != nil:
		c.LogFailure(err, "deleting a media file", "fileID", id)
		http.Redirect(w, r, mediaBack(r, id, url.Values{"error": {"delete"}}), http.StatusSeeOther)
	default:
		http.Redirect(w, r, mediaBack(r, 0, url.Values{"success": {"deleted"}}), http.StatusSeeOther)
	}
}

// MediaDeleteUnused handles POST /admin/content/media/delete-unused:
// every file nothing uses.
func (c *Controller) MediaDeleteUnused(w http.ResponseWriter, r *http.Request) {
	n, err := c.media.DeleteUnused(r.Context())
	if err != nil {
		c.LogFailure(err, "deleting the unused media files")
		http.Redirect(w, r, mediaBack(r, 0, url.Values{"error": {"delete_unused"}}), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, mediaBack(r, 0, url.Values{"success": {"unused_deleted"}, "count": {fmt.Sprint(n)}}), http.StatusSeeOther)
}
