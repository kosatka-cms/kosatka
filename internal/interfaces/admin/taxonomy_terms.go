package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TaxonomyTermList renders GET /admin/config/taxonomy/{vid}/terms - the terms of one vocabulary.
func (c *Controller) TaxonomyTermList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vid, ok := c.pathVocabularyID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}

	vocabulary, err := c.taxonomyService.VocabularyByID(ctx, vid)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the vocabulary", "vocabularyID", vid)
		return
	}

	terms, err := c.taxonomyService.TermsByVocabularyID(ctx, vid)
	if err != nil {
		c.LogFailure(err, "error listing taxonomy terms", "vid", vid)
		httputil.InternalServerError(w)
		return
	}
	sortedTerms := terms.SortByHierarchy()
	displayNames := terms.BuildHierarchy()
	depths := sortedTerms.BuildDepths()

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Terms: @name", "@name", vocabulary.Name)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Terms @name - Admin panel", "@name", vocabulary.Name)}
	data["vocabulary"] = vocabulary
	data["terms"] = sortedTerms
	data["termDisplayNames"] = displayNames
	data["termDepths"] = depths
	data["success"] = r.URL.Query().Get("success") == "1"
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/taxonomy/terms.html", data)
}

// TaxonomyTermCreate handles POST /admin/config/taxonomy/{vid}/terms.
func (c *Controller) TaxonomyTermCreate(w http.ResponseWriter, r *http.Request) {
	vid, ok := c.pathVocabularyID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	redirectBase := "/admin/config/taxonomy/" + strconv.Itoa(vid) + "/terms"
	if r.Method != http.MethodPost {
		http.Redirect(w, r, redirectBase, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	currentUser, err := c.GetCurrentUser(r)
	if err != nil || currentUser == nil {
		http.Redirect(w, r, redirectBase+"?error=auth", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Redirect(w, r, redirectBase+"?error=empty", http.StatusSeeOther)
		return
	}
	// No manual slug - "<vocabulary prefix>/<name>", like a term
	// created from a node form (TaxonomyService.GenerateTermSlug).
	termSlug := strings.TrimSpace(r.FormValue("slug"))
	if termSlug == "" {
		if termSlug, err = c.taxonomyService.GenerateTermSlug(ctx, vid, name, 0); err != nil {
			c.LogFailure(err, "error generating taxonomy term slug", "vid", vid)
			http.Redirect(w, r, redirectBase+"?error=create", http.StatusSeeOther)
			return
		}
	}
	// Output unescaped on the term page (taxonomy/term.html).
	desc := strings.TrimSpace(htmlsanitize.SanitizeAdmin(r.FormValue("description")))
	weight, _ := strconv.Atoi(r.FormValue("weight"))
	parentID, _ := strconv.Atoi(r.FormValue("parent_id"))

	if _, err := c.taxonomyService.CreateTermFull(ctx, vid, name, termSlug, desc, weight, parentID, currentUser.ID); err != nil {
		c.LogFailure(err, "error creating taxonomy term", "vid", vid)
		http.Redirect(w, r, redirectBase+"?error=create", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, redirectBase+"?success=1", http.StatusSeeOther)
}

// TaxonomyTermEdit renders GET /admin/config/taxonomy/{vid}/terms/{id}/edit -
// one term's name/slug/description/weight/parent (the list itself only
// offers drag-and-drop). Submits to TaxonomyTermUpdate.
func (c *Controller) TaxonomyTermEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vid, ok := c.pathVocabularyID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	vocabulary, err := c.taxonomyService.VocabularyByID(ctx, vid)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the vocabulary", "vocabularyID", vid)
		return
	}
	term, err := c.taxonomyService.TermByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the term", "id", id)
		return
	}
	if term.Vocabulary.ID != vid {
		c.RenderNotFound(w, r)
		return
	}
	terms, err := c.taxonomyService.TermsByVocabularyID(ctx, vid)
	if err != nil {
		c.LogFailure(err, "error loading taxonomy terms for edit page", "vid", vid)
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Term \"@term\": @vocabulary", "@term", term.Name, "@vocabulary", vocabulary.Name)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Term @term - @vocabulary - Admin panel", "@term", term.Name, "@vocabulary", vocabulary.Name)}
	data["vocabulary"] = vocabulary
	data["term"] = term
	data["terms"] = terms
	data["termDisplayNames"] = terms.BuildHierarchy()
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"
	c.AddFormSectionsData(r.Context(), data, interfaces.FormTerm, strconv.Itoa(term.ID))

	c.RenderOrFail(w, r, "admin/taxonomy/term_edit.html", data)
}

// TaxonomyTermUpdate handles POST /admin/config/taxonomy/{vid}/terms/{id}
// and redirects back to the term's edit page.
func (c *Controller) TaxonomyTermUpdate(w http.ResponseWriter, r *http.Request) {
	vid, ok := c.pathVocabularyID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	redirectBase := "/admin/config/taxonomy/" + strconv.Itoa(vid) + "/terms"
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, redirectBase, http.StatusSeeOther)
		return
	}
	editRedirect := redirectBase + "/" + strconv.Itoa(id) + "/edit"
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
	// Emptied slug - generated as for a new term (vocabulary prefix,
	// unique), the term's own current address not counting as taken.
	termSlug := strings.TrimSpace(r.FormValue("slug"))
	if termSlug == "" {
		var err error
		if termSlug, err = c.taxonomyService.GenerateTermSlug(ctx, vid, name, id); err != nil {
			c.LogFailure(err, "error generating taxonomy term slug", "vid", vid, "id", id)
			http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
			return
		}
	}
	// Output unescaped on the term page (taxonomy/term.html).
	desc := strings.TrimSpace(htmlsanitize.SanitizeAdmin(r.FormValue("description")))
	weight, _ := strconv.Atoi(r.FormValue("weight"))
	parentID, _ := strconv.Atoi(r.FormValue("parent_id"))

	if _, err := c.taxonomyService.UpdateTerm(ctx, id, name, termSlug, desc, weight, parentID); err != nil {
		switch {
		case errors.Is(err, service.ErrTaxonomyTermParentLoop):
			http.Redirect(w, r, editRedirect+"?error=parent-loop", http.StatusSeeOther)
		case errors.Is(err, service.ErrTaxonomyTermParentInvalid):
			http.Redirect(w, r, editRedirect+"?error=parent-invalid", http.StatusSeeOther)
		default:
			c.LogFailure(err, "error updating taxonomy term", "id", id)
			http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		}
		return
	}
	if err := c.taxonomyService.UpdateTermPageTitle(ctx, id, r.FormValue("page_title")); err != nil {
		c.LogFailure(err, "error updating taxonomy term page title", "id", id)
		http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		return
	}
	if err := interfaces.SaveFormSections(ctx, interfaces.FormTerm, strconv.Itoa(id), r); err != nil {
		c.LogFailure(err, "error saving the modules' sections of a term", "id", id)
		http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, editRedirect+"?success=1", http.StatusSeeOther)
}

// TaxonomyTermsSortOrderUpdate handles POST
// /admin/config/taxonomy/{vid}/terms/reorder - the terms list's
// drag-and-drop: "order" lists every term ID in its final depth-first
// position (becoming its Weight), and "parent_id_<id>" carries a moved
// term's new parent. The script moves a subtree as one block, so a term
// can't become a child of its own descendant (UpdateTermSortOrder refuses
// it anyway). Uses UpdateTermSortOrder rather than UpdateTerm so
// name/slug/description are never touched (nor re-validated, e.g. an
// over-long stored description).
func (c *Controller) TaxonomyTermsSortOrderUpdate(w http.ResponseWriter, r *http.Request) {
	vid, ok := c.pathVocabularyID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	redirectBase := "/admin/config/taxonomy/" + strconv.Itoa(vid) + "/terms"
	if r.Method != http.MethodPost {
		http.Redirect(w, r, redirectBase, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	orderRaw := strings.TrimSpace(r.FormValue("order"))
	if orderRaw == "" {
		http.Redirect(w, r, redirectBase+"?error=empty-order", http.StatusSeeOther)
		return
	}

	terms, err := c.taxonomyService.TermsByVocabularyID(ctx, vid)
	if err != nil {
		c.LogFailure(err, "error loading taxonomy terms for reorder", "vid", vid)
		http.Redirect(w, r, redirectBase+"?error=display-update", http.StatusSeeOther)
		return
	}
	termByID := make(map[int]entity.TaxonomyTerm, len(terms))
	for _, t := range terms {
		termByID[t.ID] = t
	}

	order := strings.Split(orderRaw, ",")
	for i, idStr := range order {
		termID, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			continue
		}
		term, ok := termByID[termID]
		if !ok {
			continue
		}
		parentID, err := strconv.Atoi(r.FormValue("parent_id_" + strconv.Itoa(termID)))
		if err != nil || parentID == termID {
			parentID = term.ParentID
		}
		if err := c.taxonomyService.UpdateTermSortOrder(ctx, termID, i, parentID); err != nil {
			c.LogFailure(err, "error updating taxonomy term sort order", "termID", termID)
			http.Redirect(w, r, redirectBase+"?error=display-update", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, redirectBase+"?success=1", http.StatusSeeOther)
}

// termChildLabels returns the names of the terms in vid whose parent is
// termID - used by TaxonomyTermDeleteConfirm to explain why
// ErrTaxonomyTermHasChildren blocks a delete.
func (c *Controller) termChildLabels(ctx context.Context, vid, termID int) ([]string, error) {
	terms, err := c.taxonomyService.TermsByVocabularyID(ctx, vid)
	if err != nil {
		return nil, err
	}
	var labels []string
	for _, t := range terms {
		if t.ParentID == termID {
			labels = append(labels, t.Name)
		}
	}
	return labels, nil
}

// TaxonomyTermDeleteConfirm renders GET
// /admin/config/taxonomy/{vid}/terms/{id}/delete - a confirmation page
// showing how many nodes use the term and which child terms it has, the
// two conditions DeleteTerm refuses on (ErrTaxonomyTermInUse /
// ErrTaxonomyTermHasChildren).
func (c *Controller) TaxonomyTermDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vid, ok := c.pathVocabularyID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	vocabulary, err := c.taxonomyService.VocabularyByID(ctx, vid)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the vocabulary", "vocabularyID", vid)
		return
	}
	term, err := c.taxonomyService.TermByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the term", "id", id)
		return
	}
	if term.Vocabulary.ID != vid {
		c.RenderNotFound(w, r)
		return
	}
	nodeCount, err := c.taxonomyService.CountNodesByTermID(ctx, id)
	if err != nil {
		c.LogFailure(err, "error counting nodes for term delete page", "id", id)
	}
	childLabels, err := c.termChildLabels(ctx, vid, id)
	if err != nil {
		c.LogFailure(err, "error counting child terms for term delete page", "id", id)
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Delete term \"@name\"", "@name", term.Name)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Delete term @name - Admin panel", "@name", term.Name)}
	data["vocabulary"] = vocabulary
	data["term"] = term
	data["nodeCount"] = nodeCount
	data["childLabels"] = childLabels
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/taxonomy/term_delete.html", data)
}

// TaxonomyTermDelete handles POST /admin/config/taxonomy/{vid}/terms/{id}/delete.
// Errors redirect back to the term's delete-confirmation page so the
// reason is shown next to the term.
func (c *Controller) TaxonomyTermDelete(w http.ResponseWriter, r *http.Request) {
	vid, ok := c.pathVocabularyID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	redirectBase := "/admin/config/taxonomy/" + strconv.Itoa(vid) + "/terms"
	if r.Method != http.MethodPost {
		http.Redirect(w, r, redirectBase, http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, redirectBase+"?error=invalid", http.StatusSeeOther)
		return
	}
	deleteConfirmRedirect := redirectBase + "/" + strconv.Itoa(id) + "/delete"

	if err := c.taxonomyService.DeleteTerm(ctx, id); err != nil {
		switch {
		case errors.Is(err, service.ErrTaxonomyTermInUse):
			http.Redirect(w, r, deleteConfirmRedirect+"?error=in-use", http.StatusSeeOther)
		case errors.Is(err, service.ErrTaxonomyTermHasChildren):
			http.Redirect(w, r, deleteConfirmRedirect+"?error=has-children", http.StatusSeeOther)
		default:
			c.LogFailure(err, "error deleting taxonomy term", "id", id)
			http.Redirect(w, r, deleteConfirmRedirect+"?error=delete", http.StatusSeeOther)
		}
		return
	}
	http.Redirect(w, r, redirectBase+"?deleted=1", http.StatusSeeOther)
}
