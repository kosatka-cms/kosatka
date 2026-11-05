package interfaces

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Login - GET and POST /login: the sign-in form, and its submission
// (CAPTCHA, then AuthService.Login; on to the second factor when the
// account has one); a signed-in user goes on to the destination or /me.
func (c *UserController) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			c.RenderBadForm(w, r)
			return
		}

		loginID := strings.TrimSpace(r.FormValue("login"))
		if loginID == "" {
			loginID = strings.TrimSpace(r.FormValue("email"))
		}
		destination := strings.TrimSpace(r.FormValue("destination"))
		if msg := c.captchaFormError(ctx, r, entity.CaptchaFormLogin); msg != "" {
			c.renderLogin(w, r, destination, loginID, msg, "")
			return
		}
		if _, err := c.authService.Login(ctx, w, r, loginID, r.FormValue("pass")); err != nil {
			if errors.Is(err, service.ErrSecondFactorNeeded) {
				http.Redirect(w, r, service.SecondFactorURL(destination), http.StatusFound)
				return
			}
			// The visitor's mistake isn't logged; a failure to check it is.
			if errors.Is(err, service.ErrLoginFailed) {
				c.LogFailure(err, "logging in")
			}
			c.renderLogin(w, r, destination, loginID, i18n.ErrorText(ctx, err), "")
			return
		}

		csrf.EnsureToken(c.store, w, r)

		redirectTo := "/me"
		if dest := service.SafeRedirectPath(r.FormValue("destination")); dest != "" {
			redirectTo = dest
		}
		http.Redirect(w, r, redirectTo, http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	_, loggedIn := c.authService.GetUserID(r)

	if loggedIn {
		redirectTo := "/me"
		if dest := service.SafeRedirectPath(r.URL.Query().Get("destination")); dest != "" {
			redirectTo = dest
		}
		http.Redirect(w, r, redirectTo, http.StatusFound)
	} else {
		c.renderLogin(w, r, r.URL.Query().Get("destination"), "", "", r.URL.Query().Get("msg"))
	}
}

// renderLogin renders the login page: where to go after it (destination),
// what the visitor typed (login), the error of a failed try (loginError)
// or the message a redirect here names (msg, applyLoginFlashMessage).
func (c *UserController) renderLogin(w http.ResponseWriter, r *http.Request, destination, login, loginError, msg string) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/html")
	data := c.GetCommonData(r)
	data["destination"] = destination
	if login != "" {
		data["login"] = login
	}
	if loginError != "" {
		data["login_error"] = loginError
	}
	applyLoginFlashMessage(ctx, data, msg)
	data["recaptcha_site_key"] = captcha.SiteKey(entity.CaptchaFormLogin)
	title := i18n.T(ctx, "User login")
	loginSEO := PageSEOForList(title, r.URL.Path)
	loginSEO.NoIndex = true
	data["pageSEO"] = loginSEO
	data["title"] = title
	c.setPathTrail(data, r.URL.Path, title)
	data["registrationOpen"] = c.registrationOpen(ctx)
	// A service page: the card in the middle, no sidebar; the social
	// networks' buttons by name, side by side.
	data["servicePage"] = true
	data["oauthCompact"] = true
	c.RenderOrFail(w, r, "user/login.html", data)
}

// registrationOpen - whether visitors can create accounts (the login
// page then links to registration).
func (c *UserController) registrationOpen(ctx context.Context) bool {
	open, err := c.registration.Open(ctx)
	return err == nil && open
}

// LogoutConfirm - GET /logout: a page with the logout button (Logout is a
// POST, checked by CSRFMiddleware, so another site can't log a visitor
// out); a visitor who isn't logged in goes to the front page.
func (c *UserController) LogoutConfirm(w http.ResponseWriter, r *http.Request) {
	if _, loggedIn := c.authService.GetUserID(r); !loggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	csrf.EnsureToken(c.store, w, r)
	data := c.GetCommonData(r)
	data["title"] = i18n.T(r.Context(), "Log out")
	data["pageSEO"] = PageSEOForList(i18n.T(r.Context(), "Log out"), r.URL.Path)
	data["servicePage"] = true // the sign-in pages: a card, no sidebar
	c.RenderOrFail(w, r, "user/logout.html", data)
}

// Logout - POST /logout.
func (c *UserController) Logout(w http.ResponseWriter, r *http.Request) {
	err := c.authService.Logout(r.Context(), w, r)
	if err != nil {
		httputil.InternalServerError(w)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

// applyLoginFlashMessage - the login page's message for ?msg= (a redirect
// here names what happened): an error or a notice.
func applyLoginFlashMessage(ctx context.Context, data map[string]any, msg string) {
	switch msg {
	case "oauth_error":
		data["login_error"] = i18n.T(ctx, "Could not log in with the social network. Try again or use your username and password.")
	case "oauth_email_taken":
		data["login_error"] = i18n.T(ctx, "An account with this e-mail address already exists. Log in with its username and password, then connect the social network in the profile.")
	case "oauth_no_email":
		data["login_error"] = i18n.T(ctx, "The social network didn't give an e-mail address to confirm. Register with the form, then connect the social network in the profile.")
	case "register_disabled":
		data["login_error"] = i18n.T(ctx, "New user registration is disabled.")
	case "invalid_token":
		data["login_error"] = i18n.T(ctx, "The link is invalid or has expired.")
	case "registered":
		data["login_notice"] = i18n.T(ctx, "Registration is complete. Log in.")
	case "password_reset":
		data["login_notice"] = i18n.T(ctx, "The password is changed. Log in with the new one.")
	case "account_deleted":
		data["login_notice"] = i18n.T(ctx, "Your account is deleted.")
	case "email_confirmed":
		data["login_notice"] = i18n.T(ctx, "The email address is confirmed. Log in.")
	case "second_factor":
		data["login_error"] = i18n.T(ctx, "Too many wrong codes, or too much time has passed. Sign in again.")
	case "pending_approval":
		data["login_notice"] = i18n.T(ctx, "The email address is confirmed. The account is awaiting administrator approval.")
	}
}

// ConfirmEmail - GET /user/confirm-email?token=: the account's email
// address confirmed by the link's token, the user signed in when the
// account may sign in (else the sign-in page says why not).
func (c *UserController) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" || c.userTokenService == nil {
		http.Redirect(w, r, "/login?msg=invalid_token", http.StatusFound)
		return
	}
	uid, ok, err := c.userTokenService.ConsumeEmailConfirm(ctx, token)
	if err != nil {
		c.LogFailure(err, "using an e-mail confirmation token")
	}
	if !ok {
		http.Redirect(w, r, "/login?msg=invalid_token", http.StatusFound)
		return
	}
	user, err := c.userService.ConfirmEmail(ctx, uid)
	if err != nil {
		c.LogFailure(err, "confirming the e-mail", "user_id", uid)
		http.Redirect(w, r, "/login?msg=invalid_token", http.StatusFound)
		return
	}

	// Logged in as at the login form (AuthService.LoginUser: not a blocked
	// account, not one awaiting approval); else the login page says the
	// email is confirmed and, if so, that the account awaits approval.
	if err := c.authService.LoginUser(ctx, w, r, user); err != nil {
		if errors.Is(err, service.ErrSecondFactorNeeded) {
			http.Redirect(w, r, service.SecondFactorURL("/me"), http.StatusFound)
			return
		}
		if errors.Is(err, service.ErrLoginFailed) {
			c.LogFailure(err, "logging in after the email confirmation", "user_id", user.ID)
		}
		msg := "email_confirmed"
		if user.Status == entity.UserStatusPending {
			msg = "pending_approval"
		}
		http.Redirect(w, r, "/login?msg="+msg, http.StatusFound)
		return
	}
	http.Redirect(w, r, "/me", http.StatusFound)
}

// LoginOnce exchanges a one-time login token for a session and redirects to profile edit.
func (c *UserController) LoginOnce(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" || c.userTokenService == nil {
		http.Redirect(w, r, "/login?msg=invalid_token", http.StatusFound)
		return
	}
	uid, ok, err := c.userTokenService.ConsumeOneTimeLogin(ctx, token)
	if err != nil || !ok {
		if c.logger != nil {
			c.logger.Warn("login-once failed", "ok", ok, "error", err)
		}
		http.Redirect(w, r, "/login?msg=invalid_token", http.StatusFound)
		return
	}
	u, err := c.userService.ByID(ctx, uid)
	if err != nil {
		c.LogFailure(err, "loading the one-time login's user", "user_id", uid)
		http.Redirect(w, r, "/login?msg=invalid_token", http.StatusFound)
		return
	}
	// As at the login form (AuthService.LoginUser): a blocked account or
	// one awaiting approval isn't let in by a link either.
	if err := c.authService.LoginUser(ctx, w, r, u); err != nil {
		// A one-time link (a password reset) doesn't skip the code.
		if errors.Is(err, service.ErrSecondFactorNeeded) {
			http.Redirect(w, r, service.SecondFactorURL("/me/edit"), http.StatusFound)
			return
		}
		if errors.Is(err, service.ErrLoginFailed) {
			c.LogFailure(err, "logging in by a one-time link", "user_id", u.ID)
		}
		http.Redirect(w, r, "/login?msg=invalid_token", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/me/edit", http.StatusFound)
}
