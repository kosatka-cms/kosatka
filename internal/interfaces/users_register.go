package interfaces

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Signup - GET and POST /user/register: the sign-up form (or a page saying
// sign-up is closed), and its submission (handleSignupPost); a signed-in
// user goes to /me.
func (c *UserController) Signup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	if _, loggedIn := c.authService.GetUserID(r); loggedIn {
		http.Redirect(w, r, "/me", http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
		c.handleSignupPost(w, r)
		return
	}

	if open, err := c.registration.Open(r.Context()); err == nil && !open {
		c.renderSignupClosed(w, r, r.URL.Query().Get("destination"))
		return
	}
	c.renderSignup(w, r, r.URL.Query().Get("destination"), map[string]string{}, map[string]string{})
}

// handleSignupPost - the sign-up form submitted: the robot checks
// (honeypot, CAPTCHA), then RegistrationService.Register; an active
// account is logged in.
func (c *UserController) handleSignupPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	destination := r.FormValue("destination")
	form := map[string]string{
		"username": strings.TrimSpace(r.FormValue("username")),
		"email":    strings.ToLower(strings.TrimSpace(r.FormValue("email"))),
	}
	if service.CheckBotTrap(r) != nil {
		c.renderSignup(w, r, destination, form, map[string]string{"form": i18n.T(ctx, "The form was sent too quickly, or looks sent by a program. Wait a few seconds and send it again.")})
		return
	}
	if msg := c.captchaFormError(ctx, r, entity.CaptchaFormRegister); msg != "" {
		c.renderSignup(w, r, destination, form, map[string]string{"form": msg})
		return
	}

	password := r.FormValue("password")
	created, outcome, err := c.registration.Register(ctx, accounts.Signup{
		Username:        form["username"],
		Email:           form["email"],
		Password:        password,
		PasswordConfirm: r.FormValue("password_confirm"),
		Timezone:        userTimezoneFromForm(r.FormValue("timezone"), ""),
		IP:              httputil.ClientIP(r),
		BaseURL:         mailBaseURL(),
		FieldValue:      r.FormValue,
	})
	var invalid accounts.SignupInvalid
	switch {
	case errors.As(err, &invalid):
		c.renderSignup(w, r, destination, form, invalid)
		return
	case errors.Is(err, accounts.ErrRegistrationClosed):
		c.renderSignupClosed(w, r, destination)
		return
	case err != nil:
		c.LogFailure(err, "registering an account")
		c.renderSignup(w, r, destination, form, map[string]string{"form": i18n.T(ctx, "Error creating the account. Please try again later.")})
		return
	}

	if outcome != accounts.SignupActive {
		data := c.GetCommonData(r)
		data["email"] = created.Email
		data["pendingApproval"] = outcome == accounts.SignupPendingApproval
		data["pageSEO"] = PageSEOForList(i18n.T(ctx, "Registration - check your email"), "/user/register")
		c.setPathTrail(data, r.URL.Path, i18n.T(ctx, "Registration"))
		data["servicePage"] = true // the sign-in pages: a card, no sidebar
		c.RenderOrFail(w, r, "user/register_success.html", data)
		return
	}
	if _, err := c.authService.Login(ctx, w, r, created.Email, password); err != nil {
		if errors.Is(err, service.ErrSecondFactorNeeded) {
			http.Redirect(w, r, service.SecondFactorURL(destination), http.StatusFound)
			return
		}
		c.LogFailure(err, "auto-login after signup failed")
		http.Redirect(w, r, "/login?msg=registered", http.StatusFound)
		return
	}
	redirectTo := "/me"
	if dest := service.SafeRedirectPath(destination); dest != "" {
		redirectTo = dest
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

// renderSignup renders the sign-up form: what the visitor typed (form)
// and what to correct (errs, by field; "form" - the whole form's).
func (c *UserController) renderSignup(w http.ResponseWriter, r *http.Request, destination string, form, errs map[string]string) {
	ctx := r.Context()
	data := c.GetCommonData(r)
	data["destination"] = destination
	data["errors"] = errs
	data["form"] = form
	data["recaptcha_site_key"] = captcha.SiteKey(entity.CaptchaFormRegister)
	data["pageSEO"] = PageSEOForList(i18n.T(ctx, "Registration"), r.URL.Path)
	data["title"] = i18n.T(ctx, "Registration")
	c.setPathTrail(data, r.URL.Path, i18n.T(ctx, "Registration"))
	data["servicePage"] = true // the sign-in pages: a card, no sidebar
	c.RenderOrFail(w, r, "user/register.html", c.withSignupFields(r, data))
}

// renderSignupClosed - the page saying accounts are the administrators'
// to create.
func (c *UserController) renderSignupClosed(w http.ResponseWriter, r *http.Request, destination string) {
	data := c.GetCommonData(r)
	data["destination"] = destination
	data["pageSEO"] = PageSEOForList(i18n.T(r.Context(), "Registration unavailable"), "/user/register")
	c.setPathTrail(data, r.URL.Path, i18n.T(r.Context(), "Registration"))
	data["servicePage"] = true // the sign-in pages: a card, no sidebar
	c.RenderOrFail(w, r, "user/register_disabled.html", data)
}

// withSignupFields adds to a sign-up page's data the required profile
// fields with what the visitor submitted (signupFields).
func (c *UserController) withSignupFields(r *http.Request, data map[string]any) map[string]any {
	data["signupFields"] = RequiredFieldInputs(r, c.userFields)
	return data
}

// RequiredFieldInputs - the required profile fields with r's submitted
// values: the inputs of a form creating a user.
func RequiredFieldInputs(r *http.Request, fields *accounts.UserFieldService) []entity.UserFieldValue {
	if fields == nil {
		return nil
	}
	defs, err := fields.RequiredFields(r.Context())
	if err != nil {
		return nil // the form posts without them; reading it reports the error
	}
	out := make([]entity.UserFieldValue, len(defs))
	for i, f := range defs {
		out[i] = entity.UserFieldValue{Field: f, Value: strings.TrimSpace(r.FormValue(accounts.UserFieldFormName(f.ID)))}
	}
	return out
}

// ReadRequiredFields reads the required profile fields of a form creating
// a user: their values, or the message of the one to correct.
func ReadRequiredFields(ctx context.Context, fields *accounts.UserFieldService, get func(string) string) (map[int]string, string, error) {
	if fields == nil {
		return nil, "", nil
	}
	values, err := fields.RequiredFormValues(ctx, get)
	var invalid accounts.UserFieldFormError
	if errors.As(err, &invalid) {
		return nil, invalid.Message(ctx), nil
	}
	return values, "", err
}
