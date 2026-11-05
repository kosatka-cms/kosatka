package interfaces

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/twofactor"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TwoFactorPath - the user's own two-factor sign-in page.
const TwoFactorPath = "/me/two-factor"

// twoFactorSetupSession - the session holding the secret being turned on
// (shown as the QR code) until a code confirms it.
const twoFactorSetupSession = "two-factor-setup"

// LoginCode handles GET/POST /login/code, the second step of a sign-in:
// the code of the app (or a recovery code, ?recovery=1 says so) for the
// pending sign-in; right - signed in, to the destination; wrong - again;
// too many or too late, or none pending - the login page.
func (c *UserController) LoginCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := c.authService.PendingSecondFactor(ctx, r)
	if !ok {
		http.Redirect(w, r, "/login?msg=second_factor", http.StatusFound)
		return
	}
	destination := service.SafeRedirectPath(r.FormValue("destination"))
	var codeError string
	if r.Method == http.MethodPost {
		if r.FormValue("cancel") == "1" {
			c.authService.CancelSecondFactor(w, r)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		_, err := c.authService.CompleteSecondFactor(ctx, w, r, r.FormValue("code"))
		if err == nil {
			csrf.EnsureToken(c.store, w, r)
			if destination == "" {
				destination = "/me"
			}
			http.Redirect(w, r, destination, http.StatusFound)
			return
		}
		if errors.Is(err, service.ErrLoginFailed) {
			c.LogFailure(err, "checking the two-factor code")
		}
		if _, still := c.authService.PendingSecondFactor(ctx, r); !still {
			http.Redirect(w, r, "/login?msg=second_factor", http.StatusFound)
			return
		}
		codeError = i18n.ErrorText(ctx, err)
	}
	data := c.GetCommonData(r)
	title := i18n.T(ctx, "Confirmation code")
	seo := PageSEOForList(title, r.URL.Path)
	seo.NoIndex = true
	data["pageSEO"] = seo
	data["title"] = title
	data["username"] = user.Username
	data["destination"] = destination
	data["recovery"] = r.FormValue("recovery") == "1"
	data["codeError"] = codeError
	data["servicePage"] = true // the sign-in pages: a card, no sidebar
	c.RenderOrFail(w, r, "user/login_code.html", data)
}

// TwoFactor renders GET /me/two-factor: while it's off, turning it on -
// a new secret (kept in the session) as a QR code and as text, the code
// confirming it; while on, since when, the recovery codes left, new ones,
// turning it off (not while the role needs it).
func (c *UserController) TwoFactor(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil || c.twoFactor == nil {
		http.Redirect(w, r, "/login?destination="+TwoFactorPath, http.StatusFound)
		return
	}
	status, err := c.twoFactor.Status(ctx, user.ID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the two-factor sign-in", "user_id", user.ID)
		return
	}
	data := c.GetCommonData(r)
	title := i18n.T(ctx, "Two-factor sign-in")
	seo := PageSEOForList(title, r.URL.Path)
	seo.NoIndex = true
	data["pageSEO"] = seo
	data["title"] = title
	data["status"] = status
	data["required"] = c.twoFactor.Required(ctx, *user)
	q := r.URL.Query()
	data["error"] = q.Get("error")
	data["off"] = q.Get("off") == "1"
	if status == nil {
		secret, err := c.setupSecret(w, r, user.ID)
		if err != nil {
			c.RenderLoadFailure(w, r, err, "a two-factor secret", "user_id", user.ID)
			return
		}
		setup, err := c.twoFactor.SetupFor(ctx, *user, secret)
		if err != nil {
			c.RenderLoadFailure(w, r, err, "the two-factor QR code", "user_id", user.ID)
			return
		}
		data["setup"] = setup
		// The QR code is the engine's own SVG (twofactor.QRSVG).
		data["qr"] = template.HTML(setup.QR) //nolint:gosec // built here from the secret's address, no user text.
	}
	c.RenderOrFail(w, r, "user/two_factor.html", data)
}

// setupSecret - the secret userID is turning on: the session's, or a new
// one kept there with whose it is (TwoFactorEnable takes it for that user
// only).
func (c *UserController) setupSecret(w http.ResponseWriter, r *http.Request, userID int) (string, error) {
	session, err := c.store.Get(r, twoFactorSetupSession)
	if err != nil {
		session, _ = c.store.New(r, twoFactorSetupSession)
	}
	if s, ok := session.Values["secret"].(string); ok && s != "" && session.Values["user_id"] == userID {
		return s, nil
	}
	secret, err := c.twoFactor.NewSecret()
	if err != nil {
		return "", err
	}
	session.Values["secret"] = secret
	session.Values["user_id"] = userID
	session.Options.MaxAge = 3600
	session.Options.HttpOnly = true
	if err := session.Save(r, w); err != nil {
		return "", err
	}
	return secret, nil
}

// forgetSetupSecret drops the secret being turned on.
func (c *UserController) forgetSetupSecret(w http.ResponseWriter, r *http.Request) {
	if session, err := c.store.Get(r, twoFactorSetupSession); err == nil {
		session.Options.MaxAge = -1
		_ = session.Save(r, w)
	}
}

// TwoFactorEnable handles POST /me/two-factor/enable: the code confirms
// the session's secret - on, and the recovery codes shown once.
func (c *UserController) TwoFactorEnable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil || c.twoFactor == nil {
		c.RenderForbidden(w, r)
		return
	}
	// The secret of this user's setup only: not one set up in this
	// browser for another account.
	session, err := c.store.Get(r, twoFactorSetupSession)
	secret, _ := session.Values["secret"].(string)
	if err != nil || secret == "" || session.Values["user_id"] != user.ID {
		http.Redirect(w, r, TwoFactorPath, http.StatusSeeOther)
		return
	}
	codes, err := c.twoFactor.Enable(ctx, *user, secret, r.FormValue("code"))
	switch {
	case errors.Is(err, twofactor.ErrAlreadyOn):
		c.forgetSetupSecret(w, r)
		http.Redirect(w, r, TwoFactorPath, http.StatusSeeOther)
		return
	case err != nil:
		if !errors.Is(err, twofactor.ErrWrongCode) {
			c.LogFailure(err, "turning on the two-factor sign-in", "user_id", user.ID)
		}
		http.Redirect(w, r, TwoFactorPath+"?error=code", http.StatusSeeOther)
		return
	}
	c.forgetSetupSecret(w, r)
	// The account's other sessions end: opened without the second factor.
	if err := c.authService.EndSessions(ctx, w, r, user.ID); err != nil {
		c.LogFailure(err, "ending the sessions after turning on the two-factor sign-in", "user_id", user.ID)
	}
	c.renderRecoveryCodes(w, r, codes, true)
}

// TwoFactorCodes handles POST /me/two-factor/codes: new recovery codes,
// once a code proves it's the user; shown once.
func (c *UserController) TwoFactorCodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil || c.twoFactor == nil {
		c.RenderForbidden(w, r)
		return
	}
	codes, err := c.twoFactor.NewRecoveryCodes(ctx, *user, r.FormValue("code"))
	if err != nil {
		if !onlyWrongCode(err) {
			c.LogFailure(err, "new recovery codes", "user_id", user.ID)
		}
		http.Redirect(w, r, TwoFactorPath+"?error=code", http.StatusSeeOther)
		return
	}
	c.renderRecoveryCodes(w, r, codes, false)
}

// onlyWrongCode - err is the wrong code alone, nothing failed besides.
func onlyWrongCode(err error) bool {
	j, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return errors.Is(err, twofactor.ErrWrongCode)
	}
	for _, e := range j.Unwrap() {
		if e != nil && !errors.Is(e, twofactor.ErrWrongCode) {
			return false
		}
	}
	return true
}

// TwoFactorDisable handles POST /me/two-factor/disable: off, once a code
// proves it's the user - not while their role needs it.
func (c *UserController) TwoFactorDisable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil || c.twoFactor == nil {
		c.RenderForbidden(w, r)
		return
	}
	if err := c.twoFactor.Disable(ctx, *user, r.FormValue("code")); err != nil {
		code := "code"
		if errors.Is(err, twofactor.ErrRequired) {
			code = "required"
		} else if !onlyWrongCode(err) {
			c.LogFailure(err, "turning off the two-factor sign-in", "user_id", user.ID)
		}
		http.Redirect(w, r, TwoFactorPath+"?error="+code, http.StatusSeeOther)
		return
	}
	// Turned off by someone holding a session and a code: the other
	// sessions end, whoever's they are.
	if err := c.authService.EndSessions(ctx, w, r, user.ID); err != nil {
		c.LogFailure(err, "ending the sessions after turning off the two-factor sign-in", "user_id", user.ID)
	}
	http.Redirect(w, r, TwoFactorPath+"?off=1", http.StatusSeeOther)
}

// renderRecoveryCodes shows the recovery codes once: to write down,
// download, copy or print (just - it was turned on now).
func (c *UserController) renderRecoveryCodes(w http.ResponseWriter, r *http.Request, codes []string, just bool) {
	ctx := r.Context()
	data := c.GetCommonData(r)
	title := i18n.T(ctx, "Recovery codes")
	seo := PageSEOForList(title, r.URL.Path)
	seo.NoIndex = true
	data["pageSEO"] = seo
	data["title"] = title
	data["codes"] = codes
	data["codesText"] = strings.Join(codes, "\n") + "\n"
	// A text file to download, built here from the codes alone.
	data["codesURL"] = template.URL("data:text/plain;charset=utf-8," + url.PathEscape(data["codesText"].(string))) //nolint:gosec // the codes, no user text.
	data["justEnabled"] = just
	w.Header().Set("Cache-Control", "no-store")
	c.RenderOrFail(w, r, "user/two_factor_codes.html", data)
}
