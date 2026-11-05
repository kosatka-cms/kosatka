package polls

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/polls/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// PollController - the pages of the poll content type: the poll's page
// and block, its create and edit forms, voting and the votes' log.
type PollController struct {
	*modulekit.BaseController
	pollService  PollService
	nodesService modulekit.NodesService
	nodeRepo     modulekit.NodesRepository
	nodeDeleter  modulekit.NodeDeleter
}

// PollDeps - what NewPollController needs besides the BaseDeps.
type PollDeps struct {
	PollService  PollService
	NodesService modulekit.NodesService
	NodeDeleter  modulekit.NodeDeleter
}

// NewPollController - the polls controller on the base dependencies and d
// (the polls and nodes services, the node deleter).
func NewPollController(deps modulekit.BaseDeps, d PollDeps) *PollController {
	base := modulekit.NewBaseController(deps)
	base.SetLogger(deps.Logger.With("component", "poll.handler"))
	return &PollController{
		BaseController: base,
		pollService:    d.PollService,
		nodesService:   d.NodesService,
		nodeRepo:       deps.Nodes,
		nodeDeleter:    d.NodeDeleter,
	}
}

// Detail renders the poll's page at its address (GET /<slug>, the node
// page of a poll): its question, results and vote form, the edit and
// delete links for those who may.
func (c *PollController) Detail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/")

	p, err := c.pollService.BySlug(ctx, path)
	if err != nil {
		if modulekit.IsNotFound(err) {
			c.RenderNotFound(w, r)
			return
		}
		if errors.Is(err, context.Canceled) {
			return
		}
		c.LogFailure(err, "poll by slug", "slug", path)
		httputil.InternalServerError(w)
		return
	}

	user, _ := c.GetCurrentUser(r)
	if !c.RequireVisibleOrModerator(w, r, &p.Node, user) {
		return
	}

	currentUser, _ := modulekit.RequestUser(r)
	canEdit, canDelete := c.EditDeletePermissions(ctx, &p.Node, currentUser)

	userID := 0
	loggedIn := false
	if uid, ok := c.AuthService().GetUserID(r); ok && uid > 0 {
		userID = uid
		loggedIn = true
	}

	view, err := c.pollService.BuildView(ctx, p, userID, loggedIn)
	if err != nil {
		c.LogFailure(err, "poll build view")
		httputil.InternalServerError(w)
		return
	}

	data := c.GetCommonData(r)
	data["item"] = p
	data["pollView"] = view
	// The node's own meta title and description (the editor's SEO
	// fields), else the question and the description.
	seo := modulekit.PageSEO{Title: p.Node.Title, Description: modulekit.TruncateMeta(p.Description, 160)}
	if p.Node.PageSEO.Title != "" {
		seo.Title = p.Node.PageSEO.Title
	}
	if p.Node.PageSEO.Description != "" {
		seo.Description = p.Node.PageSEO.Description
	}
	data["pageSEO"] = seo
	data["canEdit"] = canEdit
	data["canDelete"] = canDelete
	canViewPollVotes := c.AuthService().HasPermission(ctx, currentUser, manifest.PermViewVotes)
	data["canViewPollVotes"] = canViewPollVotes
	data["moduleNode"] = modulekit.ModuleNodeData(ctx, p.Node)
	data["CSRFToken"] = csrf.EnsureToken(c.Store(), w, r)

	c.SetViewTrackNID(data, p.Node)
	c.SetNodeRefTrail(ctx, data, p.Node)

	c.RenderOrFail(w, r, "poll/detail.html", data)
}

// Embed handles GET /node/{id}/poll/embed: the poll alone (poll_block of
// poll/detail.html - its question linking its page, the results and the
// vote form) for the "Poll" block, loaded by polls.js. Per visitor (their
// vote, the form's token), so a cached page still shows each one theirs.
func (c *PollController) Embed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid, ok := c.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, err := c.pollService.ByNodeID(ctx, nid)
	if err != nil {
		if !modulekit.IsNotFound(err) {
			c.LogFailure(err, "poll for the block", "nodeID", nid)
		}
		http.NotFound(w, r)
		return
	}
	user, _ := c.GetCurrentUser(r)
	if !c.AuthService().CanViewNode(ctx, &p.Node, user) {
		http.NotFound(w, r)
		return
	}
	userID := 0
	if user != nil {
		userID = user.ID
	}
	view, err := c.pollService.BuildView(ctx, p, userID, userID > 0)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "building the poll", "nodeID", nid)
		return
	}
	data := map[string]any{
		modulekit.RequestLocationKey: sitetime.FromContext(ctx),
		modulekit.RequestLanguageKey: i18n.FromContext(ctx),
		modulekit.RequestThemeKey:    modulekit.RequestTheme(ctx),
		"pollView":                   view,
		"currentUser":                user,
		"CSRFToken":                  csrf.EnsureToken(c.Store(), w, r),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := c.TemplateService().RenderNamed(w, "poll_block", data, "poll/detail.html"); err != nil {
		c.LogFailure(err, "render poll embed", "nodeID", nid)
	}
}

// VotesAudit lists individual votes (manifest.PermViewVotes). GET /node/{id}/poll/votes
func (c *PollController) VotesAudit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := c.GetCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if !c.AuthService().HasPermission(ctx, user, manifest.PermViewVotes) {
		c.RenderForbidden(w, r)
		return
	}
	nid, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	p, rows, err := c.pollService.VoteAudit(ctx, nid)
	if err != nil {
		if modulekit.IsNotFound(err) {
			c.RenderNotFound(w, r)
			return
		}
		c.LogFailure(err, "poll vote audit", "nid", nid)
		httputil.InternalServerError(w)
		return
	}

	// No node access middleware on this URL, so the user comes from the session.
	canEdit := c.AuthService().CanEditNode(ctx, &p.Node, user)
	canDelete := c.AuthService().CanDeleteNode(ctx, &p.Node, user)

	data := c.GetCommonData(r)
	data["poll"] = p
	data["editNode"] = data["poll"]
	data["item"] = p
	data["voteRows"] = rows
	data["canEdit"] = canEdit
	data["canDelete"] = canDelete
	data["canViewPollVotes"] = true
	data["pollVotesPage"] = true
	data["moduleNode"] = modulekit.ModuleNodeData(ctx, p.Node)
	data["title"] = i18n.T(ctx, "Votes - @title", "@title", p.Node.Title)
	c.SetNodeSubpageTrail(ctx, data, p.Node, i18n.T(ctx, "Votes"))
	data["pageSEO"] = modulekit.PageSEO{
		Title:       i18n.T(ctx, "Votes - @title", "@title", p.Node.Title),
		Description: modulekit.TruncateMeta(p.Node.Title, 160),
	}

	c.RenderOrFail(w, r, "poll/votes.html", data)
}

// CreateForm renders the form of a new poll (GET /create/{type} of the
// poll type).
func (c *PollController) CreateForm(w http.ResponseWriter, r *http.Request) {
	pf := &PollForm{
		Status:       1,
		OnModeration: 0,
		MaxAnswers:   1,
		AllowRevote:  true,
	}
	data := c.GetCommonData(r)
	maps.Copy(data, c.pollPageData(pf, nil))
	data["poll"] = &Poll{}
	data["editNode"] = data["poll"]
	data["isCreateMode"] = true
	data["title"] = i18n.T(r.Context(), "Create poll")
	c.SetCreateFormTrail(data, i18n.T(r.Context(), "Create poll"))
	c.ApplyEditorPageSEOFromTitle(data, r)
	c.RenderOrFail(w, r, "node/edit_poll.html", data)
}

// Create saves a new poll from its form (POST /create/{type} of the poll
// type): an invalid form comes back with its errors; then the poll's page.
func (c *PollController) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := c.GetCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	pf := &PollForm{}
	_ = pf.ParsePollForm(r)
	if errs := pf.Validate(r.Context()); len(errs) > 0 {
		data := c.GetCommonData(r)
		maps.Copy(data, c.pollPageData(pf, nil))
		data["poll"] = &Poll{}
		data["editNode"] = data["poll"]
		data["errors"] = errs
		data["isCreateMode"] = true
		data["title"] = i18n.T(ctx, "Create poll")
		c.SetCreateFormTrail(data, i18n.T(ctx, "Create poll"))
		c.ApplyEditorPageSEOFromTitle(data, r)
		c.RenderOrFail(w, r, "node/edit_poll.html", data)
		return
	}
	entPoll, err := c.pollEntityFromForm(r.Context(), pf, user, nil)
	if err != nil {
		c.RenderBadRequest(w, r, i18n.ErrorText(r.Context(), err))
		return
	}
	admin := c.applyPollNodeSettings(r, user, pf, entPoll, nil)
	if entPoll.Node.CreatedBy.ID == 0 {
		entPoll.Node.CreatedBy = *user
	}

	created, err := c.pollService.Create(ctx, entPoll)
	if err != nil {
		c.LogFailure(err, "create poll")
		httputil.InternalServerError(w)
		return
	}
	if !admin {
		http.Redirect(w, r, created.Node.URL(), http.StatusFound)
		return
	}
	if seoErr := c.nodesService.UpsertPageSEO(ctx, created.Node.ID, pf.MetaTitle, pf.MetaDescription); seoErr != nil {
		c.LogFailure(seoErr, "error upserting page SEO", "nodeID", created.Node.ID)
		httputil.InternalServerError(w)
		return
	}
	http.Redirect(w, r, created.Node.URL(), http.StatusFound)
}

// Edit renders the poll's edit form (GET /node/{id}/edit).
func (c *PollController) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, ok := modulekit.RequestNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	p, err := c.pollService.ByNodeID(ctx, node.ID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the poll", "nodeID", node.ID)
		return
	}
	pf := pollFormFromEntity(r.Context(), p)
	data := c.GetCommonData(r)
	maps.Copy(data, c.pollPageData(pf, p))
	data["poll"] = p
	data["editNode"] = data["poll"]
	data["title"] = i18n.T(ctx, "Edit poll")
	c.SetNodeSubpageTrail(ctx, data, p.Node, i18n.T(ctx, "Editing"))
	c.ApplyEditorPageSEOFromTitle(data, r)
	c.RenderOrFail(w, r, "node/edit_poll.html", data)
}

// Save saves the poll from its edit form (POST /node/{id}/edit): an
// invalid form comes back with its errors, its publishing changes only for
// who administers content; then the poll's page.
func (c *PollController) Save(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, ok := modulekit.RequestNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	user, ok := modulekit.RequestUser(r)
	if !ok || user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	existing, err := c.pollService.ByNodeID(ctx, node.ID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the poll", "nodeID", node.ID)
		return
	}
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	pf := &PollForm{}
	_ = pf.ParsePollForm(r)
	pf.NodeID = node.ID
	if !c.AuthService().HasPermission(r.Context(), user, modulekit.PermAdministerContent) {
		pf.Status = existing.Node.Status
		pf.OnModeration = existing.Node.OnModeration
	}
	if errs := pf.Validate(r.Context()); len(errs) > 0 {
		data := c.GetCommonData(r)
		maps.Copy(data, c.pollPageData(pf, existing))
		data["poll"] = existing
		data["editNode"] = data["poll"]
		data["errors"] = errs
		data["title"] = i18n.T(ctx, "Edit poll")
		c.SetNodeSubpageTrail(ctx, data, existing.Node, i18n.T(ctx, "Editing"))
		c.ApplyEditorPageSEOFromTitle(data, r)
		c.RenderOrFail(w, r, "node/edit_poll.html", data)
		return
	}
	entPoll, err := c.pollEntityFromForm(r.Context(), pf, user, existing)
	if err != nil {
		c.RenderBadRequest(w, r, i18n.ErrorText(r.Context(), err))
		return
	}
	entPoll.Node.ID = existing.Node.ID
	entPoll.ID = existing.ID
	admin := c.applyPollNodeSettings(r, user, pf, entPoll, existing)
	if _, err := c.pollService.Update(ctx, entPoll); err != nil {
		c.LogFailure(err, "update poll")
		httputil.InternalServerError(w)
		return
	}
	if !admin {
		http.Redirect(w, r, entPoll.Node.URL(), http.StatusSeeOther)
		return
	}
	if seoErr := c.nodesService.UpsertPageSEO(ctx, node.ID, pf.MetaTitle, pf.MetaDescription); seoErr != nil {
		c.LogFailure(seoErr, "error upserting page SEO", "nodeID", node.ID)
		httputil.InternalServerError(w)
		return
	}
	http.Redirect(w, r, entPoll.Node.URL(), http.StatusSeeOther)
}

// Delete deletes the poll's node (POST /node/{id}/delete); then the front
// page.
func (c *PollController) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, ok := modulekit.RequestNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	if err := c.nodeDeleter.Delete(ctx, node.ID); err != nil {
		c.LogFailure(err, "delete poll")
		httputil.InternalServerError(w)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// VoteSubmit casts the signed-in user's vote from the poll's form (POST
// /node/{id}/poll/vote): a refused vote is a bad request; then the poll's
// page.
func (c *PollController) VoteSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	// AuthMiddleware only sets user_id in context; GetUser() expects "user" from node access middleware.
	userID, loggedIn := c.AuthService().GetUserID(r)
	if !loggedIn || userID <= 0 {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	nid, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	var optionIDs []int
	for _, s := range r.Form["options"] {
		id, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || id <= 0 {
			continue
		}
		optionIDs = append(optionIDs, id)
	}
	if len(optionIDs) == 0 {
		v := r.FormValue("options")
		if v != "" {
			if id, err := strconv.Atoi(v); err == nil && id > 0 {
				optionIDs = append(optionIDs, id)
			}
		}
	}
	err := c.pollService.Vote(ctx, nid, userID, optionIDs)
	if err != nil {
		if msg := voteErrorMessage(ctx, err); msg != "" {
			c.RenderBadRequest(w, r, msg)
			return
		}
		c.LogFailure(err, "poll vote")
		httputil.InternalServerError(w)
		return
	}
	node, err := c.nodeRepo.ByID(ctx, nid)
	if err != nil || node == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r, node.URL(), http.StatusSeeOther)
}

// pollVoteAPIBody is the JSON body for POST /api/node/{id}/poll/vote.
type pollVoteAPIBody struct {
	Options []int `json:"options"`
}

// pollVoteAPIResponse is returned after a successful vote (for AJAX UI update).
type pollVoteAPIResponse struct {
	TotalVotes int                 `json:"total_votes"`
	ShowForm   bool                `json:"show_form"`
	Options    []pollVoteAPIOption `json:"options"`
}

type pollVoteAPIOption struct {
	ID       int     `json:"id"`
	Label    string  `json:"label"`
	Count    int     `json:"count"`
	Percent  float64 `json:"percent"`
	Selected bool    `json:"selected"`
}

// VoteAPI accepts JSON and returns updated counts; session cookie required (AuthJSONMiddleware).
func (c *PollController) VoteAPI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodPost {
		httputil.JSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body pollVoteAPIBody
	if err := httputil.DecodeJSON(r, &body); err != nil {
		httputil.JSONError(w, "invalid json", http.StatusBadRequest)
		return
	}
	userID, ok := modulekit.ContextUserID(r.Context())
	if !ok || userID <= 0 {
		httputil.JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	nid, ok := c.PathID(r)
	if !ok {
		httputil.JSONError(w, "invalid node id", http.StatusBadRequest)
		return
	}
	err := c.pollService.Vote(ctx, nid, userID, body.Options)
	if err != nil {
		switch {
		case errors.Is(err, ErrPollVoteForbidden):
			httputil.JSONError(w, voteErrorMessage(ctx, err), http.StatusForbidden)
		case errors.Is(err, ErrPollVoteInvalid), errors.Is(err, ErrPollRevoteNotAllowed):
			httputil.JSONError(w, voteErrorMessage(ctx, err), http.StatusBadRequest)
		default:
			c.LogFailure(err, "poll vote api")
			httputil.InternalServerError(w)
		}
		return
	}
	p, err := c.pollService.ByNodeID(ctx, nid)
	if err != nil {
		c.LogFailure(err, "poll vote api reload")
		httputil.InternalServerError(w)
		return
	}
	view, err := c.pollService.BuildView(ctx, p, userID, true)
	if err != nil {
		c.LogFailure(err, "poll vote api build view")
		httputil.InternalServerError(w)
		return
	}
	resp := pollVoteAPIResponse{
		TotalVotes: view.TotalVotes,
		ShowForm:   view.ShowForm,
		Options:    make([]pollVoteAPIOption, 0, len(view.OptionCounts)),
	}
	for _, oc := range view.OptionCounts {
		resp.Options = append(resp.Options, pollVoteAPIOption{
			ID:       oc.ID,
			Label:    oc.Label,
			Count:    oc.Count,
			Percent:  oc.Percent,
			Selected: oc.Selected,
		})
	}
	httputil.JSON(w, resp, http.StatusOK)
}

// pollPageData - the poll form's data: an editor form, a service page
// (no sidebar).
func (c *PollController) pollPageData(f *PollForm, p *Poll) map[string]any {
	data := map[string]any{
		"form":        f,
		"servicePage": true,
	}
	if p != nil {
		data["poll"] = p
		data["editNode"] = data["poll"]
	}
	return data
}

func pollFormFromEntity(ctx context.Context, p *Poll) *PollForm {
	l := sitetime.FromContext(ctx)
	f := &PollForm{
		NodeMetaFields: modulekit.NodeMetaFromPageSEO(p.Node.PageSEO),
		NodeID:         p.Node.ID,
		Title:          p.Node.Title,
		Slug:           p.Node.Slug,
		Description:    p.Description,
		MaxAnswers:     p.MaxAnswers,
		ResultsOnly:    p.ResultsOnly,
		AllowRevote:    p.AllowRevote,
		Status:         p.Node.Status,
		OnModeration:   p.Node.OnModeration,
		CommentMode:    string(p.Node.CommentMode),
	}
	if p.AvailableFrom != nil && l != nil {
		f.AvailableFrom = p.AvailableFrom.In(l).Format("2006-01-02T15:04")
	}
	if p.AvailableTo != nil && l != nil {
		f.AvailableTo = p.AvailableTo.In(l).Format("2006-01-02T15:04")
	}
	for _, o := range p.Options {
		f.OptionLabels = append(f.OptionLabels, o.Label)
		f.OptionIDs = append(f.OptionIDs, o.ID)
	}
	return f
}

func (c *PollController) pollEntityFromForm(ctx context.Context, f *PollForm, user *modulekit.User, existing *Poll) (*Poll, error) {
	af, err := ParsePollDatetimeLocal(f.AvailableFrom, sitetime.FromContext(ctx))
	if err != nil {
		return nil, i18n.Errorf("Invalid start date")
	}
	at, err := ParsePollDatetimeLocal(f.AvailableTo, sitetime.FromContext(ctx))
	if err != nil {
		return nil, i18n.Errorf("Invalid end date")
	}
	out := &Poll{
		Description:   f.Description,
		MaxAnswers:    f.MaxAnswers,
		AvailableFrom: af,
		AvailableTo:   at,
		ResultsOnly:   f.ResultsOnly,
		AllowRevote:   f.AllowRevote,
		Node: modulekit.Node{
			Title:        f.Title,
			Slug:         f.Slug,
			Status:       f.Status,
			OnModeration: f.OnModeration,
			CommentMode:  modulekit.CommentMode(f.CommentMode),
		},
	}
	if existing != nil {
		out.Node.CreatedBy = existing.Node.CreatedBy
		out.Node.CreatedAt = existing.Node.CreatedAt
		if !modulekit.IsValidCommentMode(f.CommentMode) {
			out.Node.CommentMode = existing.Node.CommentMode
		}
	} else if user != nil {
		out.Node.CreatedBy = *user
	}
	for i := range f.OptionLabels {
		id := 0
		if i < len(f.OptionIDs) {
			id = f.OptionIDs[i]
		}
		out.Options = append(out.Options, PollOption{
			ID:     id,
			Label:  f.OptionLabels[i],
			Weight: i,
		})
	}
	if existing != nil {
		out.ID = existing.ID
		out.Node.ID = existing.Node.ID
		// The edit form doesn't submit slug; don't overwrite the node's URL with "".
		if strings.TrimSpace(f.Slug) == "" {
			out.Node.Slug = existing.Node.Slug
		}
	}
	return out, nil
}

// applyPollNodeSettings sets p's node settings from the form (existing -
// the poll edited, nil for a new one): who administers content gives the
// publishing, the author, the date, the address and the comments' mode
// (the form shows them to them only); anyone else's are ignored - a new
// poll is published with a generated address, an edited one keeps its
// own. Whether the user administers content (the meta tags are theirs
// too).
func (c *PollController) applyPollNodeSettings(r *http.Request, user *modulekit.User, pf *PollForm, p, existing *Poll) bool {
	if c.AuthService().HasPermission(r.Context(), user, modulekit.PermAdministerContent) {
		p.Node.Status, p.Node.OnModeration = pf.Status, pf.OnModeration
		c.applyPollNodeMetaFromRequest(r, p, existing)
		return true
	}
	if existing == nil {
		p.Node.Status, p.Node.OnModeration = 1, 0
		p.Node.Slug, p.Node.CommentMode = "", ""
		return false
	}
	p.Node.Status, p.Node.OnModeration = existing.Node.Status, existing.Node.OnModeration
	p.Node.Slug, p.Node.CommentMode = existing.Node.Slug, existing.Node.CommentMode
	return false
}

// applyPollNodeMetaFromRequest applies the author and creation date from the node settings accordion, as for other node types.
func (c *PollController) applyPollNodeMetaFromRequest(r *http.Request, entPoll *Poll, existing *Poll) {
	if s := strings.TrimSpace(r.FormValue("created_by")); s != "" {
		if id, err := strconv.Atoi(s); err == nil && id > 0 {
			entPoll.Node.CreatedBy = modulekit.User{ID: id}
		}
	} else if existing != nil {
		entPoll.Node.CreatedBy = existing.Node.CreatedBy
	}
	if s := strings.TrimSpace(r.FormValue("created_at")); s != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", s, sitetime.FromContext(r.Context())); err == nil {
			entPoll.Node.CreatedAt = t
		}
	} else if existing != nil {
		entPoll.Node.CreatedAt = existing.Node.CreatedAt
	}
}

// voteErrorMessage - what a voter is told about a refused vote ("" - not
// the voter's mistake).
func voteErrorMessage(ctx context.Context, err error) string {
	switch {
	case errors.Is(err, ErrPollVoteForbidden):
		return i18n.T(ctx, "You can't vote in this poll.")
	case errors.Is(err, ErrPollVoteInvalid):
		return i18n.T(ctx, "Choose an answer.")
	case errors.Is(err, ErrPollRevoteNotAllowed):
		return i18n.T(ctx, "You have already voted in this poll.")
	}
	return ""
}
