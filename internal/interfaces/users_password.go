package interfaces

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/panicsafe"
)

// PasswordRequest shows forgot-password form (GET) or sends reset email (POST).
func (c *UserController) PasswordRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/html")

	if r.Method == http.MethodGet {
		data := c.GetCommonData(r)
		data["title"] = i18n.T(r.Context(), "Password recovery")
		data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Password recovery")}
		data["sent"] = r.URL.Query().Get("sent") == "1"
		data["recaptcha_site_key"] = captcha.SiteKey(entity.CaptchaFormPassword)
		c.setPathTrail(data, r.URL.Path, i18n.T(r.Context(), "Password recovery"))
		data["servicePage"] = true // the sign-in pages: a card, no sidebar
		c.RenderOrFail(w, r, "user/password_request.html", data)
		return
	}

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/user/password", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	if msg := c.captchaFormError(ctx, r, entity.CaptchaFormPassword); msg != "" {
		data := c.GetCommonData(r)
		data["title"] = i18n.T(r.Context(), "Password recovery")
		data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Password recovery")}
		data["sent"] = false
		data["recaptcha_error"] = msg
		data["recaptcha_site_key"] = captcha.SiteKey(entity.CaptchaFormPassword)
		c.setPathTrail(data, r.URL.Path, i18n.T(r.Context(), "Password recovery"))
		data["servicePage"] = true // the sign-in pages: a card, no sidebar
		c.RenderOrFail(w, r, "user/password_request.html", data)
		return
	}
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	// Always show success (no user enumeration)
	redirect := "/user/password?sent=1"

	if email != "" {
		// In the background: the answer is as quick whether the address
		// has an account (a mail goes out) or not. Request logs its own
		// failures.
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		go func() {
			defer cancel()
			defer panicsafe.Recover(c.Logger(), "password reset request")
			c.passwordReset.Request(bg, email, mailBaseURL())
		}()
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// ResetPassword shows form to set new password (GET) or saves it (POST).
func (c *UserController) ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/html")
	token := r.URL.Query().Get("token")
	if r.Method == http.MethodGet {
		data := c.GetCommonData(r)
		data["title"] = i18n.T(r.Context(), "New password")
		data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "New password")}
		data["token"] = token
		data["error"] = r.URL.Query().Get("error")
		if token == "" {
			data["error"] = "invalid"
		}
		c.setPathTrail(data, r.URL.Path, i18n.T(r.Context(), "New password"))
		data["servicePage"] = true // the sign-in pages: a card, no sidebar
		c.RenderOrFail(w, r, "user/password_reset.html", data)
		return
	}

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/user/password", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	token = r.FormValue("token")
	switch err := c.passwordReset.Reset(ctx, token, r.FormValue("password"), r.FormValue("password_confirm")); {
	case errors.Is(err, accounts.ErrResetPasswordInvalid):
		http.Redirect(w, r, "/user/reset-password?token="+url.QueryEscape(token)+"&error=mismatch", http.StatusSeeOther)
		return
	case errors.Is(err, accounts.ErrResetLinkInvalid):
		http.Redirect(w, r, "/user/reset-password?error=invalid", http.StatusSeeOther)
		return
	case err != nil:
		c.LogFailure(err, "resetting the password")
		httputil.InternalServerError(w)
		return
	}
	http.Redirect(w, r, "/login?msg=password_reset", http.StatusSeeOther)
}
