package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// SEO - token templates for node pages' title and meta description: the
// default for every node, per content type, and rules mapping
// "content type + taxonomy field + terms" to templates.

const seoAdminURL = "/admin/config/seo"

// seoTypeRow - a content type with its templates.
type seoTypeRow struct {
	Type     entity.ContentTypeDef
	Template entity.SEOTemplate
}

// seoVocabularyRow - a vocabulary with its term pages' templates.
type seoVocabularyRow struct {
	Vocabulary entity.TaxonomyVocabulary
	Template   entity.SEOTemplate
}

// seoRuleRow - a rule as the list shows it.
type seoRuleRow struct {
	Rule      entity.SEORuleDef
	TypeLabel string
	Field     string
	Terms     []string
}

// seoTemplateFromForm - the title/description template fields.
func seoTemplateFromForm(r *http.Request) (string, string) {
	return strings.TrimSpace(r.FormValue("title_template")), strings.TrimSpace(r.FormValue("description_template"))
}

// SEOPage renders GET /admin/config/seo.
func (c *Controller) SEOPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	templates, err := c.seoService.Templates(ctx)
	if err != nil {
		c.LogFailure(err, "error loading seo templates")
		httputil.InternalServerError(w)
		return
	}
	rules, err := c.seoService.Rules(ctx)
	if err != nil {
		c.LogFailure(err, "error loading seo rules")
		httputil.InternalServerError(w)
		return
	}
	types, fields, err := c.ruleFormOptions(ctx)
	if err != nil {
		c.LogFailure(err, "error loading content types for seo")
		httputil.InternalServerError(w)
		return
	}
	typeRows := make([]seoTypeRow, 0, len(types))
	typeLabels := make(map[string]string, len(types))
	for _, ct := range types {
		typeRows = append(typeRows, seoTypeRow{Type: ct, Template: templates[entity.SEOTemplateTypeScope(ct.MachineName)]})
		typeLabels[ct.MachineName] = ct.Label
	}
	termNames := map[int]map[int]string{}
	ruleRows := make([]seoRuleRow, 0, len(rules))
	for _, rule := range rules {
		row := seoRuleRow{Rule: rule, TypeLabel: typeLabels[rule.ContentType]}
		if row.TypeLabel == "" {
			row.TypeLabel = rule.ContentType
		}
		row.Field, row.Terms = c.ruleConditionLabels(ctx, fields, rule.ContentType, rule.FieldStorageIDValue(), rule.TermIDList(), termNames)
		ruleRows = append(ruleRows, row)
	}

	vocabularies, err := c.taxonomyService.Vocabularies(ctx)
	if err != nil {
		c.LogFailure(err, "error loading vocabularies for seo")
		httputil.InternalServerError(w)
		return
	}
	vocabularyRows := make([]seoVocabularyRow, 0, len(vocabularies))
	for _, v := range vocabularies {
		vocabularyRows = append(vocabularyRows, seoVocabularyRow{Vocabulary: v, Template: templates[entity.SEOTemplateVocabularyScope(v.ID)]})
	}

	data := c.getAdminData(r)
	data["title"] = "SEO"
	data["termDefaultTemplate"] = templates[entity.SEOTemplateTermScope]
	data["vocabularies"] = vocabularyRows
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", "SEO")}
	data["defaultTemplate"] = templates[entity.SEOTemplateDefaultScope]
	data["schemaTypes"] = entity.SchemaTypes
	data["types"] = typeRows
	data["rules"] = ruleRows
	data["contentTypes"] = types
	data["fields"] = fields
	data["success"] = r.URL.Query().Get("success") == "1"
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/seo/index.html", data)
}

// SEODefaultSave handles POST /admin/config/seo/default.
func (c *Controller) SEODefaultSave(w http.ResponseWriter, r *http.Request) {
	title, desc := seoTemplateFromForm(r)
	schemaType, schemaProps := seoSchemaFromForm(r, false)
	if err := c.seoService.SaveTemplate(r.Context(), entity.SEOTemplate{Scope: entity.SEOTemplateDefaultScope, TitleTemplate: title, DescriptionTemplate: desc, ImageTemplate: strings.TrimSpace(r.FormValue("image_template")),
		SchemaType: schemaType, SchemaProps: schemaProps}); err != nil {
		c.LogFailure(err, "error saving default seo template")
		http.Redirect(w, r, seoAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, seoAdminURL+"?success=1", http.StatusSeeOther)
}

// SEOTermDefaultSave handles POST /admin/config/seo/term-default.
func (c *Controller) SEOTermDefaultSave(w http.ResponseWriter, r *http.Request) {
	title, desc := seoTemplateFromForm(r)
	if err := c.seoService.SaveTemplate(r.Context(), entity.SEOTemplate{Scope: entity.SEOTemplateTermScope, TitleTemplate: title, DescriptionTemplate: desc}); err != nil {
		c.LogFailure(err, "error saving term pages seo template")
		http.Redirect(w, r, seoAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, seoAdminURL+"?success=1", http.StatusSeeOther)
}

// SEOVocabularyEdit renders GET /admin/config/seo/vocabularies/{id}/edit.
func (c *Controller) SEOVocabularyEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	vocabulary, err := c.taxonomyService.VocabularyByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the vocabulary", "id", id)
		return
	}
	if vocabulary == nil {
		c.RenderNotFound(w, r)
		return
	}
	templates, err := c.seoService.Templates(ctx)
	if err != nil {
		c.LogFailure(err, "error loading seo templates")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "SEO: terms \"@name\"", "@name", vocabulary.Name)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "SEO: terms @name - Admin panel", "@name", vocabulary.Name)}
	data["vocabulary"] = vocabulary
	data["template"] = templates[entity.SEOTemplateVocabularyScope(vocabulary.ID)]
	data["defaultTemplate"] = templates[entity.SEOTemplateTermScope]

	c.RenderOrFail(w, r, "admin/seo/vocabulary_edit.html", data)
}

// SEOVocabularySave handles POST /admin/config/seo/vocabularies/{id}.
func (c *Controller) SEOVocabularySave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, seoAdminURL, http.StatusSeeOther)
		return
	}
	if v, err := c.taxonomyService.VocabularyByID(ctx, id); err != nil || v == nil {
		c.LogFailure(err, "loading the vocabulary", "vocabularyID", id)
		c.RenderNotFound(w, r)
		return
	}
	title, desc := seoTemplateFromForm(r)
	if err := c.seoService.SaveTemplate(ctx, entity.SEOTemplate{Scope: entity.SEOTemplateVocabularyScope(id), TitleTemplate: title, DescriptionTemplate: desc}); err != nil {
		c.LogFailure(err, "error saving vocabulary seo template", "vocabulary_id", id)
		http.Redirect(w, r, seoAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, seoAdminURL+"?success=1", http.StatusSeeOther)
}

// seoTypeFields - a content type's fields for the token help.
func (c *Controller) seoTypeFields(r *http.Request, contentType string) entity.FieldInstanceDefs {
	ct, err := c.contentTypeService.ContentTypeByMachineName(r.Context(), contentType)
	if err != nil || ct == nil {
		c.LogFailure(err, "loading the content type", "machineName", contentType)
		return nil
	}
	fields, err := c.contentTypeService.FieldsByContentTypeID(r.Context(), ct.ID)
	if err != nil {
		c.LogFailure(err, "loading the content type's fields", "contentTypeID", ct.ID)
		return nil
	}
	return fields
}

// SEOTypeEdit renders GET /admin/config/seo/types/{id}/edit.
func (c *Controller) SEOTypeEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	ct, err := c.contentTypeService.ContentTypeByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the content type", "id", id)
		return
	}
	if ct == nil {
		c.RenderNotFound(w, r)
		return
	}
	templates, err := c.seoService.Templates(ctx)
	if err != nil {
		c.LogFailure(err, "error loading seo templates")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = "SEO: " + ct.Label
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", "SEO: "+ct.Label)}
	data["contentType"] = ct
	data["template"] = templates[entity.SEOTemplateTypeScope(ct.MachineName)]
	data["defaultTemplate"] = templates[entity.SEOTemplateDefaultScope]
	data["typeFields"] = c.seoTypeFields(r, ct.MachineName)
	data["schemaTypes"] = entity.SchemaTypes
	data["schemaSample"] = c.schemaSample(ctx, ct)

	c.RenderOrFail(w, r, "admin/seo/type_edit.html", data)
}

// SEOTypeSave handles POST /admin/config/seo/types/{id}.
func (c *Controller) SEOTypeSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, seoAdminURL, http.StatusSeeOther)
		return
	}
	ct, err := c.contentTypeService.ContentTypeByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the content type", "id", id)
		return
	}
	if ct == nil {
		c.RenderNotFound(w, r)
		return
	}
	title, desc := seoTemplateFromForm(r)
	schemaType, schemaProps := seoSchemaFromForm(r, true)
	if err := c.seoService.SaveTemplate(ctx, entity.SEOTemplate{Scope: entity.SEOTemplateTypeScope(ct.MachineName), TitleTemplate: title, DescriptionTemplate: desc, ImageTemplate: strings.TrimSpace(r.FormValue("image_template")),
		SchemaType: schemaType, SchemaProps: schemaProps}); err != nil {
		c.LogFailure(err, "error saving seo type template", "type", ct.MachineName)
		http.Redirect(w, r, seoAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, seoAdminURL+"?success=1", http.StatusSeeOther)
}

// SEORuleCreate handles POST /admin/config/seo/rules; a rule with a field
// goes on to its edit page to pick the terms.
func (c *Controller) SEORuleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, seoAdminURL+"?error=empty", http.StatusSeeOther)
		return
	}
	_, fields, err := c.ruleFormOptions(ctx)
	if err != nil {
		c.LogFailure(err, "error loading content types for seo rule")
		httputil.InternalServerError(w)
		return
	}
	rule := entity.SEORuleDef{ContentType: strings.TrimSpace(r.FormValue("content_type"))}
	rule.TitleTemplate, rule.DescriptionTemplate = seoTemplateFromForm(r)
	if rule.ContentType == "" || (rule.TitleTemplate == "" && rule.DescriptionTemplate == "") {
		http.Redirect(w, r, seoAdminURL+"?error=empty", http.StatusSeeOther)
		return
	}
	rule.FieldStorageID, rule.TermIDs = c.termConditionFromForm(ctx, r, fields, rule.ContentType)
	created, err := c.seoService.CreateRule(ctx, rule)
	if err != nil {
		c.LogFailure(err, "error creating seo rule")
		http.Redirect(w, r, seoAdminURL+"?error=create", http.StatusSeeOther)
		return
	}
	if created.FieldStorageID != nil && created.TermIDs == "" {
		http.Redirect(w, r, seoAdminURL+"/rules/"+strconv.Itoa(created.ID)+"/edit?error=terms", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, seoAdminURL+"?success=1", http.StatusSeeOther)
}

// SEORuleEdit renders GET /admin/config/seo/rules/{id}/edit.
func (c *Controller) SEORuleEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	rule, err := c.seoService.RuleByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the SEO rule", "id", id)
		return
	}
	types, fields, err := c.ruleFormOptions(ctx)
	if err != nil {
		c.LogFailure(err, "error loading content types for seo rule")
		httputil.InternalServerError(w)
		return
	}
	terms, err := c.ruleConditionTerms(ctx, fields, rule.ContentType, rule.FieldStorageIDValue(), rule.HasTerm)
	if err != nil {
		c.LogFailure(err, "error loading terms for seo rule")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "SEO rule")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "SEO rule"))}
	data["rule"] = rule
	data["contentTypes"] = types
	data["fields"] = fields
	data["terms"] = terms
	data["typeFields"] = c.seoTypeFields(r, rule.ContentType)
	data["success"] = r.URL.Query().Get("success") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/seo/rule_edit.html", data)
}

// SEORuleUpdate handles POST /admin/config/seo/rules/{id}.
func (c *Controller) SEORuleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, seoAdminURL, http.StatusSeeOther)
		return
	}
	editURL := seoAdminURL + "/rules/" + strconv.Itoa(id) + "/edit"
	existing, err := c.seoService.RuleByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the SEO rule", "id", id)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, editURL+"?error=empty", http.StatusSeeOther)
		return
	}
	_, fields, err := c.ruleFormOptions(ctx)
	if err != nil {
		c.LogFailure(err, "error loading content types for seo rule")
		httputil.InternalServerError(w)
		return
	}
	rule := entity.SEORuleDef{ID: id, Weight: existing.Weight, ContentType: strings.TrimSpace(r.FormValue("content_type"))}
	rule.TitleTemplate, rule.DescriptionTemplate = seoTemplateFromForm(r)
	if rule.ContentType == "" || (rule.TitleTemplate == "" && rule.DescriptionTemplate == "") {
		http.Redirect(w, r, editURL+"?error=empty", http.StatusSeeOther)
		return
	}
	rule.FieldStorageID, rule.TermIDs = c.termConditionFromForm(ctx, r, fields, rule.ContentType)
	if err := c.seoService.UpdateRule(ctx, rule); err != nil {
		c.LogFailure(err, "error updating seo rule", "id", id)
		http.Redirect(w, r, editURL+"?error=update", http.StatusSeeOther)
		return
	}
	if rule.FieldStorageID != nil && rule.TermIDs == "" {
		http.Redirect(w, r, editURL+"?error=terms", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, editURL+"?success=1", http.StatusSeeOther)
}

// SEORuleReorder handles POST /admin/config/seo/rules/reorder - the
// drag-and-drop order as a CSV of IDs in "order".
func (c *Controller) SEORuleReorder(w http.ResponseWriter, r *http.Request) {
	var ids []int
	for _, s := range strings.Split(r.FormValue("order"), ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		http.Redirect(w, r, seoAdminURL, http.StatusSeeOther)
		return
	}
	if err := c.seoService.ReorderRules(r.Context(), ids); err != nil {
		c.LogFailure(err, "error reordering seo rules")
		http.Redirect(w, r, seoAdminURL+"?error=reorder", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, seoAdminURL+"?success=1", http.StatusSeeOther)
}

// SEORuleDeleteConfirm renders GET /admin/config/seo/rules/{id}/delete.
func (c *Controller) SEORuleDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	rule, err := c.seoService.RuleByID(r.Context(), id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the SEO rule", "id", id)
		return
	}
	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Delete SEO rule")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Delete SEO rule"))}
	data["rule"] = rule

	c.RenderOrFail(w, r, "admin/seo/rule_delete.html", data)
}

// SEORuleDelete handles POST /admin/config/seo/rules/{id}/delete.
func (c *Controller) SEORuleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, seoAdminURL, http.StatusSeeOther)
		return
	}
	if err := c.seoService.DeleteRule(r.Context(), id); err != nil {
		c.LogFailure(err, "error deleting seo rule", "id", id)
		http.Redirect(w, r, seoAdminURL+"?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, seoAdminURL+"?deleted=1", http.StatusSeeOther)
}
