package interfaces

import (
	"errors"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// mayCancelAccount - whether user may delete their own account here: the
// permission, and not an administrator (UserDeletionService.Cancel).
func (c *UserController) mayCancelAccount(r *http.Request, user *entity.User) bool {
	return c.cancel != nil && user != nil && !service.HasGrant(user.Role, service.PermFullAccess) &&
		c.authService.HasPermission(r.Context(), user, service.PermCancelOwnAccount)
}

// cancelDeletesContent - whether the site's account settings say a
// deleted account takes its content too (entity.CancelDeleteContent).
func (c *UserController) cancelDeletesContent(r *http.Request) bool {
	if c.accountPolicy == nil {
		return false
	}
	s, err := c.accountPolicy.Settings(r.Context())
	if err != nil {
		c.LogFailure(err, "the account settings for deleting an account")
		return false
	}
	return s.CancelContent == entity.CancelDeleteContent
}

// CancelAccount handles POST /me/cancel: the signed-in user deletes their
// own account - with the permission, their password and the "can't be
// undone" tick; signed out, to the sign-in page saying so. A wrong password or no
// tick - back to the profile form with the reason.
func (c *UserController) CancelAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := c.GetCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !c.mayCancelAccount(r, user) {
		c.RenderForbidden(w, r)
		return
	}
	back := func(reason string) {
		http.Redirect(w, r, "/me/edit?cancel_error="+reason+"#account-cancel", http.StatusSeeOther)
	}
	if r.FormValue("confirm") != "1" {
		back("confirm")
		return
	}
	ok, err := c.authService.VerifyPassword(ctx, user.ID, r.FormValue("password"))
	if err != nil {
		c.LogFailure(err, "checking the password to delete an account", "userID", user.ID)
		back("failed")
		return
	}
	if !ok {
		back("password")
		return
	}
	if err := c.cancel.Cancel(ctx, *user, c.cancelDeletesContent(r)); err != nil {
		if !errors.Is(err, service.ErrCancelAdministrator) {
			c.LogFailure(err, "deleting an account by its owner", "userID", user.ID)
		}
		back("failed")
		return
	}
	if err := c.authService.Logout(ctx, w, r); err != nil {
		c.LogFailure(err, "signing out a deleted account", "userID", user.ID)
	}
	http.Redirect(w, r, "/login?msg=account_deleted", http.StatusSeeOther)
}
