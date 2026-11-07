package admin

import (
	"context"
	"errors"
	"maps"
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
// email, the transport (SMTP, or a module's: service.MailTransport) and
// the sender address - not
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
		next, code := mailSettingsFromForm(r, *saved, c.mailService.Status().Source == service.MailSourceConfig, c.mailService.EnvSetting)
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
	data["smtpPasswordSaved"] = saved.SMTPPassword != ""
	// None saved: the transport the server's environment sets, else no
	// choice (SMTP isn't assumed: the switch and the log save without one).
	transport := saved.Transport
	if transport == "" {
		transport = c.mailService.EnvTransport()
	}
	data["transport"] = transport
	data["transports"] = mailTransportSections(ctx, *saved, c.mailService.EnvSetting)
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
// sets the transport: the form changes the switch and the log only;
// envSet - whether the server's environment sets a transport's setting
// (a required one is then filled). No transport chosen keeps the saved
// one, or none: the switch, the log and the sender save without one.
func mailSettingsFromForm(r *http.Request, saved entity.MailSettings, fromConfig bool, envSet func(transport, field string) bool) (entity.MailSettings, string) {
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
	if t := r.FormValue("transport"); t != "" {
		next.Transport = t
	}
	if _, ok := availableMailTransport(next.Transport); !ok && next.Transport != entity.MailTransportSMTP && next.Transport != "" {
		return saved, "transport"
	}
	next.FromAddress = strings.TrimSpace(r.FormValue("from_address"))
	if next.FromAddress != "" {
		if _, err := mail.ParseAddress(next.FromAddress); err != nil {
			return saved, "from"
		}
	}
	// Empty secret fields keep the saved secrets (they are never shown).
	next.TransportSettings = mailTransportSettingsFromForm(r, saved.TransportSettings)
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
	switch {
	case next.Transport == "":
		return next, ""
	case next.FromAddress == "":
		return saved, "from"
	case next.Transport == entity.MailTransportSMTP:
		if next.SMTPHost == "" || next.SMTPPort == 0 {
			return saved, "smtp"
		}
	default:
		if t, _ := availableMailTransport(next.Transport); !mailTransportFilled(t, next.TransportSettings[t.Name], envSet) {
			return saved, "required"
		}
	}
	return next, ""
}

// mailTransportField - a setting of a module's way of sending on the
// Mail page: its form name and input, the saved value (not a secret's),
// whether a secret is saved.
type mailTransportField struct {
	Input, Label, Help string
	Secret, Required   bool
	Value              string
	Saved              bool
	// Env - the environment variable that sets it on this server: the
	// page shows that instead of an input.
	Env string
}

// mailTransportSection - a module's way of sending on the Mail page.
type mailTransportSection struct {
	Name, Title string
	Fields      []mailTransportField
}

// availableMailTransport - the module's way of sending named name if its
// module is enabled.
func availableMailTransport(name string) (service.MailTransport, bool) {
	t, ok := service.MailTransportByName(name)
	if !ok || !service.ModuleEnabled(t.Module) {
		return service.MailTransport{}, false
	}
	return t, true
}

// availableMailTransports - the modules' ways of sending whose modules
// are enabled.
func availableMailTransports() []service.MailTransport {
	var out []service.MailTransport
	for _, t := range service.MailTransports() {
		if service.ModuleEnabled(t.Module) {
			out = append(out, t)
		}
	}
	return out
}

// mailTransportInput - the form name of the transport's setting (the
// names have no dots).
func mailTransportInput(transport, field string) string {
	return "ts." + transport + "." + field
}

// mailTransportSections - the Mail page's sections of the available
// modules' ways of sending, with the saved settings (no secret shown);
// envSet - whether the server's environment sets a setting.
func mailTransportSections(ctx context.Context, saved entity.MailSettings, envSet func(transport, field string) bool) []mailTransportSection {
	var out []mailTransportSection
	for _, t := range availableMailTransports() {
		sec := mailTransportSection{Name: t.Name, Title: t.Title}
		for _, f := range t.Fields {
			value := saved.TransportSetting(t.Name, f.Name)
			field := mailTransportField{
				Input: mailTransportInput(t.Name, f.Name), Label: i18n.T(ctx, f.Label),
				Secret: f.Secret, Required: f.Required, Saved: f.Secret && value != "",
			}
			if f.Help != "" {
				field.Help = i18n.T(ctx, f.Help)
			}
			if !f.Secret {
				field.Value = value
			}
			if f.Env != "" && envSet != nil && envSet(t.Name, f.Name) {
				field.Env = f.Env
			}
			sec.Fields = append(sec.Fields, field)
		}
		out = append(out, sec)
	}
	return out
}

// mailTransportSettingsFromForm - the modules' ways of sending's
// settings the form submits on top of the saved ones: an empty secret
// keeps the saved one; a transport not on the page (its module off, or
// not in the binary) keeps its settings.
func mailTransportSettingsFromForm(r *http.Request, saved map[string]map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string, len(saved))
	for transport, values := range saved {
		out[transport] = maps.Clone(values)
	}
	for _, t := range availableMailTransports() {
		values := out[t.Name]
		if values == nil {
			values = map[string]string{}
		}
		for _, f := range t.Fields {
			v := r.FormValue(mailTransportInput(t.Name, f.Name))
			if f.Secret {
				if v = strings.TrimSpace(v); v != "" {
					values[f.Name] = v
				}
				continue
			}
			values[f.Name] = strings.TrimSpace(v)
		}
		out[t.Name] = values
	}
	return out
}

// mailTransportFilled - whether a module's way of sending has its
// required settings: in values, or set by the server's environment
// (envSet).
func mailTransportFilled(t service.MailTransport, values map[string]string, envSet func(transport, field string) bool) bool {
	if t.Name == "" {
		return false
	}
	for _, f := range t.Fields {
		if f.Required && strings.TrimSpace(values[f.Name]) == "" && (envSet == nil || !envSet(t.Name, f.Name)) {
			return false
		}
	}
	return true
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
