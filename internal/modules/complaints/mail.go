package complaints

import (
	"slices"

	"github.com/kosatka-cms/kosatka/internal/modules/complaints/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// MailNewComplaint - the e-mail of a complaint new to the queue, to the
// site's address (modulekit.AddMail).
const MailNewComplaint = manifest.Name + ".new_complaint"

// mailPlaceholders - the complaint's placeholders of the e-mail.
var mailPlaceholders = []string{"[complaint:about]", "[complaint:reason]", "[complaint:note]", "[complaint:url]", "[complaint:queue-url]"}

// mailDef - the new complaint e-mail on the Mail page, its text kept
// there.
func mailDef() modulekit.MailDef {
	return modulekit.MailDef{
		Key:          MailNewComplaint,
		Module:       manifest.Name,
		Label:        i18n.N("New complaint"),
		Description:  i18n.N("A visitor reported a content item or a comment nobody had reported yet (or again, after a decision)."),
		Recipient:    i18n.N("to the site's email"),
		Optional:     true,
		Placeholders: slices.Concat(modulekit.MailSitePlaceholders, mailPlaceholders),
		Sample: map[string]string{
			"[complaint:about]": "a comment by anna on \"How themes work\"", "[complaint:reason]": "Spam or advertising",
			"[complaint:note]": "A link to a casino", "[complaint:url]": "https://example.com/articles/how-themes-work#comment-12",
			"[complaint:queue-url]": "https://example.com" + manifest.AdminQueuePath,
		},
		Default: modulekit.MailTemplate{
			Subject: i18n.N("[[site:name]] A complaint: [complaint:about]"),
			Body:    i18n.N("A visitor complained about [complaint:about].\n\nReason: [complaint:reason]\nNote: [complaint:note]\n\nThe page: [complaint:url]\nThe complaints: [complaint:queue-url]"),
		},
	}
}
