package admin

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// renderPublishStatusFragment renders the publish/discard action area:
// the "Published: <date>" text and which buttons to show. Re-rendered after
// ViewPublishChanges and by ViewPublishStatus, which other mutations call
// since any of them can change whether there are unpublished changes.
// "Discard changes" shows only when a snapshot exists and differs.
func (c *Controller) renderPublishStatusFragment(ctx context.Context, w io.Writer, view *entity.ViewDisplayDef) error {
	hasUnpublished, err := c.viewService.HasUnpublishedChanges(ctx, view.ID)
	if err != nil {
		c.LogFailure(err, "error checking unpublished changes for publish status fragment", "viewID", view.ID)
	}
	data := map[string]any{"view": view, "hasUnpublishedChanges": hasUnpublished, service.RequestLocationKey: sitetime.FromContext(ctx), service.RequestLanguageKey: i18n.FromContext(ctx)}
	return c.templateService.RenderNamed(w, "view_publish_status", data, viewFragmentFiles...)
}

// ViewSettingsOverride and ViewSettingsRevert handle POST
// /admin/config/views/{id}/settings/override and .../revert - the
// "Page settings" card's override/revert banner (see
// views.DefaultService.OverrideSettings/RevertSettings).
func (c *Controller) ViewSettingsOverride(w http.ResponseWriter, r *http.Request) {
	id, redirectBase, ok := c.viewPost(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	updated, err := c.viewService.OverrideSettings(ctx, id)
	if err != nil {
		c.LogFailure(err, "error overriding view settings", "id", id)
		c.jsonOrRedirectError(w, r, redirectBase, "override")
		return
	}
	c.jsonOrRedirectSuccess(w, r, redirectBase, func(w io.Writer) error {
		return c.renderSettingsSummaryFragment(ctx, w, updated)
	})
}

// ViewSettingsRevert handles POST /admin/config/views/{id}/settings/revert:
// the display's settings follow its View's Default again.
func (c *Controller) ViewSettingsRevert(w http.ResponseWriter, r *http.Request) {
	id, redirectBase, ok := c.viewPost(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	updated, err := c.viewService.RevertSettings(ctx, id)
	if err != nil {
		c.LogFailure(err, "error reverting view settings", "id", id)
		c.jsonOrRedirectError(w, r, redirectBase, "revert")
		return
	}
	c.jsonOrRedirectSuccess(w, r, redirectBase, func(w io.Writer) error {
		return c.renderSettingsSummaryFragment(ctx, w, updated)
	})
}

// ViewFiltersOverride and ViewFiltersRevert handle POST
// /admin/config/views/{id}/filters/override and .../revert.
func (c *Controller) ViewFiltersOverride(w http.ResponseWriter, r *http.Request) {
	rows := c.filterRows()
	rows.fork(c, w, r, rows.override, "override")
}

// ViewFiltersRevert handles POST /admin/config/views/{id}/filters/revert:
// the display's filters follow its View's Default again.
func (c *Controller) ViewFiltersRevert(w http.ResponseWriter, r *http.Request) {
	rows := c.filterRows()
	rows.fork(c, w, r, rows.revert, "revert")
}

// ViewSortOverride and ViewSortRevert handle POST
// /admin/config/views/{id}/sorts/override and .../revert.
func (c *Controller) ViewSortOverride(w http.ResponseWriter, r *http.Request) {
	rows := c.sortRows()
	rows.fork(c, w, r, rows.override, "override")
}

// ViewSortRevert handles POST /admin/config/views/{id}/sorts/revert: the
// display's sort criteria follow its View's Default again.
func (c *Controller) ViewSortRevert(w http.ResponseWriter, r *http.Request) {
	rows := c.sortRows()
	rows.fork(c, w, r, rows.revert, "revert")
}

// ViewFieldsOverride and ViewFieldsRevert handle POST
// /admin/config/views/{id}/fields/override and .../revert.
func (c *Controller) ViewFieldsOverride(w http.ResponseWriter, r *http.Request) {
	rows := c.fieldRows()
	rows.fork(c, w, r, rows.override, "override")
}

// ViewFieldsRevert handles POST /admin/config/views/{id}/fields/revert:
// the display's fields follow its View's Default again.
func (c *Controller) ViewFieldsRevert(w http.ResponseWriter, r *http.Request) {
	rows := c.fieldRows()
	rows.fork(c, w, r, rows.revert, "revert")
}

// ViewPublishChanges handles POST /admin/config/views/{id}/publish -
// "Publish changes".
func (c *Controller) ViewPublishChanges(w http.ResponseWriter, r *http.Request) {
	id, redirectBase, ok := c.viewPost(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	updated, err := c.viewService.PublishDisplay(ctx, id)
	if err != nil {
		c.LogFailure(err, "error publishing view display", "id", id)
		c.jsonOrRedirectError(w, r, redirectBase, "publish")
		return
	}
	c.jsonOrRedirectSuccess(w, r, redirectBase, func(w io.Writer) error {
		return c.renderPublishStatusFragment(ctx, w, updated)
	})
}

// ViewDiscardChanges handles POST /admin/config/views/{id}/discard -
// "Discard changes". Always a plain redirect (admin_views.js doesn't
// intercept it): discarding rewrites all four cards, so a full reload is
// simpler than a multi-fragment JSON response.
func (c *Controller) ViewDiscardChanges(w http.ResponseWriter, r *http.Request) {
	id, redirectBase, ok := c.viewPost(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if _, err := c.viewService.DiscardDisplayChanges(ctx, id); err != nil {
		c.LogFailure(err, "error discarding view display changes", "id", id)
		code := "discard"
		if errors.Is(err, views.ErrNoPublishedSnapshot) {
			code = "no-snapshot"
		}
		http.Redirect(w, r, redirectBase+"?error="+code, http.StatusSeeOther)
		return
	}
	c.refreshViewBlocks()
	http.Redirect(w, r, redirectBase+"?success=1", http.StatusSeeOther)
}

// ViewPublishStatus handles GET /admin/config/views/{id}/publish-status -
// a read-only refresh of the publish/discard action bar. admin_views.js
// calls it after every save, reorder or delete, since any of them can
// change whether there are unpublished changes.
func (c *Controller) ViewPublishStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		httputil.JSONError(w, "invalid", http.StatusBadRequest)
		return
	}
	view, err := c.viewService.ViewByID(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading the View display", "viewID", id)
		httputil.JSONError(w, "not-found", http.StatusNotFound)
		return
	}
	var buf bytes.Buffer
	if err := c.renderPublishStatusFragment(ctx, &buf, view); err != nil {
		c.LogFailure(err, "error rendering publish status fragment", "id", id)
		httputil.JSONError(w, "render", http.StatusInternalServerError)
		return
	}
	httputil.JSON(w, map[string]any{"html": buf.String()}, http.StatusOK)
}
