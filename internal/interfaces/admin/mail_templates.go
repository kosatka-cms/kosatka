package admin

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// mailTemplatesPath - the Mail page's "Emails" tab.
const mailTemplatesPath = entity.MailTemplatesPath

// mailRow - an email on the "Emails" tab: its definition, whether it is
// sent, when it last was (zero - not in the log).
type mailRow struct {
	entity.MailDef
	Enabled  bool
	LastSent time.Time
}

// mailGroup - the emails of the core or of one module.
type mailGroup struct {
	// Label - "" for the core's (the page names the group), else the
	// module's label.
	Label string
	Rows  []mailRow
}

// MailTemplatesPage renders GET /admin/config/mail/templates: the site's
// emails (service.MailDefs), the core's first, then each enabled module's -
// who gets each, whether it is sent (a switch for an optional one), a
// link to its text.
func (c *Controller) MailTemplatesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if c.mailAdmin(w, r) == nil {
		return
	}
	stored, err := c.mailService.Stored(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "listing the emails")
		return
	}
	last := map[string]time.Time{}
	if log := c.mailService.Log(); log != nil {
		if last, err = log.LastSent(ctx); err != nil {
			c.LogFailure(err, "the emails last sent")
		}
	}
	var groups []mailGroup
	for _, def := range service.MailDefs() {
		if def.Module != "" && !service.ModuleEnabled(def.Module) {
			continue
		}
		label := ""
		if m, ok := entity.ModuleByName(def.Module); ok && def.Module != "" {
			label = m.Label
		}
		if len(groups) == 0 || groups[len(groups)-1].Label != label {
			groups = append(groups, mailGroup{Label: label})
		}
		st, ok := stored[def.Key]
		g := &groups[len(groups)-1]
		g.Rows = append(g.Rows, mailRow{MailDef: def, Enabled: !ok || st.Enabled, LastSent: last[def.Key]})
	}
	title := i18n.T(ctx, "Emails")
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["groups"] = groups
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error") == "1"
	c.RenderOrFail(w, r, "admin/config/mail_templates.html", data)
}

// mailDefOf - the email of the request's {key}, or 404 (none, or its
// module off).
func (c *Controller) mailDefOf(w http.ResponseWriter, r *http.Request) (entity.MailDef, bool) {
	def, ok := service.MailDefByKey(r.PathValue("key"))
	if !ok || (def.Module != "" && !service.ModuleEnabled(def.Module)) {
		c.RenderNotFound(w, r)
		return entity.MailDef{}, false
	}
	return def, true
}

// MailTemplateEdit renders GET /admin/config/mail/templates/{key}: the
// email's subject and text, its placeholders, how it looks with sample
// values, a test email.
func (c *Controller) MailTemplateEdit(w http.ResponseWriter, r *http.Request) {
	user := c.mailAdmin(w, r)
	if user == nil {
		return
	}
	def, ok := c.mailDefOf(w, r)
	if !ok {
		return
	}
	c.renderMailTemplate(w, r, user, def, nil, "", false)
}

// MailTemplateSave handles POST /admin/config/mail/templates/{key}: the
// email's subject and text (reset=1 - back to the default's).
func (c *Controller) MailTemplateSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := c.mailAdmin(w, r)
	if user == nil {
		return
	}
	def, ok := c.mailDefOf(w, r)
	if !ok {
		return
	}
	t := entity.MailTemplate{Subject: strings.TrimSpace(r.FormValue("subject")), Body: strings.TrimSpace(r.FormValue("body"))}
	if r.FormValue("reset") == "1" {
		t = entity.MailTemplate{}
	}
	back := entity.MailTemplatePath(def.Key)
	if err := c.mailService.SaveTemplate(ctx, def, t); err != nil {
		c.LogFailure(err, "saving an email's text", "email", def.Key)
		http.Redirect(w, r, back+"?error=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, back+"?saved=1", http.StatusSeeOther)
}

// MailTemplateTest handles POST /admin/config/mail/templates/{key}/test:
// the email with sample values to the address given (the current user's
// by default), switched on or not; the page shows the result.
func (c *Controller) MailTemplateTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := c.mailAdmin(w, r)
	if user == nil {
		return
	}
	def, ok := c.mailDefOf(w, r)
	if !ok {
		return
	}
	to := strings.TrimSpace(r.FormValue("to"))
	testErr := ""
	if _, err := mail.ParseAddress(to); err != nil {
		testErr = i18n.T(ctx, "Enter a valid email address.")
	} else if err := c.mailService.SendSample(ctx, def, to, c.mailSample(r, user, def)); err != nil {
		c.LogFailure(err, "sending a test email", "email", def.Key)
		testErr = err.Error()
		if errors.Is(err, service.ErrMailOff) {
			testErr = i18n.T(ctx, "Email sending is off or not configured.")
		}
	}
	c.renderMailTemplate(w, r, user, def, &to, testErr, testErr == "")
}

// MailTemplateSwitch handles POST /admin/config/mail/templates/{key}/enabled:
// an optional email switched on (on=1) or off.
func (c *Controller) MailTemplateSwitch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if c.mailAdmin(w, r) == nil {
		return
	}
	def, ok := c.mailDefOf(w, r)
	if !ok {
		return
	}
	if !def.Optional {
		c.RenderBadForm(w, r)
		return
	}
	if err := c.mailService.SetMailEnabled(ctx, def.Key, r.FormValue("on") == "1"); err != nil {
		c.LogFailure(err, "switching an email", "email", def.Key)
		http.Redirect(w, r, mailTemplatesPath+"?error=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, mailTemplatesPath+"?saved=1", http.StatusSeeOther)
}

// renderMailTemplate renders the email's page; testTo - the test email's
// address just used (nil - the user's), testError/testSent - its result.
func (c *Controller) renderMailTemplate(w http.ResponseWriter, r *http.Request, user *entity.User, def entity.MailDef, testTo *string, testError string, testSent bool) {
	ctx := r.Context()
	t, err := c.mailService.Template(ctx, def)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading an email's text")
		return
	}
	stored, err := c.mailService.Stored(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading an email's text")
		return
	}
	st, changed := stored[def.Key]
	sample := c.mailSample(r, user, def)
	title := i18n.T(ctx, def.Label)
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["adminTrail"] = []entity.BreadcrumbItem{{URL: mailTemplatesPath, Label: i18n.T(ctx, "Emails")}}
	data["def"] = def
	data["enabled"] = !changed || st.Enabled
	data["template"] = t
	data["preview"] = entity.MailTemplate{
		Subject: mailtemplate.ReplacePlaceholders(t.Subject, sample),
		Body:    mailtemplate.ReplacePlaceholders(t.Body, sample),
	}
	to := user.Email
	if testTo != nil {
		to = *testTo
	}
	data["testTo"] = to
	data["testSent"] = testSent
	data["testError"] = testError
	q := r.URL.Query()
	data["saved"] = q.Get("saved") == "1"
	data["error"] = q.Get("error") == "1"
	c.RenderOrFail(w, r, "admin/config/mail_template.html", data)
}

// mailSample - sample values of def's placeholders: the site's, the
// current user's, sample links, the email's own (def.Sample).
func (c *Controller) mailSample(r *http.Request, user *entity.User, def entity.MailDef) mailtemplate.TokenData {
	site := &entity.SiteInformation{}
	if c.siteInformationRepo != nil {
		if s, err := c.siteInformationRepo.Get(r.Context()); err == nil && s != nil {
			site = s
		} else {
			c.LogFailure(err, "the site's information of an email's sample")
		}
	}
	base := interfaces.MailBaseURL()
	if base == "" {
		base = "https://example.com"
	}
	d := accounts.MailTokenData(base, site, user, "example", "example", "example")
	d.Extra = def.Sample
	return d
}

// mailLogPath - the Mail page's "Log" tab.
const mailLogPath = "/admin/config/mail/log"

// mailLogPerPage - the log's entries on a page.
const mailLogPerPage = 50

// MailLogPage renders GET /admin/config/mail/log: the emails sent or
// tried, newest first, 50 a page - when, to whom, which email and its
// subject, what came of it; filtered by a part of the address (?to=), the
// email (?key=) and the result (?status=).
func (c *Controller) MailLogPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if c.mailAdmin(w, r) == nil {
		return
	}
	q := r.URL.Query()
	f := entity.MailLogFilter{To: strings.TrimSpace(q.Get("to")), Key: q.Get("key"), Status: q.Get("status"), Limit: mailLogPerPage}
	switch f.Status {
	case "", entity.MailStatusSent, entity.MailStatusFailed, entity.MailStatusUnsent:
	default:
		f.Status = ""
	}
	page := c.GetPageNumber(r)
	f.Offset = (page - 1) * mailLogPerPage
	var entries []entity.MailLogEntry
	total := 0
	if log := c.mailService.Log(); log != nil {
		var err error
		if entries, total, err = log.List(ctx, f); err != nil {
			c.RenderLoadFailure(w, r, err, "listing the mail log")
			return
		}
	}
	labels := map[string]string{}
	for _, def := range service.MailDefs() {
		labels[def.Key] = i18n.T(ctx, def.Label)
	}
	title := i18n.T(ctx, "Mail log")
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["entries"] = entries
	data["labels"] = labels
	data["defs"] = service.MailDefs()
	data["filter"] = f
	data["filtered"] = f.To != "" || f.Key != "" || f.Status != ""
	data["keepDays"] = c.mailService.LogKeepDays()
	data["pagination"] = pagination.New(page, total, mailLogPerPage, mailLogPath, q)
	c.RenderOrFail(w, r, "admin/config/mail_log.html", data)
}
