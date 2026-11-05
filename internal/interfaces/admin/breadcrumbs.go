package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Breadcrumbs - rules mapping "content type + (optional) taxonomy field
// and terms" to a menu link. Pages whose path is in a menu get a trail
// without any rule.

const breadcrumbRulesURL = "/admin/config/breadcrumbs"

// ruleFieldOption - a taxonomy field of a content type, for the
// rule form's field select (filtered by the chosen type in JS).
type ruleFieldOption struct {
	ContentType    string
	FieldStorageID int
	Label          string
	VocabularyID   int
}

// ruleTermOption - a vocabulary term in tree order.
type ruleTermOption struct {
	ID       int
	Name     string
	Depth    int
	Selected bool
}

// Indent - the option label's nesting prefix.
func (o ruleTermOption) Indent() string { return strings.Repeat("- ", o.Depth) }

// breadcrumbRuleRow - a rule as the list shows it.
type breadcrumbRuleRow struct {
	Rule      entity.BreadcrumbRuleDef
	TypeLabel string
	Field     string
	Terms     []string
	// Link - the menu link's titles from the root; empty when the link
	// is gone or disabled (the rule then falls through).
	Link string
}

// ruleFormOptions - content types and their taxonomy fields.
func (c *Controller) ruleFormOptions(ctx context.Context) (entity.ContentTypeDefs, []ruleFieldOption, error) {
	types, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		return nil, nil, err
	}
	var fields []ruleFieldOption
	for _, ct := range types {
		defs, err := c.contentTypeService.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, f := range defs {
			if f.FieldType == entity.FieldTypeTaxonomy && f.VocabularyID() > 0 {
				fields = append(fields, ruleFieldOption{ContentType: ct.MachineName, FieldStorageID: f.FieldStorageID, Label: f.Label, VocabularyID: f.VocabularyID()})
			}
		}
	}
	return types, fields, nil
}

func findRuleField(fields []ruleFieldOption, contentType string, storageID int) (ruleFieldOption, bool) {
	for _, f := range fields {
		if f.ContentType == contentType && f.FieldStorageID == storageID {
			return f, true
		}
	}
	return ruleFieldOption{}, false
}

// ruleTermTree - the vocabulary's terms in tree order (parents
// before children, siblings in the repository's order).
func ruleTermTree(terms entity.TaxonomyTerms, selected func(int) bool) []ruleTermOption {
	known := make(map[int]bool, len(terms))
	for _, t := range terms {
		known[t.ID] = true
	}
	children := map[int][]entity.TaxonomyTerm{}
	for _, t := range terms {
		parent := t.ParentID
		if !known[parent] {
			parent = 0
		}
		children[parent] = append(children[parent], t)
	}
	out := make([]ruleTermOption, 0, len(terms))
	var walk func(parent, depth int)
	walk = func(parent, depth int) {
		for _, t := range children[parent] {
			out = append(out, ruleTermOption{ID: t.ID, Name: t.Name, Depth: depth, Selected: selected(t.ID)})
			if depth < 32 {
				walk(t.ID, depth+1)
			}
		}
	}
	walk(0, 0)
	return out
}

// breadcrumbRuleFromForm reads the type, field and menu link; terms are
// kept only when they belong to the chosen field's vocabulary.
func (c *Controller) breadcrumbRuleFromForm(ctx context.Context, r *http.Request, fields []ruleFieldOption) (entity.BreadcrumbRuleDef, bool) {
	rule := entity.BreadcrumbRuleDef{ContentType: strings.TrimSpace(r.FormValue("content_type"))}
	rule.MenuLinkID, _ = strconv.Atoi(r.FormValue("menu_link_id"))
	if rule.ContentType == "" || rule.MenuLinkID <= 0 {
		return rule, false
	}
	rule.FieldStorageID, rule.TermIDs = c.termConditionFromForm(ctx, r, fields, rule.ContentType)
	return rule, true
}

// termConditionFromForm - a rule's optional condition: the chosen
// taxonomy field of contentType and the submitted terms of its
// vocabulary (others dropped); nil, "" without a field.
func (c *Controller) termConditionFromForm(ctx context.Context, r *http.Request, fields []ruleFieldOption, contentType string) (*int, string) {
	storageID, _ := strconv.Atoi(r.FormValue("field_storage_id"))
	field, ok := findRuleField(fields, contentType, storageID)
	if !ok {
		return nil, ""
	}
	terms, err := c.taxonomyService.TermsByVocabularyID(ctx, field.VocabularyID)
	if err != nil {
		c.LogFailure(err, "error loading terms for a rule", "vid", field.VocabularyID)
		return &field.FieldStorageID, ""
	}
	inVocabulary := make(map[int]bool, len(terms))
	for _, t := range terms {
		inVocabulary[t.ID] = true
	}
	var ids []string
	for _, v := range r.Form["term_ids"] {
		if id, err := strconv.Atoi(v); err == nil && inVocabulary[id] {
			ids = append(ids, strconv.Itoa(id))
		}
	}
	return &field.FieldStorageID, strings.Join(ids, ",")
}

// ruleConditionLabels - a rule condition's field label and term names
// for a list (termNames caches vocabulary → term → name).
func (c *Controller) ruleConditionLabels(ctx context.Context, fields []ruleFieldOption, contentType string, storageID int, termIDs []int, termNames map[int]map[int]string) (string, []string) {
	field, ok := findRuleField(fields, contentType, storageID)
	if !ok {
		return "", nil
	}
	names, loaded := termNames[field.VocabularyID]
	if !loaded {
		names = map[int]string{}
		if terms, err := c.taxonomyService.TermsByVocabularyID(ctx, field.VocabularyID); err == nil {
			for _, t := range terms {
				names[t.ID] = t.Name
			}
		} else {
			c.LogFailure(err, "listing the vocabulary's terms", "vocabularyID", field.VocabularyID)
		}
		termNames[field.VocabularyID] = names
	}
	var out []string
	for _, id := range termIDs {
		if name, ok := names[id]; ok {
			out = append(out, name)
		}
	}
	return field.Label, out
}

// ruleConditionTerms - the term tree of a rule's field (nil without one).
func (c *Controller) ruleConditionTerms(ctx context.Context, fields []ruleFieldOption, contentType string, storageID int, selected func(int) bool) ([]ruleTermOption, error) {
	field, ok := findRuleField(fields, contentType, storageID)
	if !ok {
		return nil, nil
	}
	vocabularyTerms, err := c.taxonomyService.TermsByVocabularyID(ctx, field.VocabularyID)
	if err != nil {
		return nil, err
	}
	return ruleTermTree(vocabularyTerms, selected), nil
}

// BreadcrumbRuleList renders GET /admin/config/breadcrumbs.
func (c *Controller) BreadcrumbRuleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rules, err := c.breadcrumbService.Rules(ctx)
	if err != nil {
		c.LogFailure(err, "error listing breadcrumb rules")
		httputil.InternalServerError(w)
		return
	}
	types, fields, err := c.ruleFormOptions(ctx)
	if err != nil {
		c.LogFailure(err, "error loading content types for breadcrumb rules")
		httputil.InternalServerError(w)
		return
	}
	typeLabels := make(map[string]string, len(types))
	for _, ct := range types {
		typeLabels[ct.MachineName] = ct.Label
	}
	termNames := map[int]map[int]string{} // vocabulary → term → name
	rows := make([]breadcrumbRuleRow, 0, len(rules))
	for _, rule := range rules {
		row := breadcrumbRuleRow{Rule: rule, TypeLabel: typeLabels[rule.ContentType], Link: c.breadcrumbService.LinkTitlePath(rule.MenuLinkID)}
		if row.TypeLabel == "" {
			row.TypeLabel = rule.ContentType
		}
		row.Field, row.Terms = c.ruleConditionLabels(ctx, fields, rule.ContentType, rule.FieldStorageIDValue(), rule.TermIDList(), termNames)
		rows = append(rows, row)
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Breadcrumbs")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Breadcrumbs"))}
	data["rows"] = rows
	data["contentTypes"] = types
	data["fields"] = fields
	data["menuLinks"] = c.breadcrumbService.MenuLinkOptions()
	data["success"] = r.URL.Query().Get("success") == "1"
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/breadcrumbs/index.html", data)
}

// BreadcrumbRuleCreate handles POST /admin/config/breadcrumbs; a rule
// with a field goes on to its edit page to pick the terms.
func (c *Controller) BreadcrumbRuleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, breadcrumbRulesURL+"?error=empty", http.StatusSeeOther)
		return
	}
	_, fields, err := c.ruleFormOptions(ctx)
	if err != nil {
		c.LogFailure(err, "error loading content types for breadcrumb rule")
		httputil.InternalServerError(w)
		return
	}
	rule, ok := c.breadcrumbRuleFromForm(ctx, r, fields)
	if !ok {
		http.Redirect(w, r, breadcrumbRulesURL+"?error=empty", http.StatusSeeOther)
		return
	}
	created, err := c.breadcrumbService.CreateRule(ctx, rule)
	if err != nil {
		c.LogFailure(err, "error creating breadcrumb rule")
		http.Redirect(w, r, breadcrumbRulesURL+"?error=create", http.StatusSeeOther)
		return
	}
	if created.FieldStorageID != nil && created.TermIDs == "" {
		http.Redirect(w, r, breadcrumbRulesURL+"/"+strconv.Itoa(created.ID)+"/edit?error=terms", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, breadcrumbRulesURL+"?success=1", http.StatusSeeOther)
}

// BreadcrumbRuleEdit renders GET /admin/config/breadcrumbs/{id}/edit.
func (c *Controller) BreadcrumbRuleEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	rule, err := c.breadcrumbService.RuleByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the breadcrumb rule", "id", id)
		return
	}
	types, fields, err := c.ruleFormOptions(ctx)
	if err != nil {
		c.LogFailure(err, "error loading content types for breadcrumb rule")
		httputil.InternalServerError(w)
		return
	}
	terms, err := c.ruleConditionTerms(ctx, fields, rule.ContentType, rule.FieldStorageIDValue(), rule.HasTerm)
	if err != nil {
		c.LogFailure(err, "error loading terms for breadcrumb rule")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Breadcrumb rule")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Breadcrumb rule"))}
	data["rule"] = rule
	data["contentTypes"] = types
	data["fields"] = fields
	data["terms"] = terms
	data["menuLinks"] = c.breadcrumbService.MenuLinkOptions()
	data["linkFound"] = c.breadcrumbService.LinkTitlePath(rule.MenuLinkID) != ""
	data["success"] = r.URL.Query().Get("success") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/breadcrumbs/edit.html", data)
}

// BreadcrumbRuleUpdate handles POST /admin/config/breadcrumbs/{id}.
func (c *Controller) BreadcrumbRuleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, breadcrumbRulesURL, http.StatusSeeOther)
		return
	}
	editURL := breadcrumbRulesURL + "/" + strconv.Itoa(id) + "/edit"
	existing, err := c.breadcrumbService.RuleByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the breadcrumb rule", "id", id)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, editURL+"?error=empty", http.StatusSeeOther)
		return
	}
	_, fields, err := c.ruleFormOptions(ctx)
	if err != nil {
		c.LogFailure(err, "error loading content types for breadcrumb rule")
		httputil.InternalServerError(w)
		return
	}
	rule, ok := c.breadcrumbRuleFromForm(ctx, r, fields)
	if !ok {
		http.Redirect(w, r, editURL+"?error=empty", http.StatusSeeOther)
		return
	}
	rule.ID = id
	rule.Weight = existing.Weight
	if _, err := c.breadcrumbService.UpdateRule(ctx, rule); err != nil {
		c.LogFailure(err, "error updating breadcrumb rule", "id", id)
		http.Redirect(w, r, editURL+"?error=update", http.StatusSeeOther)
		return
	}
	if rule.FieldStorageID != nil && rule.TermIDs == "" {
		http.Redirect(w, r, editURL+"?error=terms", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, editURL+"?success=1", http.StatusSeeOther)
}

// BreadcrumbRuleReorder handles POST /admin/config/breadcrumbs/reorder -
// the drag-and-drop order as a CSV of IDs in "order".
func (c *Controller) BreadcrumbRuleReorder(w http.ResponseWriter, r *http.Request) {
	var ids []int
	for _, s := range strings.Split(r.FormValue("order"), ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		http.Redirect(w, r, breadcrumbRulesURL, http.StatusSeeOther)
		return
	}
	if err := c.breadcrumbService.ReorderRules(r.Context(), ids); err != nil {
		c.LogFailure(err, "error reordering breadcrumb rules")
		http.Redirect(w, r, breadcrumbRulesURL+"?error=reorder", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, breadcrumbRulesURL+"?success=1", http.StatusSeeOther)
}

// BreadcrumbRuleDeleteConfirm renders GET /admin/config/breadcrumbs/{id}/delete.
func (c *Controller) BreadcrumbRuleDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	rule, err := c.breadcrumbService.RuleByID(r.Context(), id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the breadcrumb rule", "id", id)
		return
	}
	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Delete breadcrumb rule")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Delete breadcrumb rule"))}
	data["rule"] = rule
	data["link"] = c.breadcrumbService.LinkTitlePath(rule.MenuLinkID)

	c.RenderOrFail(w, r, "admin/breadcrumbs/delete.html", data)
}

// BreadcrumbRuleDelete handles POST /admin/config/breadcrumbs/{id}/delete.
func (c *Controller) BreadcrumbRuleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, breadcrumbRulesURL, http.StatusSeeOther)
		return
	}
	if err := c.breadcrumbService.DeleteRule(r.Context(), id); err != nil {
		c.LogFailure(err, "error deleting breadcrumb rule", "id", id)
		http.Redirect(w, r, breadcrumbRulesURL+"?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, breadcrumbRulesURL+"?deleted=1", http.StatusSeeOther)
}
