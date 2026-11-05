package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// pathVocabularyID parses the "vid" path value as a positive int; ok is
// false if missing or invalid. Term routes carry both "vid" and "id"
// (see BaseController.PathID).
func (c *Controller) pathVocabularyID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("vid"))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// resolveSlug returns the trimmed manual slug if provided, otherwise a slug generated from name.
func resolveSlug(manual, name string) string {
	manual = strings.TrimSpace(manual)
	if manual != "" {
		return manual
	}
	return slug.Generate(name)
}

// TaxonomyVocabularyList renders GET /admin/config/taxonomy - the list of taxonomy dictionaries.
func (c *Controller) TaxonomyVocabularyList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vocabularies, err := c.taxonomyService.Vocabularies(ctx)
	if err != nil {
		c.LogFailure(err, "error listing taxonomy vocabularies")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Vocabularies")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Vocabularies"))}
	data["vocabularies"] = vocabularies
	data["success"] = r.URL.Query().Get("success") == "1"
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/taxonomy/index.html", data)
}

// TaxonomyVocabulariesSortOrderUpdate handles POST
// /admin/config/taxonomy/reorder - the vocabularies list's drag-and-drop:
// "order" lists every vocabulary ID in its final position, which becomes
// its Weight. Uses UpdateVocabularySortOrder so name/slug/description are
// never touched.
func (c *Controller) TaxonomyVocabulariesSortOrderUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/taxonomy", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	orderRaw := strings.TrimSpace(r.FormValue("order"))
	if orderRaw == "" {
		http.Redirect(w, r, "/admin/config/taxonomy?error=empty-order", http.StatusSeeOther)
		return
	}

	order := strings.Split(orderRaw, ",")
	for i, idStr := range order {
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			continue
		}
		if err := c.taxonomyService.UpdateVocabularySortOrder(ctx, id, i); err != nil {
			c.LogFailure(err, "error updating taxonomy vocabulary sort order", "id", id)
			http.Redirect(w, r, "/admin/config/taxonomy?error=display-update", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/admin/config/taxonomy?success=1", http.StatusSeeOther)
}

// TaxonomyVocabularyCreate handles POST /admin/config/taxonomy.
func (c *Controller) TaxonomyVocabularyCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/taxonomy", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	currentUser, err := c.GetCurrentUser(r)
	if err != nil || currentUser == nil {
		http.Redirect(w, r, "/admin/config/taxonomy?error=auth", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Redirect(w, r, "/admin/config/taxonomy?error=empty", http.StatusSeeOther)
		return
	}
	termSlug := resolveSlug(r.FormValue("slug"), name)
	desc := strings.TrimSpace(r.FormValue("description"))
	weight, _ := strconv.Atoi(r.FormValue("weight"))

	termPathPrefix := service.NormalizeTermPathPrefix(r.FormValue("term_path_prefix"))
	publicTermPages := r.FormValue("public_term_pages") == "1"
	if _, err := c.taxonomyService.CreateVocabulary(ctx, name, termSlug, termPathPrefix, desc, publicTermPages, weight, currentUser.ID); err != nil {
		c.LogFailure(err, "error creating taxonomy vocabulary")
		http.Redirect(w, r, "/admin/config/taxonomy?error=create", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/taxonomy?success=1", http.StatusSeeOther)
}

// TaxonomyVocabularyEdit renders GET /admin/config/taxonomy/{id}/edit -
// one vocabulary's settings; submits to TaxonomyVocabularyUpdate.
func (c *Controller) TaxonomyVocabularyEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	vocabulary, err := c.taxonomyService.VocabularyByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the vocabulary", "vocabularyID", id)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Vocabulary \"@name\"", "@name", vocabulary.Name)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Vocabulary @name - Admin panel", "@name", vocabulary.Name)}
	data["vocabulary"] = vocabulary
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"

	c.RenderOrFail(w, r, "admin/taxonomy/vocabulary_edit.html", data)
}

// TaxonomyVocabularyUpdate handles POST /admin/config/taxonomy/{id} and
// redirects back to the vocabulary's edit page.
func (c *Controller) TaxonomyVocabularyUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/taxonomy", http.StatusSeeOther)
		return
	}
	editRedirect := "/admin/config/taxonomy/" + strconv.Itoa(id) + "/edit"
	if r.Method != http.MethodPost {
		http.Redirect(w, r, editRedirect, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Redirect(w, r, editRedirect+"?error=empty", http.StatusSeeOther)
		return
	}
	termSlug := resolveSlug(r.FormValue("slug"), name)
	desc := strings.TrimSpace(r.FormValue("description"))
	weight, _ := strconv.Atoi(r.FormValue("weight"))

	termPathPrefix := service.NormalizeTermPathPrefix(r.FormValue("term_path_prefix"))
	publicTermPages := r.FormValue("public_term_pages") == "1"
	if _, err := c.taxonomyService.UpdateVocabulary(ctx, id, name, termSlug, termPathPrefix, desc, publicTermPages, weight); err != nil {
		c.LogFailure(err, "error updating taxonomy vocabulary", "id", id)
		http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, editRedirect+"?success=1", http.StatusSeeOther)
}

// TaxonomyVocabularyDeleteConfirm renders GET
// /admin/config/taxonomy/{id}/delete - a confirmation page. Shows how many
// terms remain, since DeleteVocabulary refuses while any exist
// (ErrTaxonomyVocabularyHasTerms); the confirm button is hidden then.
func (c *Controller) TaxonomyVocabularyDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	vocabulary, err := c.taxonomyService.VocabularyByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the vocabulary", "vocabularyID", id)
		return
	}
	terms, err := c.taxonomyService.TermsByVocabularyID(ctx, id)
	if err != nil {
		c.LogFailure(err, "error counting terms for vocabulary delete page", "id", id)
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Delete vocabulary \"@name\"", "@name", vocabulary.Name)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Delete vocabulary @name - Admin panel", "@name", vocabulary.Name)}
	data["vocabulary"] = vocabulary
	data["termCount"] = len(terms)

	c.RenderOrFail(w, r, "admin/taxonomy/vocabulary_delete.html", data)
}

// TaxonomyVocabularyDelete handles POST /admin/config/taxonomy/{id}/delete.
func (c *Controller) TaxonomyVocabularyDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/taxonomy", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/taxonomy?error=invalid", http.StatusSeeOther)
		return
	}

	if err := c.taxonomyService.DeleteVocabulary(ctx, id); err != nil {
		if errors.Is(err, service.ErrTaxonomyVocabularyHasTerms) {
			http.Redirect(w, r, "/admin/config/taxonomy/"+strconv.Itoa(id)+"/delete?error=has-terms", http.StatusSeeOther)
			return
		}
		c.LogFailure(err, "error deleting taxonomy vocabulary", "id", id)
		http.Redirect(w, r, "/admin/config/taxonomy/"+strconv.Itoa(id)+"/delete?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/taxonomy?deleted=1", http.StatusSeeOther)
}
