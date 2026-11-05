package admin

import (
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// mailAdmin - the current user when they have full access; otherwise it
// answers 403 (the page holds the transport's secrets).
func (c *Controller) mailAdmin(w http.ResponseWriter, r *http.Request) *entity.User {
	user, _ := c.GetCurrentUser(r)
	if user == nil || !service.HasGrant(user.Role, service.PermFullAccess) {
		c.RenderForbidden(w, r)
		return nil
	}
	if c.mailService == nil {
		httputil.InternalServerError(w)
		return nil
	}
	return user
}

// MailPage handles GET/POST /admin/config/mail: whether the site sends
// email, the transport (Resend or SMTP) and the sender address - not
// while the server config sets the transport (it wins). Secrets are
// never shown back; an empty field keeps the saved one.
func (c *Controller) MailPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := c.mailAdmin(w, r)
	if user == nil {
		return
	}
	saved, err := c.mailService.Saved(ctx)
	if err != nil {
		c.LogFailure(err, "error loading the mail settings")
		httputil.InternalServerError(w)
		return
	}

	if r.Method == http.MethodPost {
		next, code := mailSettingsFromForm(r, *saved, c.mailService.Status().Source == service.MailSourceConfig)
		if code != "" {
			http.Redirect(w, r, "/admin/config/mail?error="+code, http.StatusSeeOther)
			return
		}
		if err := c.mailService.Save(ctx, next); err != nil {
			c.LogFailure(err, "error saving the mail settings")
			http.Redirect(w, r, "/admin/config/mail?error=1", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/config/mail?saved=1", http.StatusSeeOther)
		return
	}

	c.renderMailPage(w, r, user, saved, "", false)
}

// renderMailPage renders the settings page; testError/testSent - the
// result of a test email just sent (rendered, not passed in the URL).
func (c *Controller) renderMailPage(w http.ResponseWriter, r *http.Request, user *entity.User, saved *entity.MailSettings, testError string, testSent bool) {
	ctx := r.Context()
	title := i18n.T(ctx, "Mail")
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["settings"] = saved
	data["resendKeySaved"] = saved.ResendAPIKey != ""
	data["smtpPasswordSaved"] = saved.SMTPPassword != ""
	transport := saved.Transport
	if transport == "" {
		transport = entity.MailTransportResend
	}
	data["transport"] = transport
	data["logKeepOptions"] = settingOptions(mailLogKeepChoices, saved.LogKeepDays, func(v int) string {
		if v == 0 {
			return i18n.T(ctx, "don't keep a log")
		}
		return i18n.Plural(ctx, v, "@count day", "@count days")
	})
	data["status"] = c.mailService.Status()
	data["fromConfig"] = c.mailService.Status().Source == service.MailSourceConfig
	data["adminEmail"] = user.Email
	q := r.URL.Query()
	data["saved"] = q.Get("saved") == "1"
	data["error"] = q.Get("error")
	data["testSent"] = testSent
	data["testError"] = testError
	c.RenderOrFail(w, r, "admin/config/mail.html", data)
}

// mailLogKeepChoices - how many days the mail log keeps an email (0 - no
// log), the Sending tab's choices.
var mailLogKeepChoices = []int{7, 30, 90, 365, 0}

// mailSettingsFromForm - the settings the form submits on top of the saved
// ones, or the error code for the page; fromConfig - the server config
// sets the transport: the form changes the switch and the log only.
func mailSettingsFromForm(r *http.Request, saved entity.MailSettings, fromConfig bool) (entity.MailSettings, string) {
	next := saved
	next.Enabled = r.FormValue("enabled") == "1"
	if r.Form.Has("log_keep_days") {
		keep, ok := formChoice(r, "log_keep_days", mailLogKeepChoices)
		if !ok {
			return saved, "log"
		}
		next.LogKeepDays = keep
	}
	if fromConfig {
		return next, ""
	}
	if r.FormValue("clear") == "1" {
		// Back to the server config's settings (the log's as chosen).
		return entity.MailSettings{Enabled: next.Enabled, LogKeepDays: next.LogKeepDays}, ""
	}
	next.Transport = r.FormValue("transport")
	if next.Transport != entity.MailTransportResend && next.Transport != entity.MailTransportSMTP {
		return saved, "transport"
	}
	next.FromAddress = strings.TrimSpace(r.FormValue("from_address"))
	if next.FromAddress != "" {
		if _, err := mail.ParseAddress(next.FromAddress); err != nil {
			return saved, "from"
		}
	}
	// Empty secret fields keep the saved secrets (they are never shown).
	if key := strings.TrimSpace(r.FormValue("resend_api_key")); key != "" {
		next.ResendAPIKey = key
	}
	next.SMTPHost = strings.TrimSpace(r.FormValue("smtp_host"))
	next.SMTPPort = 0
	if p := strings.TrimSpace(r.FormValue("smtp_port")); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil || port < 1 || port > 65535 {
			return saved, "port"
		}
		next.SMTPPort = port
	}
	next.SMTPUser = strings.TrimSpace(r.FormValue("smtp_user"))
	if pw := r.FormValue("smtp_password"); pw != "" {
		next.SMTPPassword = pw
	}
	if next.FromAddress == "" {
		return saved, "from"
	}
	switch next.Transport {
	case entity.MailTransportResend:
		if next.ResendAPIKey == "" {
			return saved, "resend"
		}
	case entity.MailTransportSMTP:
		if next.SMTPHost == "" || next.SMTPPort == 0 {
			return saved, "smtp"
		}
	}
	return next, ""
}

// MailTest handles POST /admin/config/mail/test: a test email to the
// current admin through the settings in effect; the page shows the result.
func (c *Controller) MailTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := c.mailAdmin(w, r)
	if user == nil {
		return
	}
	saved, err := c.mailService.Saved(ctx)
	if err != nil {
		c.LogFailure(err, "error loading the mail settings")
		httputil.InternalServerError(w)
		return
	}
	testErr := ""
	switch {
	case strings.TrimSpace(user.Email) == "":
		testErr = i18n.T(ctx, "Your account has no email address.")
	default:
		subject := i18n.T(ctx, "Test email")
		body := i18n.T(ctx, "This is a test email from the site's mail settings. If you got it, email works.")
		if err := c.mailService.SendTest(user.Email, subject, body); err != nil {
			c.logger.Warn("test email failed", "error", err)
			testErr = err.Error()
			if errors.Is(err, service.ErrMailOff) {
				testErr = i18n.T(ctx, "Email sending is off or not configured.")
			}
		}
	}
	c.renderMailPage(w, r, user, saved, testErr, testErr == "")
}
