package errorreports

import (
	"slices"

	"github.com/kosatka-cms/kosatka/internal/modules/error_reports/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// MailNewReport - the e-mail of a new report to the site's address
// (modulekit.AddMail).
const MailNewReport = manifest.Name + ".new_report"

// mailPlaceholders - the report's placeholders of the e-mail.
var mailPlaceholders = []string{"[report:id]", "[report:date]", "[report:author]", "[report:page]",
	"[report:text]", "[report:comment]", "[report:ip]", "[report:admin-url]"}

// mailDef - the new report e-mail on the Mail page, its text kept there.
func mailDef() modulekit.MailDef {
	return modulekit.MailDef{
		Key:          MailNewReport,
		Module:       manifest.Name,
		Label:        i18n.N("New error report"),
		Description:  i18n.N("A visitor reported a mistake in a page's text."),
		Recipient:    i18n.N("to the site's email"),
		Optional:     true,
		Placeholders: slices.Concat(modulekit.MailSitePlaceholders, mailPlaceholders),
		Sample: map[string]string{
			"[report:id]": "42", "[report:date]": "2026-11-02 12:40", "[report:author]": "anna",
			"[report:page]": "https://example.com/articles/how-themes-work", "[report:text]": "the words selected",
			"[report:comment]": "a typo", "[report:ip]": "192.0.2.10", "[report:admin-url]": "https://example.com" + adminListURL,
		},
		Default: modulekit.MailTemplate{
			Subject: i18n.N("[[site:name]] Error report #[report:id]"),
			Body:    i18n.N("New error report on the site [site:name]\n\nID: [report:id]\nDate: [report:date]\nAuthor: [report:author]\nPage: [report:page]\n\nSelected text:\n[report:text]\n\nComment:\n[report:comment]\n\nIP: [report:ip]\n\nList in the admin area: [report:admin-url]"),
		},
	}
}
