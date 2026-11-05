package languages

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TranslationsController - a content item's Translations tab and making
// a translation.
type TranslationsController struct {
	*modulekit.BaseController
	db           *sql.DB
	nodes        modulekit.NodesRepository
	types        modulekit.ContentTypeService
	translations *Translations
}

// NewTranslationsController creates a TranslationsController.
func NewTranslationsController(base *modulekit.BaseController, db *sql.DB, nodes modulekit.NodesRepository, types modulekit.ContentTypeService, translations *Translations) *TranslationsController {
	base.SetLogger(base.Logger().With("component", "languages.translations"))
	return &TranslationsController{BaseController: base, db: db, nodes: nodes, types: types, translations: translations}
}

// translationRow - a site language on the Translations tab: its version
// of the item (none - not translated).
type translationRow struct {
	Code    string
	Name    string
	Version *Version
	// Current - the version the tab was opened from.
	Current bool
}

// access - the request's item when its type is translated, the site has
// other languages and the user may edit it; false - answered.
func (c *TranslationsController) access(w http.ResponseWriter, r *http.Request) (*modulekit.Node, *modulekit.User, bool) {
	ctx := r.Context()
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return nil, nil, false
	}
	node, err := c.nodes.ByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the node to translate", "nodeID", id)
		return nil, nil, false
	}
	user, _ := c.GetCurrentUser(r)
	if !c.AuthService().CanEditNode(ctx, node, user) {
		c.RenderForbidden(w, r)
		return nil, nil, false
	}
	translatable, err := c.translations.Translatable(ctx, node.Type.ToString())
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the translatable types")
		return nil, nil, false
	}
	if _, others := modulekit.SiteLanguages(); !translatable || len(others) == 0 {
		c.RenderNotFound(w, r)
		return nil, nil, false
	}
	return node, user, true
}

// Page renders GET /node/{id}/translations: each site language - the
// default first - with its version of the item: the original, the
// translations (outdated marked), Translate where there's none.
func (c *TranslationsController) Page(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, user, ok := c.access(w, r)
	if !ok {
		return
	}
	versions, err := c.translations.Versions(ctx, node.ID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the translations", "nodeID", node.ID)
		return
	}
	if len(versions) == 0 {
		versions = []Version{{Member: Member{Nid: node.ID, Original: true}, Node: *node}}
	}
	byLang := ByLanguage(versions)
	def, others := modulekit.SiteLanguages()
	codes := []string{def}
	for _, l := range others {
		codes = append(codes, l.Code)
	}
	rows := make([]translationRow, 0, len(codes))
	for _, code := range codes {
		row := translationRow{Code: code, Name: i18n.NativeName(code)}
		if v, ok := byLang[code]; ok {
			row.Version, row.Current = &v, v.Node.ID == node.ID
		}
		rows = append(rows, row)
	}
	canEdit, canDelete := c.EditDeletePermissions(ctx, node, user)
	canCreate := c.AuthService().CanCreateNode(ctx, node.Type, user)
	// The item's actions as on its page: the history where its type keeps
	// one, a copy.
	revisions := false
	if def, err := c.types.ContentTypeByMachineName(ctx, node.Type.ToString()); err == nil && def != nil {
		revisions = def.Revisions
	}
	data := c.GetCommonData(r)
	data["canViewRevisions"] = canEdit && revisions && c.MayViewRevisions(ctx, node, user)
	data["canClone"] = canEdit && canCreate
	data["item"] = struct{ Node modulekit.Node }{*node}
	// The modules' parts of the item's actions (this tab's link too).
	data["moduleNode"] = modulekit.ModuleNodeData(ctx, *node)
	data["canEdit"], data["canDelete"] = canEdit, canDelete
	data["translationsPage"] = true
	data["rows"] = rows
	data["canTranslate"] = canCreate
	data["error"] = r.URL.Query().Get("error")
	title := i18n.T(ctx, "Translations: @title", "@title", node.Title)
	data["title"] = title
	data["pageSEO"] = modulekit.PageSEO{Title: title, NoIndex: true}
	c.SetNodeSubpageTrail(ctx, data, *node, i18n.T(ctx, "Translations"))
	c.RenderOrFail(w, r, "languages/translations.html", data)
}

// Translate handles POST /node/{id}/translations/{lang}: the item's
// translation into lang - a copy of it in lang, unpublished, in its set -
// when its set has none yet; its form opens.
func (c *TranslationsController) Translate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, user, ok := c.access(w, r)
	if !ok {
		return
	}
	back := fmt.Sprintf("/node/%d/translations", node.ID)
	lang := r.PathValue("lang")
	if !c.AuthService().CanCreateNode(ctx, node.Type, user) {
		c.RenderForbidden(w, r)
		return
	}
	if !siteLanguage(lang) {
		http.Redirect(w, r, back+"?error=language", http.StatusSeeOther)
		return
	}
	versions, err := c.translations.Versions(ctx, node.ID)
	if err != nil {
		c.LogFailure(err, "loading the translations", "nodeID", node.ID)
		http.Redirect(w, r, back+"?error=save", http.StatusSeeOther)
		return
	}
	if _, ok := ByLanguage(versions)[lang]; ok || (len(versions) == 0 && node.Langcode == lang) {
		http.Redirect(w, r, back+"?error=exists", http.StatusSeeOther)
		return
	}
	id, err := c.translations.Translate(ctx, c.db, *node, lang, user)
	if err != nil {
		c.LogFailure(err, "translating a node", "nodeID", node.ID, "lang", lang)
		http.Redirect(w, r, back+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/node/%d/edit", id), http.StatusSeeOther)
}

// siteLanguage - code is one of the site's languages.
func siteLanguage(code string) bool {
	def, others := modulekit.SiteLanguages()
	if code == def {
		return true
	}
	for _, l := range others {
		if l.Code == code {
			return true
		}
	}
	return false
}

// formSaved - a content item's form saved it: its set's translations are
// outdated (it is their original) or it is up to date (a translation).
func (t *Translations) formSaved(ctx context.Context, node modulekit.Node, isNew bool) error {
	if isNew {
		return nil
	}
	return t.repo.Saved(ctx, node.ID)
}
