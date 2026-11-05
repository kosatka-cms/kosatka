package interfaces

import (
	"errors"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// AcceptInvite handles GET/POST /user/invite?token=…: the invited person
// chooses their login (the one made for them offered) and sets their
// password; the account saved, they are signed in. A link no good -
// the page says so.
func (c *UserController) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			c.RenderBadForm(w, r)
			return
		}
		token = r.FormValue("token")
		username := r.FormValue("username")
		user, err := c.registration.AcceptInvite(ctx, token, username, r.FormValue("password"), r.FormValue("password_confirm"))
		var toCorrect accounts.SignupInvalid
		switch {
		case errors.As(err, &toCorrect):
			c.renderInvite(w, r, token, username, nil, toCorrect)
			return
		case errors.Is(err, accounts.ErrInviteInvalid):
			c.renderInvite(w, r, "", "", nil, nil)
			return
		case err != nil:
			c.LogFailure(err, "accepting an invitation")
			c.renderInvite(w, r, token, username, nil, map[string]string{"form": i18n.T(ctx, "Error saving. Please try again later.")})
			return
		}
		// As at the login form: the account's role may need the second
		// factor, a blocked account isn't let in.
		if err := c.authService.LoginUser(ctx, w, r, user); err != nil {
			if errors.Is(err, service.ErrSecondFactorNeeded) {
				http.Redirect(w, r, service.SecondFactorURL("/me"), http.StatusSeeOther)
				return
			}
			c.LogFailure(err, "signing in an invited user", "user_id", user.ID)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/me", http.StatusSeeOther)
		return
	}

	user, err := c.registration.InvitedUser(ctx, token)
	switch {
	case errors.Is(err, accounts.ErrInviteInvalid):
		c.renderInvite(w, r, "", "", nil, nil)
	case err != nil:
		c.RenderLoadFailure(w, r, err, "loading an invitation")
	default:
		c.renderInvite(w, r, token, user.Username, user, nil)
	}
}

// renderInvite renders the invitation's page: token "" - the link is no
// good; username - the login offered (or typed); user - the invited
// account (its role named), nil after a submission; errs - what to
// correct.
func (c *UserController) renderInvite(w http.ResponseWriter, r *http.Request, token, username string, user *entity.User, errs map[string]string) {
	ctx := r.Context()
	data := c.GetCommonData(r)
	data["title"] = i18n.T(ctx, "Invitation")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "Invitation")}
	data["token"] = token
	data["username"] = username
	data["invited"] = user
	if errs == nil {
		errs = map[string]string{}
	}
	data["errors"] = errs
	c.setPathTrail(data, r.URL.Path, i18n.T(ctx, "Invitation"))
	data["servicePage"] = true // the sign-in pages: a card, no sidebar
	c.RenderOrFail(w, r, "user/invite.html", data)
}
