package interfaces

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
	"golang.org/x/crypto/bcrypt"
)

// errEditForbidden - the signed-in user may not edit the account asked
// for: a 403, not the sign-in page.
var errEditForbidden = errors.New("may not edit this account")

// editFormTarget - the account the profile form edits and whether it is
// the user's own or one they administer: /user/edit - their own;
// /user/{id}/edit - that account, for who may administer users and
// manage it (errEditForbidden otherwise; an id that isn't a user - a
// not-found error). Nil, no error - a guest.
func (c *UserController) editFormTarget(ctx context.Context, r *http.Request) (*entity.User, bool, bool, error) {
	userID, loggedIn := c.authService.GetUserID(r)
	if !loggedIn {
		return nil, false, false, nil
	}
	currentUser, err := c.userService.ByID(ctx, userID)
	if err != nil {
		return nil, false, false, err
	}

	pathID := r.PathValue("id")
	if pathID == "" {
		return currentUser, true, false, nil
	}

	if !c.authService.HasPermission(ctx, currentUser, service.PermAdministerUsers) {
		return nil, false, false, errEditForbidden
	}
	targetID, err := strconv.Atoi(pathID)
	if err != nil {
		return nil, false, false, fmt.Errorf("user %q: %w", pathID, entity.ErrEntityNotFound)
	}
	targetUser, err := c.userService.ByID(ctx, targetID)
	if err != nil {
		return nil, false, false, err
	}
	// Not an account with rights the editor may not give.
	if !service.MayManageAccount(ctx, c.authService, currentUser, targetUser) {
		return nil, false, false, errEditForbidden
	}
	return targetUser, false, true, nil
}

// roleChoices - the roles the editor may give target
// (service.AssignableRoles); none when target's own role is one the
// editor may not take away.
func (c *UserController) roleChoices(ctx context.Context, r *http.Request, target *entity.User) entity.Roles {
	editor, err := c.GetCurrentUser(r)
	if err != nil || editor == nil || c.roleRepo == nil {
		return nil
	}
	if target.Role != nil && !service.MayAssignRole(ctx, c.authService, editor, target.Role) {
		return nil
	}
	roles, err := c.roleRepo.List(ctx)
	if err != nil {
		c.LogFailure(err, "error listing roles for the profile form")
		return nil
	}
	return service.AssignableRoles(ctx, c.authService, editor, *roles)
}

// EditForm - GET /me/edit and /user/{id}/edit: the profile edit form of
// the signed-in user, or of another one for who administers users.
func (c *UserController) EditForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	targetUser, isSelf, isAdmin, err := c.editFormTarget(ctx, r)
	if errors.Is(err, errEditForbidden) {
		c.RenderForbidden(w, r)
		return
	}
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the user to edit", "id", r.PathValue("id"))
		return
	}
	if targetUser == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	data := c.GetCommonData(r)
	editForPage := *targetUser
	editForPage.Avatar = imageurl.Avatar(targetUser.Avatar, imageurl.AvatarProfileStyle)
	data["editUser"] = &editForPage
	data["userTimezonesEnabled"] = sitetime.UserZonesEnabled()
	data["timezones"] = sitetime.ZoneOptions(targetUser.Timezone)
	data["userFields"] = c.userFieldValues(ctx, targetUser.ID)
	data["moduleSettings"] = c.loadUserSettingsSections(ctx, *targetUser)
	data["isSelf"] = isSelf
	data["isAdmin"] = isAdmin
	data["formErrors"] = r.URL.Query()["error"]
	// The profile's header and tabs (profile_head), the "Edit" tab active;
	// a service page: no sidebar.
	profileBase := "/" + editForPage.Slug
	if isSelf {
		profileBase = "/me"
	}
	viewer, _ := c.GetCurrentUser(r)
	canAdminister := c.authService.HasPermission(ctx, viewer, service.PermAdministerUsers)
	// Two-factor sign-in: the user's own page link; an administrator may
	// reset someone else's.
	if c.twoFactor != nil {
		on, err := c.twoFactor.Enabled(ctx, targetUser.ID)
		if err != nil {
			c.LogFailure(err, "the two-factor sign-in for the profile form", "user_id", targetUser.ID)
		}
		data["twoFactor"] = true
		data["twoFactorOn"] = on
		data["canResetTwoFactor"] = on && !isSelf && canAdminister
		data["twoFactorReset"] = r.URL.Query().Get("two_factor_reset") == "1"
	}
	// The social networks to sign in with: one's own profile connects and
	// disconnects them.
	if isSelf && c.socialLogin != nil {
		if buttons := oauthButtons(r); len(buttons) > 0 {
			connected, err := c.socialLogin.Connected(ctx, targetUser.ID)
			if err != nil {
				c.LogFailure(err, "the social networks connected, for the profile form", "user_id", targetUser.ID)
			}
			data["oauthServices"] = oauthServiceRows(buttons, connected)
			data["oauthMessage"] = oauthProfileMessage(ctx, r.URL.Query().Get("oauth"))
		}
	}
	data["profileUser"] = &editForPage
	data["isCurrentUser"] = isSelf
	// The "Delete account" card: one's own, with the permission.
	if isSelf && c.mayCancelAccount(r, viewer) {
		data["canCancelAccount"] = true
		data["cancelDeletesContent"] = c.cancelDeletesContent(r)
		data["cancelError"] = r.URL.Query().Get("cancel_error")
	}
	data["canEdit"] = true
	data["activeTab"] = "edit"
	data["profileModuleTabs"] = c.profileModuleTabs(ctx, viewer, targetUser, isSelf, canAdminister, profileBase, "")
	data["profileViewTabs"] = c.profileViewTabs(ctx, targetUser.ID, true, profileBase)
	data["servicePage"] = true
	data["title"] = i18n.T(r.Context(), "Edit profile")
	data["pageSEO"] = PageSEOForProfileEdit(&editForPage, r.URL.Path)

	if isAdmin {
		if roles := c.roleChoices(ctx, r, targetUser); len(roles) > 0 {
			data["roles"] = roles
		}
		// defaultRoleName - preselected for a user without a role: the
		// registered users' role.
		data["defaultRoleName"] = entity.RoleUser
	}

	profileURL := "/" + editForPage.Slug
	if isSelf {
		profileURL = "/me"
	}
	c.setChildTrail(data, []entity.BreadcrumbItem{{URL: profileURL, Label: editForPage.Username}}, i18n.T(r.Context(), "Editing"))

	c.RenderOrFail(w, r, "user/edit_form.html", data)
}

// EditSave - POST /me/edit and /user/{id}/edit: the profile edit form
// saved (the account, its fields and the modules' sections), back to the
// profile or to the form with its error.
func (c *UserController) EditSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	targetUser, isSelf, isAdmin, err := c.editFormTarget(ctx, r)
	if errors.Is(err, errEditForbidden) {
		c.RenderForbidden(w, r)
		return
	}
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the user to save", "id", r.PathValue("id"))
		return
	}
	if targetUser == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	form := r.Form
	// Lowercase, as registration stores it.
	username, email := strings.TrimSpace(form.Get("username")), strings.TrimSpace(strings.ToLower(form.Get("email")))
	password := form.Get("password")

	fieldValues, msg, err := c.checkEditForm(ctx, r, targetUser, isSelf, username, email)
	if err != nil {
		c.LogFailure(err, "checking the user edit form")
		httputil.InternalServerError(w)
		return
	}
	if msg != "" {
		c.redirectEditWithError(w, r, targetUser.ID, isSelf, isAdmin, msg)
		return
	}

	updated := *targetUser
	updated.Username = username
	updated.Slug = entity.UserSlug(username)
	updated.Email = email
	// A new address isn't confirmed by being typed: until its link is
	// followed (or a password reset reaches it), it isn't the account's
	// confirmed one.
	emailChanged := email != targetUser.Email
	if emailChanged {
		updated.EmailVerified = false
	}
	updated.Timezone = userTimezoneFromForm(form.Get("timezone"), targetUser.Timezone)
	updated.Signature = htmlsanitize.Sanitize(form.Get("signature"))
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			c.LogFailure(err, "bcrypt error")
			httputil.InternalServerError(w)
			return
		}
		updated.PasswordHash = string(hash)
	}
	if isAdmin {
		c.applyAdminFields(ctx, r, targetUser, &updated)
	}
	if msg := c.applyAvatar(ctx, r, targetUser.ID, &updated); msg != "" {
		c.redirectEditWithError(w, r, targetUser.ID, isSelf, isAdmin, msg)
		return
	}

	if _, err := c.userService.Edit(ctx, updated); err != nil {
		c.LogFailure(err, "user edit error")
		httputil.InternalServerError(w)
		return
	}
	// A new password ends the account's other sessions (one opened with
	// a stolen password or cookie among them).
	if password != "" {
		if err := c.authService.EndSessions(ctx, w, r, targetUser.ID); err != nil {
			c.LogFailure(err, "ending the sessions after a password change", "userID", targetUser.ID)
			httputil.InternalServerError(w)
			return
		}
	}
	if emailChanged && c.registration != nil {
		c.registration.MailEmailConfirmation(ctx, &updated, mailBaseURL())
	}
	if isAdmin {
		service.AuditUserStatus(ctx, updated, targetUser.Status, updated.Status)
		service.AuditUserRole(ctx, updated, roleLabel(targetUser.Role), roleLabel(updated.Role))
	}
	if c.userTimezones != nil {
		c.userTimezones.Forget(targetUser.ID)
	}
	if c.userFields != nil {
		if err := c.userFields.SaveValues(ctx, targetUser.ID, fieldValues); err != nil {
			c.LogFailure(err, "error saving user field values", "userID", targetUser.ID)
			httputil.InternalServerError(w)
			return
		}
	}
	// The modules' sections (AddUserSettingsSection).
	if err := saveUserSettingsSections(ctx, *targetUser, r); err != nil {
		c.LogFailure(err, "saving the modules' user settings", "userID", targetUser.ID)
		httputil.InternalServerError(w)
		return
	}

	if isSelf {
		http.Redirect(w, r, "/me", http.StatusFound)
	} else {
		http.Redirect(w, r, fmt.Sprintf("/user/%d/edit", targetUser.ID), http.StatusFound)
	}
}

// checkEditForm checks the edit form of target (isSelf - by themselves)
// with its new username and email: the user fields' values, else the
// message to show (msg), or a failure (err).
func (c *UserController) checkEditForm(ctx context.Context, r *http.Request, target *entity.User, isSelf bool, username, email string) (map[int]string, string, error) {
	form := r.Form
	password, passwordConfirm := form.Get("password"), form.Get("password_confirm")
	// The checks of registration: allowed characters, a well-formed email,
	// both unique (another user's taken name would fail on the database's
	// unique index).
	if msg := accounts.ValidateUsername(ctx, c.userService, username, target.ID); msg != "" {
		return nil, msg, nil
	}
	if msg := accounts.ValidateEmail(ctx, c.userService, email, target.ID); msg != "" {
		return nil, msg, nil
	}
	var fieldValues map[int]string
	if c.userFields != nil {
		var err error
		fieldValues, err = c.userFields.FormValues(ctx, form.Get)
		var invalid accounts.UserFieldFormError
		if errors.As(err, &invalid) {
			return nil, invalid.Message(ctx), nil
		}
		if err != nil {
			return nil, "", fmt.Errorf("reading user fields: %w", err)
		}
	}
	// The current password guards the sign-in only: a user changing their
	// own e-mail or password gives it; the other details need none.
	if isSelf && (!strings.EqualFold(email, target.Email) || password != "" || passwordConfirm != "") {
		ok, err := c.authService.VerifyPassword(ctx, target.ID, form.Get("current_password"))
		if err != nil {
			return nil, "", fmt.Errorf("verifying the password: %w", err)
		}
		if !ok {
			return nil, i18n.T(ctx, "The current password is incorrect"), nil
		}
	}
	// A new password only when typed: registration's rules.
	if password != "" || passwordConfirm != "" {
		passwordErr, confirmErr := accounts.ValidatePassword(ctx, password, passwordConfirm)
		if msg := cmp.Or(confirmErr, passwordErr); msg != "" {
			return nil, msg, nil
		}
	}
	return fieldValues, "", nil
}

// applyAdminFields sets what only an administrator edits on updated (of
// target): its status and its role - only one the editor may give
// (roleChoices); another is ignored.
func (c *UserController) applyAdminFields(ctx context.Context, r *http.Request, target *entity.User, updated *entity.User) {
	switch r.Form.Get("status") {
	case strconv.Itoa(entity.UserStatusBlocked):
		updated.Status = entity.UserStatusBlocked
	case strconv.Itoa(entity.UserStatusPending):
		updated.Status = entity.UserStatusPending
	default:
		updated.Status = entity.UserStatusActive
	}
	if roleID, err := strconv.Atoi(r.Form.Get("role_id")); err == nil {
		for _, role := range c.roleChoices(ctx, r, target) {
			if role.ID == roleID {
				updated.Role = &role
			}
		}
	}
}

// applyAvatar sets updated's avatar from the form: deleted, a new one
// uploaded, or kept; the message of a failed upload, else "".
func (c *UserController) applyAvatar(ctx context.Context, r *http.Request, userID int, updated *entity.User) string {
	if r.Form.Get("delete_avatar") == "1" {
		updated.Avatar = entity.File{}
		updated.AvatarUpdateHint = "clear"
		return ""
	}
	file, header, err := r.FormFile("avatar")
	if err != nil || header == nil || header.Size <= 0 {
		updated.AvatarUpdateHint = "keep"
		return ""
	}
	defer func() { _ = file.Close() }()
	if c.fileUploadService == nil {
		return ""
	}
	uri, err := c.fileUploadService.UploadUserAvatar(ctx, file, header, userID)
	if err != nil {
		c.LogFailure(err, "avatar upload error")
		return i18n.T(ctx, "Image upload error: @error", "@error", i18n.ErrorText(ctx, err))
	}
	updated.Avatar = entity.File{URI: uri}
	updated.AvatarUpdateHint = "set"
	return ""
}

func (c *UserController) redirectEditWithError(w http.ResponseWriter, r *http.Request, userID int, isSelf, isAdmin bool, errMsg string) {
	enc := url.QueryEscape(errMsg)
	if isSelf {
		http.Redirect(w, r, "/me/edit?error="+enc, http.StatusFound)
	} else {
		http.Redirect(w, r, fmt.Sprintf("/user/%d/edit?error=%s", userID, enc), http.StatusFound)
	}
}

func (c *UserController) userFieldValues(ctx context.Context, userID int) []entity.UserFieldValue {
	if c.userFields == nil {
		return nil
	}
	values, err := c.userFields.UserValues(ctx, userID)
	if err != nil {
		c.LogFailure(err, "error loading user field values", "userID", userID)
	}
	return values
}

// userTimezoneFromForm - a user's time zone from a form ("" - the
// site's): kept as current while user time zones are off or the value
// isn't a valid zone.
func userTimezoneFromForm(value, current string) string {
	value = strings.TrimSpace(value)
	if !sitetime.UserZonesEnabled() {
		return current
	}
	if value == "" {
		return ""
	}
	if _, err := sitetime.Load(value); err != nil {
		return current
	}
	return value
}

// roleLabel - a user's role as shown ("" - none).
func roleLabel(r *entity.Role) string {
	if r == nil {
		return ""
	}
	return r.Label()
}
