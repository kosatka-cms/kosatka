package admin

import (
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// captchaFormRow - a form checkbox on the settings page.
type captchaFormRow struct {
	Name, Label string
	On          bool
}

// CaptchaPage handles GET/POST /admin/config/captcha: reCAPTCHA keys and
// the forms that show it. Full-access roles only: the page holds the
// secret key, which is never shown back.
func (c *Controller) CaptchaPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := c.GetCurrentUser(r)
	if user == nil || !service.HasGrant(user.Role, service.PermFullAccess) {
		c.RenderForbidden(w, r)
		return
	}
	if c.captchaService == nil {
		httputil.InternalServerError(w)
		return
	}
	saved, err := c.captchaService.Saved(ctx)
	if err != nil {
		c.LogFailure(err, "error loading the CAPTCHA settings")
		httputil.InternalServerError(w)
		return
	}

	if r.Method == http.MethodPost {
		next := entity.CaptchaSettings{Enabled: r.FormValue("enabled") == "1", SiteKey: saved.SiteKey, SecretKey: saved.SecretKey, Forms: map[string]bool{}}
		switch {
		case c.captchaService.KeysFromConfig():
			// The server config's keys win: the saved ones stay as they are.
		case r.FormValue("clear_keys") == "1":
			next.SiteKey, next.SecretKey = "", ""
		default:
			next.SiteKey = strings.TrimSpace(r.FormValue("site_key"))
			// An empty secret field keeps the saved secret (it is never
			// shown on the page).
			if secret := strings.TrimSpace(r.FormValue("secret_key")); secret != "" {
				next.SecretKey = secret
			}
			if (next.SiteKey == "") != (next.SecretKey == "") {
				http.Redirect(w, r, "/admin/config/captcha?error=pair", http.StatusSeeOther)
				return
			}
		}
		for _, f := range entity.CaptchaFormNames() {
			next.Forms[f] = r.FormValue("form_"+f) == "1"
		}
		if err := c.captchaService.Save(ctx, next); err != nil {
			c.LogFailure(err, "error saving the CAPTCHA settings")
			http.Redirect(w, r, "/admin/config/captcha?error=1", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/config/captcha?saved=1", http.StatusSeeOther)
		return
	}

	forms := make([]captchaFormRow, 0, len(entity.CaptchaFormNames()))
	for _, f := range entity.CaptchaFormNames() {
		forms = append(forms, captchaFormRow{Name: f, Label: i18n.T(ctx, captchaFormLabel(f)), On: saved.Forms[f]})
	}
	title := i18n.T(ctx, "Spam protection (reCAPTCHA)")
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["enabled"] = saved.Enabled
	data["siteKey"] = saved.SiteKey
	data["secretSaved"] = saved.SecretKey != ""
	data["keysFromConfig"] = c.captchaService.KeysFromConfig()
	data["forms"] = forms
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error")
	c.RenderOrFail(w, r, "admin/config/captcha.html", data)
}

// captchaFormLabel - a CAPTCHA form as the settings page names it (a
// translation source; the key when unknown).
func captchaFormLabel(key string) string {
	for _, f := range entity.CaptchaForms() {
		if f.Key == key {
			return f.Label
		}
	}
	return key
}
