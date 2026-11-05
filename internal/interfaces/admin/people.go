package admin

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// PeoplePerPage - accounts on a page of the people list.
const PeoplePerPage = 25

// PeopleList handles GET /admin/people: the accounts, filtered by status
// and role, sorted by creation or last access, a page at a time, with
// their two-factor state and pending invitations.
func (c *Controller) PeopleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := c.GetPageNumber(r)

	sortKey := strings.TrimSpace(r.URL.Query().Get("sort"))
	if sortKey != entity.AdminPeopleSortLastAccess {
		sortKey = entity.AdminPeopleSortCreated
	}
	orderKey := strings.TrimSpace(r.URL.Query().Get("order"))
	desc := orderKey != "asc"
	opts := entity.AdminPeopleListOpts{SortBy: sortKey, Desc: desc}
	switch strings.TrimSpace(r.URL.Query().Get("status")) {
	case "active":
		st := entity.UserStatusActive
		opts.Status = &st
	case "pending":
		st := entity.UserStatusPending
		opts.Status = &st
	case "blocked":
		st := entity.UserStatusBlocked
		opts.Status = &st
	}

	filterRoleID := 0
	if roleQS := strings.TrimSpace(r.URL.Query().Get("role")); roleQS != "" {
		if id, err := strconv.Atoi(roleQS); err == nil && id > 0 {
			opts.RoleID = &id
			filterRoleID = id
		}
	}

	roles, errRoles := c.roleRepo.List(ctx)
	if errRoles != nil {
		c.LogFailure(errRoles, "error listing roles for admin people")
		roles = &entity.Roles{}
	}

	users, totalCount, effectivePage, err := c.userRepo.ListForAdmin(ctx, page, PeoplePerPage, opts)
	if err != nil {
		c.LogFailure(err, "error listing users for admin")
		httputil.InternalServerError(w)
		return
	}

	pager := pagination.New(effectivePage, totalCount, PeoplePerPage, "/admin/people", r.URL.Query())

	displayOrder := orderKey
	if displayOrder != "asc" && displayOrder != "desc" {
		displayOrder = "desc"
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "People")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["users"] = users
	if users != nil {
		data["twoFactor"] = c.peopleTwoFactor(ctx, *users)
		data["invites"] = c.peopleInvites(ctx, *users)
	}
	data["pagination"] = pager
	data["created"] = r.URL.Query().Get("created") == "1"
	data["invited"] = r.URL.Query().Get("invited")
	data["inviteCancelled"] = r.URL.Query().Get("invite_cancelled") == "1"
	data["approved"] = r.URL.Query().Get("approved")
	data["sort"] = sortKey
	data["order"] = displayOrder
	data["sortURLCreated"] = peopleListSortURL(r, entity.AdminPeopleSortCreated)
	data["sortURLLastAccess"] = peopleListSortURL(r, entity.AdminPeopleSortLastAccess)
	if opts.Status != nil {
		switch *opts.Status {
		case entity.UserStatusActive:
			data["filterStatus"] = "active"
		case entity.UserStatusPending:
			data["filterStatus"] = "pending"
		case entity.UserStatusBlocked:
			data["filterStatus"] = "blocked"
		}
	}
	data["filterRoleID"] = filterRoleID
	data["roles"] = roles

	c.RenderOrFail(w, r, "admin/people/index.html", data)
}

// peopleListSortURL builds /admin/people?sort=&order= for column header links (resets to page 1).
func peopleListSortURL(r *http.Request, column string) string {
	q := r.URL.Query()
	currentSort := strings.TrimSpace(q.Get("sort"))
	if currentSort == "" {
		currentSort = entity.AdminPeopleSortCreated
	}
	currentOrder := strings.TrimSpace(q.Get("order"))
	if currentOrder != "asc" && currentOrder != "desc" {
		currentOrder = "desc"
	}

	nextOrder := "desc"
	if currentSort == column {
		if currentOrder == "desc" {
			nextOrder = "asc"
		} else {
			nextOrder = "desc"
		}
	}

	v := url.Values{}
	v.Set("sort", column)
	v.Set("order", nextOrder)
	if q.Get("created") == "1" {
		v.Set("created", "1")
	}
	if s := strings.TrimSpace(q.Get("status")); s != "" {
		v.Set("status", s)
	}
	if rid := strings.TrimSpace(q.Get("role")); rid != "" {
		v.Set("role", rid)
	}
	return "/admin/people?" + v.Encode()
}

// PeopleCreateForm shows the admin form to create a user (GET /admin/people/create).
func (c *Controller) PeopleCreateForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Redirect(w, r, "/admin/people/create", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	roles, err := c.assignableRoles(r)
	if err != nil {
		c.LogFailure(err, "error listing roles for people create")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "New user")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["roles"] = roles
	data["errors"] = map[string]string{}
	data["form"] = map[string]string{}

	c.RenderOrFail(w, r, "admin/people/create.html", c.withRequiredFields(r, data))
}

// withRequiredFields adds to the "add user" page's data the required
// profile fields with what was submitted (signupFields).
func (c *Controller) withRequiredFields(r *http.Request, data map[string]any) map[string]any {
	data["signupFields"] = interfaces.RequiredFieldInputs(r, c.userFields)
	return data
}

// PeopleCreate handles POST /admin/people/create.
func (c *Controller) PeopleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/people/create", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")
	roleIDStr := strings.TrimSpace(r.FormValue("role_id"))
	statusStr := strings.TrimSpace(r.FormValue("status"))
	// An invitation unless a password is set now (a form without the
	// choice that sends a password sets it).
	mode := r.FormValue("password_mode")
	invite := mode == "invite" || (mode == "" && password == "")

	form := map[string]string{
		"username":         username,
		"email":            email,
		"role_id":          roleIDStr,
		"status":           statusStr,
		"password":         "",
		"password_confirm": "",
		"password_mode":    "invite",
	}
	if !invite {
		form["password_mode"] = "set"
	}
	inviter := ""
	if me, _ := c.GetCurrentUser(r); me != nil {
		inviter = me.Username
	}

	// The role and the status are the form's to check here (whose roles
	// the admin may give); the rest RegistrationService checks.
	invalid := accounts.SignupInvalid{}
	var role *entity.Role
	if roleID, err := strconv.Atoi(roleIDStr); roleIDStr != "" && (err != nil || roleID <= 0) {
		invalid["role_id"] = i18n.T(ctx, "Invalid role")
	} else if roleIDStr != "" {
		roles, err := c.assignableRoles(r)
		if err != nil {
			c.LogFailure(err, "error listing roles for people create")
			httputil.InternalServerError(w)
			return
		}
		for _, rr := range roles {
			if rr.ID == roleID {
				role = &rr
			}
		}
		if role == nil {
			invalid["role_id"] = i18n.T(ctx, "Role not found")
		}
	}
	blocked := false
	switch statusStr {
	case "", "1":
	case "0":
		blocked = true
	default:
		invalid["status"] = i18n.T(ctx, "Invalid status")
	}

	_, err := c.registration.CreateAccount(ctx, accounts.NewAccount{Username: username, Email: email, Password: password, PasswordConfirm: passwordConfirm,
		Invite: invite, InvitedBy: inviter,
		Role: role, Blocked: blocked, FieldValue: r.FormValue, BaseURL: interfaces.MailBaseURL(), Invalid: invalid})
	var toCorrect accounts.SignupInvalid
	switch {
	case errors.As(err, &toCorrect):
		c.renderPeopleCreate(w, r, form, toCorrect)
		return
	case err != nil:
		c.LogFailure(err, "error creating user from admin")
		c.renderPeopleCreate(w, r, form, map[string]string{"form": i18n.T(ctx, "Error creating the account. Please try again later.")})
		return
	}
	if invite {
		http.Redirect(w, r, "/admin/people?invited="+c.inviteOutcome(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/people?created=1", http.StatusSeeOther)
}

// inviteOutcome - what the list says of an invitation just made or sent
// again: "1" - sent; "nourl" - the site's address isn't set, "off" -
// email is off: the account is there, the invitation not sent.
func (c *Controller) inviteOutcome() string {
	if interfaces.MailBaseURL() == "" {
		return "nourl"
	}
	if c.mailService != nil {
		if st := c.mailService.Status(); !st.Enabled || st.Source == service.MailSourceNone {
			return "off"
		}
	}
	return "1"
}

// invitedTarget - the account of the request's {id} still waiting for its
// invitation (entity.PasswordUnset) that the current administrator may
// manage;
// otherwise the answer is given (nil).
func (c *Controller) invitedTarget(w http.ResponseWriter, r *http.Request) (current, target *entity.User) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return nil, nil
	}
	current, _ = c.GetCurrentUser(r)
	target, err := c.userRepo.ByID(r.Context(), id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading an invited user")
		return nil, nil
	}
	if !service.MayManageAccount(r.Context(), c.authService, current, target) {
		c.RenderForbidden(w, r)
		return nil, nil
	}
	if target.PasswordHash != entity.PasswordUnset {
		http.Redirect(w, r, "/admin/people", http.StatusSeeOther)
		return nil, nil
	}
	return current, target
}

// PeopleInviteResend handles POST /admin/people/{id}/invite: the
// invitation sent again with a new link (the earlier one stops working).
func (c *Controller) PeopleInviteResend(w http.ResponseWriter, r *http.Request) {
	current, target := c.invitedTarget(w, r)
	if target == nil {
		return
	}
	if err := c.registration.SendInvite(r.Context(), target, interfaces.MailBaseURL(), current.Username); err != nil {
		c.LogFailure(err, "sending an invitation again", "user_id", target.ID)
		http.Redirect(w, r, "/admin/people?error=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/people?invited="+c.inviteOutcome(), http.StatusSeeOther)
}

// PeopleInviteCancel handles POST /admin/people/{id}/invite/cancel: an
// invitation not accepted withdrawn - its account, never used, deleted.
func (c *Controller) PeopleInviteCancel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, target := c.invitedTarget(w, r)
	if target == nil {
		return
	}
	for done := false; !done; {
		var err error
		if done, err = c.userDeletionService.DeleteWithContentStep(ctx, target.ID, 5*time.Second); err != nil {
			c.LogFailure(err, "withdrawing an invitation", "user_id", target.ID)
			http.Redirect(w, r, "/admin/people?error=1", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/admin/people?invite_cancelled=1", http.StatusSeeOther)
}

// renderPeopleCreate renders the account form again: what the admin
// typed (form) and what to correct (errs).
func (c *Controller) renderPeopleCreate(w http.ResponseWriter, r *http.Request, form, errs map[string]string) {
	ctx := r.Context()
	roles, err := c.assignableRoles(r)
	if err != nil {
		c.LogFailure(err, "error listing roles for people create")
		httputil.InternalServerError(w)
		return
	}
	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "New user")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["roles"] = roles
	data["errors"] = errs
	data["form"] = form
	c.RenderOrFail(w, r, "admin/people/create.html", c.withRequiredFields(r, data))
}

// PeopleApprove activates a pending user (POST /admin/people/{id}/approve).
func (c *Controller) PeopleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/people", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/people?status=pending", http.StatusSeeOther)
		return
	}
	current, _ := c.GetCurrentUser(r)
	target, err := c.userRepo.ByID(ctx, id)
	if err == nil && !service.MayManageAccount(ctx, c.authService, current, target) {
		c.RenderForbidden(w, r)
		return
	}
	switch err := c.registration.Approve(ctx, id, interfaces.MailBaseURL()); {
	case errors.Is(err, accounts.ErrNotPending) || errors.Is(err, entity.ErrEntityNotFound):
		http.Redirect(w, r, "/admin/people?status=pending", http.StatusSeeOther)
	case err != nil:
		c.LogFailure(err, "people approve update", "user_id", id)
		http.Redirect(w, r, "/admin/people?status=pending&error=1", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/admin/people?status=pending&approved=1", http.StatusSeeOther)
	}
}

// assignableRoles - the roles the current admin may give an account
// (service.AssignableRoles).
func (c *Controller) assignableRoles(r *http.Request) (entity.Roles, error) {
	roles, err := c.roleRepo.List(r.Context())
	if err != nil {
		return nil, err
	}
	editor, _ := c.GetCurrentUser(r)
	return service.AssignableRoles(r.Context(), c.authService, editor, *roles), nil
}

// peopleTwoFactor - the users' two-factor sign-in as the list marks it,
// by id: "on", "missing" (their role needs it, it's off), none.
func (c *Controller) peopleTwoFactor(ctx context.Context, users []entity.User) map[int]string {
	out := map[int]string{}
	if c.twoFactor == nil || len(users) == 0 {
		return out
	}
	ids := make([]int, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	on, err := c.twoFactor.EnabledAmong(ctx, ids)
	if err != nil {
		c.LogFailure(err, "the users' two-factor sign-in")
		return out
	}
	for _, u := range users {
		switch {
		case on[u.ID]:
			out[u.ID] = "on"
		case c.twoFactor.Required(ctx, u):
			out[u.ID] = "missing"
		}
	}
	return out
}

// peopleInvite - an invitation not accepted yet, on the list: when its
// link stops working, whether it has; none sent (NotSent: no link).
type peopleInvite struct {
	Until   time.Time
	Expired bool
	NotSent bool
}

// peopleInvites - the invitations of the listed users without a password
// yet (entity.PasswordUnset), by ID; none on a failure (logged).
func (c *Controller) peopleInvites(ctx context.Context, users entity.Users) map[int]*peopleInvite {
	var ids []int
	for _, u := range users {
		if u.PasswordHash == entity.PasswordUnset {
			ids = append(ids, u.ID)
		}
	}
	out := map[int]*peopleInvite{}
	if len(ids) == 0 || c.registration == nil {
		return out
	}
	until, err := c.registration.PendingInvites(ctx, ids)
	if err != nil {
		c.LogFailure(err, "the users' invitations")
		return out
	}
	now := time.Now()
	for _, id := range ids {
		if t, ok := until[id]; ok {
			out[id] = &peopleInvite{Until: t, Expired: !t.After(now)}
		} else {
			out[id] = &peopleInvite{NotSent: true}
		}
	}
	return out
}
