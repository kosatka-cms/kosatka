package entity

import "context"

// MailTemplate - an email's subject and body with placeholders
// ([site:name], [user:name], an email's own).
type MailTemplate struct {
	Subject string
	Body    string
}

// MailTemplatesPath - the Mail page's "Emails" tab.
const MailTemplatesPath = "/admin/config/mail/templates"

// MailTemplatePath - the edit page of the email key.
func MailTemplatePath(key string) string { return MailTemplatesPath + "/" + key }

// MailSitePlaceholders - the placeholders every email may use: the
// site's.
var MailSitePlaceholders = []string{"[site:name]", "[site:slogan]", "[site:url-brief]", "[site:login-url]"}

// MailUserPlaceholders - the placeholders of an email about a user (to
// them or of them): their name, address and profile's edit page.
var MailUserPlaceholders = []string{"[user:name]", "[user:email]", "[user:edit-url]"}

// MailDef - one of the emails the site sends, on the Mail page's
// "Emails" tab: the core's (the accounts') and the modules'
// (modulekit.AddMail). It is sent by its Key (MailService.Send): its
// template with the placeholders filled in, unless it is switched off.
type MailDef struct {
	// Key - the email's id: "account.password_reset", "<module>.<name>".
	Key string
	// Module - the module sending it ("" - the core): listed while the
	// module is on.
	Module string
	// Label, Description (when it is sent) and Recipient (who gets it) -
	// translation sources (i18n.N).
	Label       string
	Description string
	Recipient   string
	// Optional - it can be switched off on the Mail page; the others are
	// always sent.
	Optional bool
	// Placeholders - those its text may use (MailSitePlaceholders, the
	// user's, its own).
	Placeholders []string
	// Sample - values of its own placeholders for the page's preview and
	// a test email ("[pm:sender]" → "anna").
	Sample map[string]string
	// Default - its built-in text, translation sources (i18n.N): kept by
	// the mail_templates table when changed. An email keeping its text
	// elsewhere gives Load and Save instead.
	Default MailTemplate
	// Load - its template, a blank subject or body the default's; Save
	// stores one (blank - back to the default). Both nil - Default and
	// the mail_templates table.
	Load func(ctx context.Context) (MailTemplate, error)
	Save func(ctx context.Context, t MailTemplate) error
	// Weight orders it among its group's emails.
	Weight int
}

// StoredMail - what the Mail page changed of an email (mail_templates):
// whether it is sent, its own text (blank - the default).
type StoredMail struct {
	Enabled bool
	MailTemplate
}

// MailTemplateRepository keeps what the Mail page changed of the emails.
type MailTemplateRepository interface {
	// All - the emails changed, by key.
	All(ctx context.Context) (map[string]StoredMail, error)
	// SaveText stores an email's text (blank - the default's).
	SaveText(ctx context.Context, key string, t MailTemplate) error
	// SetEnabled switches an email on or off.
	SetEnabled(ctx context.Context, key string, on bool) error
}
