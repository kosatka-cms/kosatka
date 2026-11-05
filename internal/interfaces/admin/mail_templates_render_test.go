package admin

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// renderMailPage renders block of an admin mail page with data.
func renderMailPage(t *testing.T, page string, data map[string]any) string {
	t.Helper()
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	if err := ts.RenderPageBlock(&buf, page, "page_body", data); err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// TestMailTemplatesPageRender - the Mail page's tabs, the "Emails" tab
// active; the emails in groups (the core's, a module's by its label):
// each one's label linked to its page, who gets it, when it was last
// sent (linked to its log; never - a dash), a switch for an optional one
// (on or off), "always" for the others.
func TestMailTemplatesPageRender(t *testing.T) {
	out := renderMailPage(t, "admin/config/mail_templates.html", map[string]any{"groups": []mailGroup{
		{Rows: []mailRow{
			{MailDef: entity.MailDef{Key: "account.password_reset", Label: "Password recovery", Description: "Asked for a link.", Recipient: "to the user"}, Enabled: true,
				LastSent: time.Date(2026, 11, 2, 12, 40, 0, 0, time.UTC)},
			{MailDef: entity.MailDef{Key: "account.activation", Label: "Account activation", Recipient: "to the user", Optional: true}, Enabled: false},
		}},
		{Label: "Forum", Rows: []mailRow{{MailDef: entity.MailDef{Key: "forum.reply", Label: "New reply", Optional: true}, Enabled: true}}},
	}})
	for _, want := range []string{
		`<a class="nav-link" href="/admin/config/mail">Sending</a>`,
		`<a class="nav-link active" aria-current="page" href="/admin/config/mail/templates">Emails</a>`,
		`<h2 class="admin-settings-title">Accounts</h2>`, `<h2 class="admin-settings-title">Forum</h2>`,
		`<a href="/admin/config/mail/templates/account.password_reset" class="admin-table-title">Password recovery</a> <span class="admin-table-sub">Asked for a link.</span>`,
		`<span class="admin-table-muted">always</span>`,
		`<a href="/admin/config/mail/log?key=account.password_reset">`, `<td class="text-nowrap admin-table-muted">-</td>`,
		`<a class="nav-link" href="/admin/config/mail/log">Log</a>`,
		`action="/admin/config/mail/templates/account.activation/enabled"`,
		`name="on" value="1" aria-label="Send &#34;Account activation&#34;"`,
		`name="on" value="1" checked aria-label="Send &#34;New reply&#34;"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}

// TestMailTemplatePageRender - an email's page: its subject and text,
// its placeholders as buttons, how it looks with sample values, the test
// email's address and result; the reset in a form of its own.
func TestMailTemplatePageRender(t *testing.T) {
	out := renderMailPage(t, "admin/config/mail_template.html", map[string]any{
		"def":      entity.MailDef{Key: "account.password_reset", Placeholders: []string{"[user:name]", "[user:reset-password-url]"}},
		"enabled":  true,
		"template": entity.MailTemplate{Subject: "Reset for [user:name]", Body: "Follow [user:reset-password-url]"},
		"preview":  entity.MailTemplate{Subject: "Reset for anna", Body: "Follow https://example.com/user/reset-password?token=example"},
		"testTo":   "admin@example.com",
		"testSent": true,
	})
	for _, want := range []string{
		`<a class="nav-link active" aria-current="page" href="/admin/config/mail/templates">Emails</a>`,
		`<div class="alert alert-success" role="status">The test email was sent to admin@example.com.</div>`,
		`name="subject" class="form-control" value="Reset for [user:name]" maxlength="512" data-token-target`,
		`data-token-target>Follow [user:reset-password-url]</textarea>`,
		`<button type="button" class="admin-mail-token" data-insert-token="[user:reset-password-url]">[user:reset-password-url]</button>`,
		`<div class="admin-mail-preview-subject">Reset for anna</div>`,
		`<button type="submit" form="mail-template-reset" class="btn btn-outline-secondary">Default text</button>`,
		`id="mail-template-reset" data-confirm="Restore the default text?" hidden> <input type="hidden" name="reset" value="1">`,
		`<input type="email" id="mail-test-to" name="to" class="form-control" value="admin@example.com" required>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}

// TestMailLogPageRender - the Log tab: the filters as chosen, an entry
// each - when, to whom, the email's label (a message of no template
// said so) and subject, sent, failed with its error, not sent; how long
// they are kept; with no log, a note.
func TestMailLogPageRender(t *testing.T) {
	at := time.Date(2026, 11, 2, 12, 40, 0, 0, time.UTC)
	data := map[string]any{
		"entries": []entity.MailLogEntry{
			{SentAt: at, Key: "account.password_reset", To: "anna@example.com", Subject: "Reset for anna", Status: entity.MailStatusSent},
			{SentAt: at, To: "site@example.com", Subject: "A form", Status: entity.MailStatusFailed, Error: "550 mailbox unavailable"},
			{SentAt: at, Key: "account.password_reset", To: "bob@example.com", Subject: "Reset for bob", Status: entity.MailStatusUnsent},
		},
		"labels":     map[string]string{"account.password_reset": "Password recovery"},
		"defs":       []entity.MailDef{{Key: "account.password_reset", Label: "Password recovery"}},
		"filter":     entity.MailLogFilter{To: "example", Key: "account.password_reset", Status: "failed"},
		"filtered":   true,
		"keepDays":   30,
		"pagination": pagination.New(1, 3, mailLogPerPage, mailLogPath, url.Values{}),
	}
	out := renderMailPage(t, "admin/config/mail_log.html", data)
	for _, want := range []string{
		`<a class="nav-link active" aria-current="page" href="/admin/config/mail/log">Log</a>`,
		`name="to" value="example"`, `<option value="account.password_reset" selected>Password recovery</option>`, `<option value="failed" selected>Failed</option>`,
		`<td class="font-monospace small">anna@example.com</td> <td> Password recovery <span class="admin-table-sub">Reset for anna</span>`,
		`<span class="admin-pill admin-pill--ok">Sent</span>`,
		`<span class="admin-table-muted">A message of no template</span>`,
		`<span class="admin-pill admin-pill--danger">Failed</span><span class="admin-table-sub font-monospace">550 mailbox unavailable</span>`,
		`<span class="admin-pill">Not sent</span>`,
		`Kept for 30 days.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	data["keepDays"], data["entries"], data["filtered"] = 0, nil, false
	if out := renderMailPage(t, "admin/config/mail_log.html", data); !strings.Contains(out, "The log is off") || !strings.Contains(out, "No emails yet.") {
		t.Errorf("no log: %s", out)
	}
}
