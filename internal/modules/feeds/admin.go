package feeds

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Feeds (/admin/config/feeds): XML feeds of the site's content for
// aggregators and RSS readers, each with its own source and an explicit
// per-content-type field map.

const feedsAdminURL = "/admin/config/feeds"

// feedsErrorCodes - the ?error= code per service error.
var feedsErrorCodes = map[error]string{
	ErrFeedLabel:        "label",
	ErrFeedPath:         "path",
	ErrFeedPathTaken:    "path-taken",
	ErrFeedContentTypes: "types",
	ErrFeedFieldMap:     "fields",
}

func feedsErrorCode(err error) string {
	for e, code := range feedsErrorCodes {
		if errors.Is(err, e) {
			return code
		}
	}
	return "save"
}

// feedTypeFields - a content type with the fields a feed can map: text
// (body), image (cover) and taxonomy (category).
type feedTypeFields struct {
	Type       modulekit.ContentTypeDef
	TextFields modulekit.FieldInstanceDefs
	Images     modulekit.FieldInstanceDefs
	Taxonomies modulekit.FieldInstanceDefs
}

func (c *FeedController) feedTypeFields(ctx context.Context) ([]feedTypeFields, error) {
	defs, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]feedTypeFields, 0, len(defs))
	for _, d := range defs {
		fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, d.ID)
		if err != nil {
			return nil, err
		}
		t := feedTypeFields{Type: d}
		for _, f := range fields.SortBySortOrder() {
			switch f.FieldType {
			case modulekit.FieldTypeText:
				t.TextFields = append(t.TextFields, f)
			case modulekit.FieldTypeImage:
				t.Images = append(t.Images, f)
			case modulekit.FieldTypeTaxonomy:
				t.Taxonomies = append(t.Taxonomies, f)
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// FeedList renders GET /admin/config/feeds.
func (c *FeedController) FeedList(w http.ResponseWriter, r *http.Request) {
	feeds, err := c.feeds.Feeds(r.Context())
	if err != nil {
		c.LogFailure(err, "error loading feeds")
		httputil.InternalServerError(w)
		return
	}
	data := c.AdminPageData(r)
	data["title"] = i18n.T(r.Context(), "Feeds")
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Feeds"))}
	data["feeds"] = feeds
	data["success"] = r.URL.Query().Get("success")
	data["error"] = r.URL.Query().Get("error")
	c.RenderOrFail(w, r, "admin/feeds.html", data)
}

// FeedNew renders GET /admin/config/feeds/new.
func (c *FeedController) FeedNew(w http.ResponseWriter, r *http.Request) {
	c.renderFeedForm(w, r, Feed{Format: FeedFormatDzen, Status: 1, ItemLimit: 50}, "")
}

// FeedEdit renders GET /admin/config/feeds/{id}.
func (c *FeedController) FeedEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, feedsAdminURL, http.StatusSeeOther)
		return
	}
	feed, err := c.feeds.FeedByID(r.Context(), id)
	if err != nil || feed == nil {
		http.Redirect(w, r, feedsAdminURL, http.StatusSeeOther)
		return
	}
	c.renderFeedForm(w, r, *feed, "")
}

// renderFeedForm renders feed's form; errCode - why saving it failed
// ("" - the page's ?error=), the form keeping what was typed.
func (c *FeedController) renderFeedForm(w http.ResponseWriter, r *http.Request, feed Feed, errCode string) {
	ctx := r.Context()
	types, err := c.feedTypeFields(ctx)
	if err != nil {
		c.LogFailure(err, "error loading content types for feed form")
		httputil.InternalServerError(w)
		return
	}
	styles, err := c.imageStyleService.ImageStyles(ctx)
	if err != nil {
		c.LogFailure(err, "error loading image styles for feed form")
		httputil.InternalServerError(w)
		return
	}
	data := c.AdminPageData(r)
	if feed.ID == 0 {
		data["title"] = i18n.T(r.Context(), "New feed")
	} else {
		data["title"] = i18n.T(r.Context(), "Feed: @label", "@label", feed.Label)
	}
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Feeds"))}
	data["feed"] = feed
	data["types"] = types
	data["imageStyles"] = styles
	data["formats"] = FeedFormats
	data["error"] = errCode
	if errCode == "" {
		data["error"] = r.URL.Query().Get("error")
	}
	data["saved"] = r.URL.Query().Get("success") != ""
	// An empty language is the site's (render.go).
	data["siteLanguage"] = i18n.Default()
	c.RenderOrFail(w, r, "admin/feed_edit.html", data)
}

// feedFromForm - the feed the form submits (ID from the path, 0 - new).
func feedFromForm(r *http.Request, id int) Feed {
	itemLimit, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("item_limit")))
	days, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("days")))
	f := Feed{
		ID:           id,
		Label:        r.FormValue("label"),
		Path:         r.FormValue("path"),
		Format:       ParseFeedFormat(r.FormValue("format")),
		ContentTypes: r.Form["content_types"],
		TermIDs:      ParseFeedTermIDs(r.FormValue("term_ids")),
		ItemLimit:    itemLimit,
		Days:         days,
		Title:        r.FormValue("title"),
		Description:  r.FormValue("description"),
		Language:     r.FormValue("language"),
		ImageStyle:   r.FormValue("image_style"),
	}
	if r.FormValue("status") == "1" {
		f.Status = 1
	}
	for _, t := range f.ContentTypes {
		f.FieldMaps = append(f.FieldMaps, FeedFieldMap{
			ContentType:   t,
			BodyField:     strings.TrimSpace(r.FormValue("body_" + t)),
			ImageField:    strings.TrimSpace(r.FormValue("image_" + t)),
			CategoryField: strings.TrimSpace(r.FormValue("category_" + t)),
		})
	}
	return f
}

// FeedSave handles POST /admin/config/feeds (new) and
// POST /admin/config/feeds/{id}.
func (c *FeedController) FeedSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, feedsAdminURL, http.StatusSeeOther)
		return
	}
	id := 0
	if r.PathValue("id") != "" {
		var ok bool
		if id, ok = c.PathID(r); !ok {
			http.Redirect(w, r, feedsAdminURL, http.StatusSeeOther)
			return
		}
	}
	feed := feedFromForm(r, id)
	saved, err := c.feeds.Save(r.Context(), feed)
	if err != nil {
		// The form again, with what was typed and why it wasn't saved.
		c.Logger().Warn("error saving feed", "error", err, "feedID", id)
		c.renderFeedForm(w, r, feed, feedsErrorCode(err))
		return
	}
	http.Redirect(w, r, feedsAdminURL+"/"+strconv.Itoa(saved)+"?success=1", http.StatusSeeOther)
}

// FeedDelete handles POST /admin/config/feeds/{id}/delete.
func (c *FeedController) FeedDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, feedsAdminURL, http.StatusSeeOther)
		return
	}
	if err := c.feeds.Delete(r.Context(), id); err != nil {
		c.LogFailure(err, "deleting the feed", "feedID", id)
		http.Redirect(w, r, feedsAdminURL+"?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, feedsAdminURL+"?success=deleted", http.StatusSeeOther)
}
