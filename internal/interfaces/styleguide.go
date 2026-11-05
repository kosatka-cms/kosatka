package interfaces

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// StyleguideController - the active theme's style guide: the engine's
// markup in one page of the site's own
// layout - headings and text, buttons, forms, alerts, tables, pagination
// and a View list of cards - to see the theme style all of it alike.
type StyleguideController struct {
	*BaseController
	// textFormats - the text formats whose editor the page shows.
	textFormats service.TextFormatService
}

// NewStyleguideController creates a StyleguideController.
func NewStyleguideController(base *BaseController, textFormats service.TextFormatService) *StyleguideController {
	base.logger = base.logger.With("component", "styleguide.controller")
	return &StyleguideController{BaseController: base, textFormats: textFormats}
}

// styleguideImage - the sample cards' picture (the engine's, so the page
// needs no uploaded files).
const styleguideImage = "/static/images/styleguide.svg"

// Page handles GET /admin/config/appearance/styleguide: the page in the
// site's layout (styleguide.html, a theme may replace it), not the admin
// one. Never indexed.
func (c *StyleguideController) Page(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := c.GetCurrentUser(r)
	data := c.GetCommonDataWithUser(r, user)

	list, err := c.sampleList(r, user)
	if err != nil {
		c.LogFailure(err, "error rendering the style guide's list")
		httputil.TemplateExecutionError(w, err)
		return
	}

	title := i18n.T(ctx, "Style guide")
	seo := PageSEOForList(title, r.URL.Path)
	seo.NoIndex = true
	data["title"] = title
	data["pageSEO"] = seo
	data["styleguideList"] = list
	data["styleguideFormats"] = c.editorFormats(r)
	data["pagination"] = pagination.New(2, 50, 10, r.URL.Path, nil)
	c.setChildTrail(data, []entity.BreadcrumbItem{{URL: "/admin/config/appearance", Label: i18n.T(ctx, "Appearance")}}, title)

	c.RenderOrFail(w, r, "styleguide.html", data)
}

// editorFormats - the enabled text formats with the visual editor (the
// editor section: as on a content field using them).
func (c *StyleguideController) editorFormats(r *http.Request) entity.TextFormatDefs {
	if c.textFormats == nil {
		return nil
	}
	all, err := c.textFormats.TextFormats(r.Context())
	if err != nil {
		c.LogFailure(err, "error loading text formats for the style guide")
		return nil
	}
	var out entity.TextFormatDefs
	for _, f := range all {
		if f.HasRichEditor && f.Status == 1 {
			out = append(out, f)
		}
	}
	return out
}

// sampleList - a View list of three cards (dynamic/list.html), with the
// fields a front page shows: a picture, the title, the author (user), the
// date, the start of a text, a link to the content and the counters. The
// cards link back to this page.
func (c *StyleguideController) sampleList(r *http.Request, user *entity.User) (template.HTML, error) {
	ctx := r.Context()
	builtins := views.BuildContentFieldInstances(ctx, entity.ViewFieldDefs{
		{BuiltinField: entity.ViewFieldBuiltinTitle, LinkTo: entity.ViewFieldLinkContent},
		{BuiltinField: entity.ViewFieldBuiltinAuthor, ShowLabel: true},
		{BuiltinField: entity.ViewFieldBuiltinCreatedAt, DateFormat: "medium"},
		{BuiltinField: entity.ViewFieldBuiltinLink},
		{BuiltinField: entity.ViewFieldBuiltinViewCount, ShowLabel: true},
		{BuiltinField: entity.ViewFieldBuiltinCommentCount, ShowLabel: true},
	}, nil, nil, nil)
	var author entity.User
	if user != nil {
		author = *user
	}
	slug := strings.TrimPrefix(r.URL.Path, "/")
	now := time.Now()
	nodes := entity.Nodes{
		{ID: 1, Slug: slug, Title: "Lorem ipsum dolor sit amet", CreatedBy: author, CreatedAt: now, Counter: entity.Counter{TotalCount: 1024}},
		{ID: 2, Slug: slug, Title: "Consectetur adipiscing elit", CreatedBy: author, CreatedAt: now.AddDate(0, 0, -1), Counter: entity.Counter{TotalCount: 256}},
		{ID: 3, Slug: slug, Title: "Sed do eiusmod tempor", CreatedBy: author, CreatedAt: now.AddDate(0, 0, -2), Counter: entity.Counter{TotalCount: 64}},
	}
	items, err := views.BuildContentListItems(ctx, nil, nodes, builtins, nil, nil, map[int]int{1: 12, 2: 3}, nil, nil)
	if err != nil {
		return "", err
	}
	// The picture and the text: fields of a content type on a real site.
	image := entity.FieldInstanceDef{ID: 1, FieldStorageID: 1, MachineName: "image", FieldType: entity.FieldTypeImage, Config: map[string]any{"link_to": string(entity.ViewFieldLinkContent)}}
	text := entity.FieldInstanceDef{ID: 2, FieldStorageID: 2, MachineName: "body", FieldType: entity.FieldTypeText, Config: map[string]any{}}
	fields := append(entity.FieldInstanceDefs{image, builtins[0], builtins[1], builtins[2], text}, builtins[3:]...)
	for i := range items {
		items[i].Fields = fields
		items[i].Values[text.ID] = "Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur."
		items[i].ImageFieldURLs[image.ID] = styleguideImage
		items[i].ImageFieldPictures[image.ID] = imageurl.Picture{Src: styleguideImage}
		items[i].ImageFieldAlts[image.ID] = ""
	}
	var buf bytes.Buffer
	data := map[string]any{"items": items, "listFieldsHead": fields, "displayStyle": "list", "isViewList": true, service.RequestThemeKey: service.RequestTheme(ctx)}
	if err := c.templateService.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
