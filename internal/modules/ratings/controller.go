package ratings

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/modules/ratings/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/randtoken"
)

// guestSessionKey - the session value holding a guest voter's token (the
// rating_votes row keeps its hash).
const guestSessionKey = "ratings_voter"

// widgetTemplates - the file of the module's widget and summary
// templates.
const widgetTemplates = "partials/ratings_hooks.html"

// Controller - the module's settings page and the rating widget of a
// content page: its fragment and the votes.
type Controller struct {
	*modulekit.BaseController
	ratings *Service
	nodes   nodeLoader
	types   typeLabels
}

// nodeLoader - a node by its ID (modulekit.NodesService).
type nodeLoader interface {
	ByID(ctx context.Context, id int) (*modulekit.Node, error)
}

// typeLabels - every content type's label by machine name, the modules'
// too (modulekit.ContentTypeService).
type typeLabels interface {
	NodeTypeLabels(ctx context.Context) map[string]string
}

// NewController creates the controller.
func NewController(deps modulekit.BaseDeps, ratings *Service, nodes nodeLoader, types typeLabels) *Controller {
	base := modulekit.NewBaseController(deps)
	base.SetLogger(deps.Logger.With("component", "ratings.controller"))
	return &Controller{BaseController: base, ratings: ratings, nodes: nodes, types: types}
}

// settingsType - a content type on the settings page.
type settingsType struct {
	MachineName string
	Label       string
	Rated       bool
}

// SettingsPage handles GET/POST /admin/config/ratings: the scale, whether
// a vote can change and be taken back, the rated content types.
func (c *Controller) SettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	labels := c.types.NodeTypeLabels(ctx)
	all := make([]string, 0, len(labels))
	for _, t := range modulekit.NodeTypesByLabel(labels) {
		all = append(all, string(t))
	}

	if r.Method == http.MethodPost {
		scale, _ := strconv.Atoi(r.FormValue("scale"))
		if !ValidScale(scale) {
			http.Redirect(w, r, manifest.SettingsPath+"?error=scale", http.StatusSeeOther)
			return
		}
		err := c.ratings.SaveSettings(ctx, Settings{Scale: scale, AllowGuests: r.FormValue("allow_guests") == "1",
			AllowChange: r.FormValue("allow_change") == "1", AllowWithdraw: r.FormValue("allow_withdraw") == "1"})
		if err == nil {
			err = c.ratings.SetRatedTypes(ctx, all, r.Form["types"])
		}
		if err != nil {
			c.LogFailure(err, "saving the ratings' settings")
			http.Redirect(w, r, manifest.SettingsPath+"?error=save", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, manifest.SettingsPath+"?saved=1", http.StatusSeeOther)
		return
	}

	settings, err := c.ratings.Settings(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the ratings' settings")
		return
	}
	rated, err := c.ratings.RatedTypes(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "listing the rated content types")
		return
	}
	types := make([]settingsType, len(all))
	for i, t := range all {
		types[i] = settingsType{MachineName: t, Label: labels[t], Rated: slices.Contains(rated, t)}
	}
	scales := make([]int, 0, MaxScale-MinScale+1)
	for n := MinScale; n <= MaxScale; n++ {
		scales = append(scales, n)
	}
	title := i18n.T(ctx, "Ratings")
	data := c.AdminPageData(r)
	data["title"] = title
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["settings"] = settings
	data["scales"] = scales
	data["types"] = types
	data["permission"] = manifest.PermRate
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error")
	c.RenderOrFail(w, r, "admin/ratings_settings.html", data)
}

// visitor - the request's user (nil - a guest) and voter (a guest's from
// their session's token; create - make one when they have none).
func (c *Controller) visitor(w http.ResponseWriter, r *http.Request, create bool) (*modulekit.User, Voter, error) {
	user, err := c.GetCurrentUser(r)
	if err != nil {
		return nil, Voter{}, err
	}
	if user != nil {
		return user, Voter{UserID: user.ID}, nil
	}
	session, _ := c.Store().Get(r, csrf.SessionName)
	token, _ := session.Values[guestSessionKey].(string)
	if token == "" && create {
		if token, err = randtoken.Generate(32); err != nil {
			return nil, Voter{}, err
		}
		session.Values[guestSessionKey] = token
		if err := session.Save(r, w); err != nil {
			return nil, Voter{}, err
		}
	}
	if token == "" {
		return nil, Voter{}, nil
	}
	sum := sha256.Sum256([]byte(token))
	return nil, Voter{Guest: hex.EncodeToString(sum[:])}, nil
}

// node - the request's node ({id}), when the visitor may see it.
func (c *Controller) node(r *http.Request, user *modulekit.User) (*modulekit.Node, bool, error) {
	id, ok := c.PathID(r)
	if !ok {
		return nil, false, nil
	}
	node, err := c.nodes.ByID(r.Context(), id)
	if err != nil {
		return nil, false, err
	}
	if !modulekit.NodeTypeEnabled(string(node.Type)) || !c.AuthService().CanViewNode(r.Context(), node, user) {
		return nil, false, nil
	}
	return node, true, nil
}

// widget renders node's widget for the visitor into a buffer.
func (c *Controller) widget(ctx context.Context, node *modulekit.Node, user *modulekit.User, v Voter) ([]byte, bool, error) {
	allowed, err := c.ratings.MayRate(ctx, user == nil, c.AuthService().HasPermission(ctx, user, manifest.PermRate))
	if err != nil {
		return nil, false, err
	}
	wd, ok, err := c.ratings.WidgetFor(ctx, *node, v, allowed, user == nil && !allowed)
	if err != nil || !ok {
		return nil, ok, err
	}
	wd.NodeURL = node.URL()
	var buf bytes.Buffer
	if err := c.TemplateService().RenderNamed(&buf, "ratings_widget", wd, widgetTemplates); err != nil {
		return nil, false, err
	}
	return buf.Bytes(), true, nil
}

// Widget handles GET /node/{id}/rating: the widget of the node's page
// with the visitor's own vote, which the node_rating block loads (the
// page may be cached).
func (c *Controller) Widget(w http.ResponseWriter, r *http.Request) {
	user, voter, err := c.visitor(w, r, false)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "ratings: the visitor")
		return
	}
	node, ok, err := c.node(r, user)
	if err != nil || !ok {
		c.RenderLoadFailure(w, r, errOrNotFound(err), "ratings: the node of a widget")
		return
	}
	html, ok, err := c.widget(r.Context(), node, user, voter)
	if err != nil || !ok {
		c.RenderLoadFailure(w, r, errOrNotFound(err), "ratings: the widget")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(html)
}

// errOrNotFound - err, or "not found" without one.
func errOrNotFound(err error) error {
	if err != nil {
		return err
	}
	return modulekit.ErrEntityNotFound
}

// Vote handles POST /node/{id}/rating: the visitor's score ("score") or
// taking theirs back ("withdraw"). A script gets the new widget as JSON
// ({"html"}, or {"error"} with a status); a form is sent back to the
// node's page.
func (c *Controller) Vote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	asJSON := r.Header.Get("X-Requested-With") == "XMLHttpRequest"
	fail := func(status int, msg string) {
		if asJSON {
			httputil.JSONError(w, msg, status)
			return
		}
		http.Error(w, msg, status)
	}
	if err := r.ParseForm(); err != nil {
		fail(http.StatusBadRequest, i18n.T(ctx, "Invalid request."))
		return
	}
	user, voter, err := c.visitor(w, r, true)
	if err != nil {
		c.LogFailure(err, "ratings: the voter")
		fail(http.StatusInternalServerError, i18n.T(ctx, "Error saving."))
		return
	}
	node, ok, err := c.node(r, user)
	if err != nil && !modulekit.IsNotFound(err) {
		c.LogFailure(err, "ratings: the node voted for")
		fail(http.StatusInternalServerError, i18n.T(ctx, "Error saving."))
		return
	}
	if !ok {
		fail(http.StatusNotFound, i18n.T(ctx, "Page not found"))
		return
	}
	allowed, err := c.ratings.MayRate(ctx, user == nil, c.AuthService().HasPermission(ctx, user, manifest.PermRate))
	if err != nil {
		c.LogFailure(err, "ratings: whether the visitor may rate")
		fail(http.StatusInternalServerError, i18n.T(ctx, "Error saving."))
		return
	}
	if !allowed {
		fail(http.StatusForbidden, i18n.T(ctx, "You may not rate content."))
		return
	}
	if r.FormValue("withdraw") == "1" {
		err = c.ratings.Withdraw(ctx, *node, voter)
	} else {
		score, _ := strconv.Atoi(r.FormValue("score"))
		err = c.ratings.Vote(ctx, *node, voter, score)
	}
	switch {
	case errors.Is(err, ErrNotRated):
		fail(http.StatusNotFound, i18n.T(ctx, "Page not found"))
		return
	case errors.Is(err, ErrScore), errors.Is(err, ErrFinal), errors.Is(err, ErrWithdraw):
		fail(http.StatusBadRequest, voteErrorMessage(ctx, err))
		return
	case err != nil:
		c.LogFailure(err, "ratings: saving a vote", "nodeID", node.ID)
		fail(http.StatusInternalServerError, i18n.T(ctx, "Error saving."))
		return
	}
	if !asJSON {
		http.Redirect(w, r, node.URL(), http.StatusSeeOther)
		return
	}
	html, _, err := c.widget(ctx, node, user, voter)
	if err != nil {
		c.LogFailure(err, "ratings: the widget after a vote", "nodeID", node.ID)
		httputil.JSON(w, map[string]any{"success": true}, http.StatusOK)
		return
	}
	httputil.JSON(w, map[string]any{"success": true, "html": string(html)}, http.StatusOK)
}

// voteErrorMessage - what a voter is told of err.
func voteErrorMessage(ctx context.Context, err error) string {
	switch {
	case errors.Is(err, ErrFinal):
		return i18n.T(ctx, "Your vote can't be changed.")
	case errors.Is(err, ErrWithdraw):
		return i18n.T(ctx, "Your vote can't be taken back.")
	default:
		return i18n.T(ctx, "Choose a number of stars.")
	}
}
