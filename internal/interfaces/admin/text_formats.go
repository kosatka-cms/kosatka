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
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// toolbarButtonOption - one checkbox in the format edit page's TinyMCE
// toolbar builder: Button is the toolbar button it adds (also the key
// admin_text_format_edit.js uses), Plugin the TinyMCE plugin it needs
// (empty for core commands). Options are grouped into toolbarGroups;
// checking any option in a group adds that group's "|"-delimited segment.
type toolbarButtonOption struct {
	Label  string
	Button string
	Plugin string
}

type toolbarGroup struct {
	Label   string
	Options []toolbarButtonOption
}

// toolbarGroups - a curated, safe subset of the bundled TinyMCE
// plugins/buttons (web/static/js/tinymce/plugins). undo/redo are always
// prepended to the toolbar string, so they have no checkbox.
var toolbarGroups = []toolbarGroup{
	{Label: i18n.N("Text"), Options: []toolbarButtonOption{
		{Label: i18n.N("Boldface"), Button: "bold"},
		{Label: i18n.N("Italic"), Button: "italic"},
		{Label: i18n.N("Underline"), Button: "underline"},
		{Label: i18n.N("Strikethrough"), Button: "strikethrough"},
		{Label: i18n.N("Headings and blocks (H2-H6, quote)"), Button: "blocks"},
		// blockquote - a core command: a one-click toggle, unlike the "blocks"
		// dropdown's Blockquote entry.
		{Label: i18n.N("Quote (blockquote)"), Button: "blockquote"},
		{Label: i18n.N("Clear formatting"), Button: "removeformat"},
	}},
	{Label: i18n.N("Alignment"), Options: []toolbarButtonOption{
		{Label: i18n.N("Left"), Button: "alignleft"},
		{Label: i18n.N("Center"), Button: "aligncenter"},
		{Label: i18n.N("Right"), Button: "alignright"},
		{Label: i18n.N("Justify"), Button: "alignjustify"},
	}},
	{Label: i18n.N("Lists"), Options: []toolbarButtonOption{
		{Label: i18n.N("Bulleted list"), Button: "bullist", Plugin: "lists"},
		{Label: i18n.N("Numbered list"), Button: "numlist", Plugin: "lists"},
	}},
	{Label: i18n.N("Insert"), Options: []toolbarButtonOption{
		{Label: i18n.N("Link"), Button: "link", Plugin: "link"},
		{Label: i18n.N("Picture"), Button: "image", Plugin: "image"},
		{Label: i18n.N("Table"), Button: "table", Plugin: "table"},
		{Label: i18n.N("Media (video)"), Button: "media", Plugin: "media"},
		{Label: i18n.N("Special characters"), Button: "charmap", Plugin: "charmap"},
		{Label: i18n.N("Anchor"), Button: "anchor", Plugin: "anchor"},
		{Label: i18n.N("Date/time"), Button: "insertdatetime", Plugin: "insertdatetime"},
	}},
	{Label: i18n.N("Code"), Options: []toolbarButtonOption{
		{Label: i18n.N("Source code (HTML)"), Button: "code", Plugin: "code"},
		{Label: i18n.N("Code block with highlighting"), Button: "codesample", Plugin: "codesample"},
	}},
	{Label: i18n.N("Other"), Options: []toolbarButtonOption{
		{Label: i18n.N("Find and replace"), Button: "searchreplace", Plugin: "searchreplace"},
		{Label: i18n.N("Fullscreen"), Button: "fullscreen", Plugin: "fullscreen"},
		{Label: i18n.N("Show block outlines"), Button: "visualblocks", Plugin: "visualblocks"},
		{Label: i18n.N("Preview window"), Button: "preview", Plugin: "preview"},
	}},
	{Label: i18n.N("News/blogs"), Options: []toolbarButtonOption{
		// insertbreak - a custom button registered via
		// editor.ui.registry.addButton (see content_edit_scripts.go and the
		// tinymce.init setup in edit_scripts.html/text_format_edit.html), so there
		// is no plugin to load; it only has to be in the toolbar string, or
		// TinyMCE silently drops it.
		{Label: i18n.N("\"Read more\" break (&lt;!--break--&gt;)"), Button: "insertbreak"},
	}},
}

// TextFormatList renders GET /admin/config/text-formats - the editable
// text formats used by FieldTypeText's per-value format picker (see
// entity.TextFormatDef).
func (c *Controller) TextFormatList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	formats, err := c.textFormatService.TextFormats(ctx)
	if err != nil {
		c.LogFailure(err, "error listing text formats")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Text formats")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["textFormats"] = formats
	data["success"] = r.URL.Query().Get("success") == "1"
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/text-formats/index.html", data)
}

// textFormatFromForm parses the fields shared by the create and edit forms.
func textFormatFromForm(r *http.Request) (label string, weight int, allowedTags string, allowClass, hasRichEditor bool, plugins, toolbar, allowedExtensions string, maxFileSizeBytes, status int, ok bool) {
	label = strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		return "", 0, "", false, false, "", "", "", 0, 0, false
	}
	weight, _ = strconv.Atoi(r.FormValue("weight"))
	allowedTags = strings.TrimSpace(r.FormValue("allowed_tags"))
	allowClass = r.FormValue("allow_class_attribute") == "1"
	hasRichEditor = r.FormValue("has_rich_editor") == "1"
	plugins = strings.TrimSpace(r.FormValue("tinymce_plugins"))
	toolbar = strings.TrimSpace(r.FormValue("tinymce_toolbar"))
	allowedExtensions = strings.Join(interfaces.ParseExtensionsForm(r.FormValue("allowed_extensions")), " ")
	maxFileSizeBytes = interfaces.MaxFileSizeBytesFromMBForm(r.FormValue("max_filesize_mb"))
	status = 0
	if r.FormValue("status") == "1" {
		status = 1
	}
	return label, weight, allowedTags, allowClass, hasRichEditor, plugins, toolbar, allowedExtensions, maxFileSizeBytes, status, true
}

// textFiltersFromForm - the format's text filters from the form's
// "filters" ticks: the ticked ones of those offered (service.TextFilters),
// and the saved ones not offered (a disabled module's) kept.
func textFiltersFromForm(r *http.Request, saved []string) string {
	offered := map[string]bool{}
	for _, f := range service.TextFilters() {
		offered[f.Name] = true
	}
	var names []string
	for _, n := range saved {
		if !offered[n] {
			names = append(names, n)
		}
	}
	for _, n := range r.Form["filters"] {
		if offered[n] && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	return strings.Join(names, " ")
}

// TextFormatCreate handles POST /admin/config/text-formats - the list
// page's label-only quick-add; everything else is configured on the edit
// page. The machine name is transliterated from the label. The new format
// starts inert (no allowed tags, i.e. plain text; everything else off) with
// the lowest Weight, so it sorts first and wins picker fallbacks (see
// entity.TextFormatDef.Weight).
func (c *Controller) TextFormatCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/text-formats", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		http.Redirect(w, r, "/admin/config/text-formats?error=empty", http.StatusSeeOther)
		return
	}

	machineName := strings.ReplaceAll(slug.Generate(label), "-", "_")
	if machineName == "" {
		machineName = "format"
	}

	existing, err := c.textFormatService.TextFormats(ctx)
	if err != nil {
		c.LogFailure(err, "error listing text formats before create")
		httputil.InternalServerError(w)
		return
	}
	weight := 0
	for _, f := range existing {
		if f.Weight <= weight {
			weight = f.Weight - 10
		}
	}

	currentUser, err := c.GetCurrentUser(r)
	createdBy := 0
	if err == nil && currentUser != nil {
		createdBy = currentUser.ID
	}

	_, err = c.textFormatService.CreateTextFormat(ctx, entity.TextFormatDef{
		MachineName: machineName,
		Label:       label,
		Weight:      weight,
		Status:      1,
		CreatedBy:   createdBy,
	})
	if err != nil {
		if errors.Is(err, service.ErrTextFormatDuplicate) {
			http.Redirect(w, r, "/admin/config/text-formats?error=duplicate", http.StatusSeeOther)
			return
		}
		c.LogFailure(err, "error creating text format", "machineName", machineName)
		http.Redirect(w, r, "/admin/config/text-formats?error=create", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/text-formats?success=1", http.StatusSeeOther)
}

// TextFormatsReorder handles POST /admin/config/text-formats/reorder -
// the drag-and-drop table: "order" is a CSV of IDs. The first ID gets the
// lowest Weight (step 10), matching TextFormats()' ascending sort, so the
// top row sorts first and wins picker fallbacks.
func (c *Controller) TextFormatsReorder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/text-formats", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	order := strings.Split(r.FormValue("order"), ",")
	if len(order) == 0 || strings.TrimSpace(r.FormValue("order")) == "" {
		http.Redirect(w, r, "/admin/config/text-formats?error=empty-order", http.StatusSeeOther)
		return
	}

	formats, err := c.textFormatService.TextFormats(ctx)
	if err != nil {
		c.LogFailure(err, "error listing text formats before reorder")
		httputil.InternalServerError(w)
		return
	}
	byID := make(map[int]entity.TextFormatDef, len(formats))
	for _, f := range formats {
		byID[f.ID] = f
	}

	for i, idStr := range order {
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			continue
		}
		f, ok := byID[id]
		if !ok {
			continue
		}
		f.Weight = i * 10
		if _, err := c.textFormatService.UpdateTextFormat(ctx, f); err != nil {
			c.LogFailure(err, "error updating text format weight", "id", id)
			http.Redirect(w, r, "/admin/config/text-formats?error=reorder", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/admin/config/text-formats?success=1", http.StatusSeeOther)
}

// TextFormatEdit renders GET /admin/config/text-formats/{id}/edit - one
// format's settings.
func (c *Controller) TextFormatEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	format, err := c.textFormatService.TextFormatByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the text format", "formatID", id)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Format \"@label\"", "@label", format.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", i18n.T(ctx, "Format @label", "@label", format.Label))}
	data["format"] = format
	data["toolbarGroups"] = toolbarGroups
	data["textFilters"] = service.TextFilters()
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"

	c.RenderOrFail(w, r, "admin/text-formats/edit.html", data)
}

// TextFormatUpdate handles POST /admin/config/text-formats/{id}. The
// machine name is an identity string and is immutable (fields reference
// formats by ID in WidgetConfig["allowed_text_formats"]).
func (c *Controller) TextFormatUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/text-formats", http.StatusSeeOther)
		return
	}
	editRedirect := fmt.Sprintf("/admin/config/text-formats/%d/edit", id)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, editRedirect, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	label, weight, allowedTags, allowClass, hasRichEditor, plugins, toolbar, allowedExtensions, maxFileSizeBytes, status, ok := textFormatFromForm(r)
	if !ok {
		http.Redirect(w, r, editRedirect+"?error=empty", http.StatusSeeOther)
		return
	}
	existing, err := c.textFormatService.TextFormatByID(ctx, id)
	if err != nil {
		c.LogFailure(err, "error loading text format", "id", id)
		http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		return
	}

	_, err = c.textFormatService.UpdateTextFormat(ctx, entity.TextFormatDef{
		ID:                  id,
		Label:               label,
		Weight:              weight,
		AllowedTags:         allowedTags,
		AllowClassAttribute: allowClass,
		HasRichEditor:       hasRichEditor,
		TinyMCEPlugins:      plugins,
		TinyMCEToolbar:      toolbar,
		AllowedExtensions:   allowedExtensions,
		MaxFileSizeBytes:    maxFileSizeBytes,
		Status:              status,
		Filters:             textFiltersFromForm(r, existing.FilterNames()),
	})
	if err != nil {
		c.LogFailure(err, "error updating text format", "id", id)
		http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, editRedirect+"?success=1", http.StatusSeeOther)
}

// TextFormatDeleteConfirm renders GET
// /admin/config/text-formats/{id}/delete - a confirmation page.
func (c *Controller) TextFormatDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	format, err := c.textFormatService.TextFormatByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the text format", "formatID", id)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Delete format \"@label\"", "@label", format.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", i18n.T(ctx, "Delete format @label", "@label", format.Label))}
	data["format"] = format

	c.RenderOrFail(w, r, "admin/text-formats/delete.html", data)
}

// TextFormatDelete handles POST /admin/config/text-formats/{id}/delete.
// Formats still referenced may be deleted: saved content stays sanitized
// as it was, and the fields' allowed-format lists and the editor picker
// drop the missing ID (see entity.FieldInstanceDef.AllowedTextFormatIDs
// and TextFormatRepository.DeleteTextFormat).
func (c *Controller) TextFormatDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/text-formats", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/text-formats?error=invalid", http.StatusSeeOther)
		return
	}

	if err := c.textFormatService.DeleteTextFormat(ctx, id); err != nil {
		c.LogFailure(err, "error deleting text format", "id", id)
		http.Redirect(w, r, "/admin/config/text-formats?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/text-formats?deleted=1", http.StatusSeeOther)
}
