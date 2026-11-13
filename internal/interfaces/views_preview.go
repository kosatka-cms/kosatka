package interfaces

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// viewPreviewRequest - JSON body for ViewPreview: page settings from the admin
// settings modal, possibly unsaved.
type viewPreviewRequest struct {
	DisplayStyle string `json:"display_style"`
	GridColumns  int    `json:"grid_columns"`
	// OverflowAfter - draft value, like GridColumns.
	OverflowAfter int `json:"overflow_after"`
	// OverflowHeaderText - draft value, like GridColumns.
	OverflowHeaderText string `json:"overflow_header_text"`
	// Carousel* - draft values, like GridColumns.
	CarouselSlidesPerView  int  `json:"carousel_slides_per_view"`
	CarouselAutoplay       bool `json:"carousel_autoplay"`
	CarouselLoop           bool `json:"carousel_loop"`
	CarouselShowNavigation bool `json:"carousel_show_navigation"`
	CarouselShowPagination bool `json:"carousel_show_pagination"`
	// CarouselLinkFieldStorageID - 0 means not set.
	CarouselLinkFieldStorageID int    `json:"carousel_link_field_storage_id"`
	CarouselEffect             string `json:"carousel_effect"`
	ShowPagination             bool   `json:"show_pagination"`
	PageSize                   int    `json:"page_size"`
	// SkipItems - the draft value; nil - the saved one (0 is a value).
	SkipItems *int `json:"skip_items"`
	// HeaderText/FooterText - draft values, rendered as-is in the admin-only
	// preview; ViewUpdate sanitizes them on save.
	HeaderText string `json:"header_text"`
	FooterText string `json:"footer_text"`
	// CustomCSS - draft value, so the preview matches the page.
	CustomCSS string `json:"custom_css"`
	// RowTemplate - the draft row template, sanitized as on save
	// (htmlsanitize.SanitizeAdminRowTemplate) so the preview matches the page.
	RowTemplate string `json:"row_template"`
	// GroupByField/GroupDateFormat - the draft grouping (an unknown field
	// groups nothing).
	GroupByField    string `json:"group_by_field"`
	GroupDateFormat string `json:"group_date_format"`
	// Page - the preview's AJAX pager page (the preview has no real URL).
	Page int `json:"page"`
	// Letter - the preview's glossary letter.
	Letter string `json:"letter"`
	// ExposedFilters - the preview's exposed filter values, posted instead of
	// a query string.
	ExposedFilters map[string]string `json:"exposed_filters"`
}

// ViewPreview handles POST /admin/config/views/{id}/preview, the admin live
// preview. Filters, fields and sorts are read from the DB (the admin UI saves
// them immediately); only the page settings in the request body may be
// unsaved.
func (c *DynamicContentController) ViewPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.JSONError(w, i18n.T(r.Context(), "The preview takes a POST request."), http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		httputil.JSONError(w, i18n.T(ctx, "The View was not found."), http.StatusBadRequest)
		return
	}
	view, err := c.viewService.ViewByID(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading the View display", "viewID", id)
		httputil.JSONError(w, i18n.T(ctx, "The View was not found."), http.StatusNotFound)
		return
	}
	effectiveView, err := c.viewService.EffectiveSettings(ctx, *view)
	if err != nil {
		c.LogFailure(err, "error resolving effective view settings for preview", "viewID", id)
		httputil.InternalServerError(w)
		return
	}
	view = &effectiveView

	var req viewPreviewRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.JSONError(w, i18n.T(ctx, "The preview request could not be read."), http.StatusBadRequest)
		return
	}

	// Comment views have their own, simpler preview path.
	if view.BaseEntity == entity.ViewBaseEntityComment {
		c.renderCommentViewPreview(w, r, view, req)
		return
	}

	displayStyle := entity.ViewDisplayStyle(req.DisplayStyle)
	if displayStyle != entity.ViewDisplayStyleTable && displayStyle != entity.ViewDisplayStyleList && displayStyle != entity.ViewDisplayStyleCarousel {
		displayStyle = entity.ViewDisplayStyleGrid
	}
	gridColumns := req.GridColumns
	if gridColumns <= 0 {
		gridColumns = view.GridColumns
	}
	overflowAfter := req.OverflowAfter
	if overflowAfter <= 0 {
		overflowAfter = view.OverflowAfter
	}
	overflowHeaderText := req.OverflowHeaderText
	if overflowHeaderText == "" {
		overflowHeaderText = view.OverflowHeaderText
	}
	carouselSlidesPerView := req.CarouselSlidesPerView
	if carouselSlidesPerView <= 0 {
		carouselSlidesPerView = view.CarouselSlidesPerView
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = view.PageSize
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	skipItems := view.SkipItems
	if req.SkipItems != nil {
		skipItems = max(*req.SkipItems, 0)
	}

	nodes, total, err := c.viewService.PreviewView(ctx, *view, views.PreviewSettings{
		DisplayStyle:        displayStyle,
		GridColumns:         gridColumns,
		ShowPagination:      req.ShowPagination,
		PageSize:            pageSize,
		Page:                page,
		SkipItems:           skipItems,
		Letter:              req.Letter,
		ExposedFilterValues: req.ExposedFilters,
	})
	if err != nil {
		c.LogFailure(err, "error previewing view", "viewID", id)
		httputil.InternalServerError(w)
		return
	}

	// Glossary: ShowGlossary is saved immediately, so it is read from the
	// stored view. A preview has no exposed-filter query params, hence the
	// empty map.
	activeLetter := strings.ToUpper(strings.TrimSpace(req.Letter))
	var glossary entity.GlossaryData
	if view.ShowGlossary {
		var live views.Config
		if live, err = c.viewService.LiveConfig(ctx, *view); err == nil {
			glossary, err = c.viewService.GlossaryOf(ctx, live, map[string]string{})
		}
		if err != nil {
			c.LogFailure(err, "error computing view glossary for preview", "viewID", id)
			// Non-fatal: render without the glossary bar.
		}
	}

	viewFields, err := c.viewService.EffectiveFieldsByView(ctx, *view)
	if err != nil {
		c.LogFailure(err, "error loading view fields for preview", "viewID", id)
		httputil.InternalServerError(w)
		return
	}
	listFields, items, err := views.BuildContentList(ctx, c.contentListSources(), id, viewFields, nodes)
	if err != nil {
		c.LogFailure(err, "error building the view's list", "viewID", id)
		httputil.InternalServerError(w)
		return
	}

	// exposedFilters - the same form as the public page, pre-filled from the
	// preview's draft values.
	filters, err := c.viewService.EffectiveFiltersByView(ctx, *view)
	if err != nil {
		c.LogFailure(err, "error loading view filters for preview", "viewID", id)
		httputil.InternalServerError(w)
		return
	}
	exposedFields := c.buildViewExposedFilterFields(ctx, view.ID, filters, req.ExposedFilters)

	// pagination - no base URL: admin_views.js loads a page link's data-page
	// in place (pagination.html).
	pg := pagination.New(page, total, pageSize, "", nil)
	data := map[string]any{
		"items":                      items,
		"listFieldsHead":             listFields,
		"displayStyle":               string(displayStyle),
		"gridColumns":                gridColumns,
		"overflowAfter":              overflowAfter,
		"overflowHeaderText":         overflowHeaderText,
		"carouselSlidesPerView":      carouselSlidesPerView,
		"carouselAutoplay":           req.CarouselAutoplay,
		"carouselLoop":               req.CarouselLoop,
		"carouselShowNavigation":     req.CarouselShowNavigation,
		"carouselShowPagination":     req.CarouselShowPagination,
		"carouselLinkFieldStorageID": req.CarouselLinkFieldStorageID,
		"carouselLinkURLs":           c.resolveCarouselLinkURLs(ctx, req.CarouselLinkFieldStorageID, nodes),
		"carouselEffect":             req.CarouselEffect,
		"total":                      total,
		"showPagination":             req.ShowPagination,
		"pagination":                 pg,
		"showGlossary":               view.ShowGlossary,
		"glossaryItems":              glossary,
		"activeLetter":               activeLetter,
		"headerText":                 req.HeaderText,
		"footerText":                 req.FooterText,
		"customCSS":                  req.CustomCSS,
		"rowTemplate":                htmlsanitize.SanitizeAdminRowTemplate(req.RowTemplate),
		"groupByField":               req.GroupByField,
		"groupDateFormat":            req.GroupDateFormat,
		"exposedFilters":             exposedFields,
		"isViewList":                 true,
	}
	var buf bytes.Buffer
	withRequestContext(r.Context(), data)
	if err := c.templateService.RenderNamed(&buf, "view_preview", data, "admin/views/preview.html", "dynamic/list.html", "pagination.html"); err != nil {
		c.LogFailure(err, "error rendering view preview fragment", "viewID", id)
		httputil.InternalServerError(w)
		return
	}
	httputil.JSON(w, map[string]any{"html": buf.String(), "head": c.viewPreviewHead(r, service.AttachedModules(buf.Bytes()))}, http.StatusOK)
}

// viewPreviewHead - the <head> of the Views preview frame
// (admin/views/preview.html "view_preview_head"): the public pages'
// stylesheets with the active theme, so the preview looks as the site
// does, and the assets of the modules the list attaches (a module's
// field). Empty on a render error (the frame falls back to bare HTML).
func (c *DynamicContentController) viewPreviewHead(r *http.Request, modules []string) string {
	var buf bytes.Buffer
	data := withRequestContext(r.Context(), map[string]any{"attachModules": modules})
	if err := c.templateService.RenderNamed(&buf, "view_preview_head", data, "admin/views/preview.html", "partials/html.html", "partials/theme_head.html"); err != nil {
		c.LogFailure(err, "error rendering the view preview head")
		return ""
	}
	return buf.String()
}

// renderCommentViewPreview is ViewPreview's path for comment-based Views, with
// the same scope as renderCommentViewPage, on the live configuration
// (LiveConfig): page size and other settings from the stored view, since
// the settings modal saves on change.
func (c *DynamicContentController) renderCommentViewPreview(w http.ResponseWriter, r *http.Request, view *entity.ViewDisplayDef, req viewPreviewRequest) {
	ctx := r.Context()
	page := req.Page
	if page < 1 {
		page = 1
	}

	live, err := c.viewService.LiveConfig(ctx, *view)
	if err != nil {
		c.LogFailure(err, "error loading comment view settings for preview", "viewID", view.ID)
		httputil.InternalServerError(w)
		return
	}
	comments, total, err := c.viewService.RunCommentConfig(ctx, live, views.RunContext{QueryParams: map[string]string{}, Page: page})
	if err != nil {
		c.LogFailure(err, "error previewing comment view", "viewID", view.ID)
		httputil.InternalServerError(w)
		return
	}
	viewFields := live.Fields
	// draft - the view with the settings modal's unsaved values.
	draft := *view
	draft.DisplayStyle = entity.ViewDisplayStyle(req.DisplayStyle)
	if draft.DisplayStyle != entity.ViewDisplayStyleTable && draft.DisplayStyle != entity.ViewDisplayStyleList {
		draft.DisplayStyle = entity.ViewDisplayStyleGrid
	}
	draft.HeaderText, draft.FooterText, draft.CustomCSS = req.HeaderText, req.FooterText, req.CustomCSS
	draft.RowTemplate = htmlsanitize.SanitizeAdminRowTemplate(req.RowTemplate)
	draft.GroupByField, draft.GroupDateFormat = req.GroupByField, req.GroupDateFormat
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = view.PageSize
	}
	data := views.CommentListData(ctx, draft, viewFields, comments)
	data["showPagination"] = req.ShowPagination
	data["pagination"] = pagination.New(page, total, pageSize, "", nil)
	var buf bytes.Buffer
	withRequestContext(r.Context(), data)
	if err := c.templateService.RenderNamed(&buf, "content", data, "dynamic/comment_list.html", "pagination.html"); err != nil {
		c.LogFailure(err, "error rendering comment view preview fragment", "viewID", view.ID)
		httputil.InternalServerError(w)
		return
	}
	httputil.JSON(w, map[string]any{"html": buf.String(), "head": c.viewPreviewHead(r, service.AttachedModules(buf.Bytes()))}, http.StatusOK)
}
