package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/mailsender"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
)

// stubMailTemplates - the mail_templates rows in memory.
type stubMailTemplates map[string]entity.StoredMail

func (r stubMailTemplates) All(context.Context) (map[string]entity.StoredMail, error) { return r, nil }

func (r stubMailTemplates) SaveText(_ context.Context, key string, t entity.MailTemplate) error {
	st, ok := r[key]
	if !ok {
		st.Enabled = true
	}
	st.MailTemplate = t
	r[key] = st
	return nil
}

func (r stubMailTemplates) SetEnabled(_ context.Context, key string, on bool) error {
	st := r[key]
	st.Enabled = on
	r[key] = st
	return nil
}

// textSender records the messages it sends.
type textSender struct{ sent []string }

func (s *textSender) SendPlain(to, subject, body string) error {
	s.sent = append(s.sent, to+"|"+subject+"|"+body)
	return nil
}

// useMailDefs registers defs for a test and forgets them after it.
func useMailDefs(t *testing.T, defs ...entity.MailDef) {
	t.Helper()
	ResetMailDefs()
	t.Cleanup(ResetMailDefs)
	for _, d := range defs {
		RegisterMail(d)
	}
}

// TestMailDefsRegistry - the emails, the core's first, then by module,
// each group by weight; a key added again replaces the earlier.
func TestMailDefsRegistry(t *testing.T) {
	useMailDefs(t,
		entity.MailDef{Key: "polls.closed", Module: "polls"},
		entity.MailDef{Key: "account.b", Weight: 2},
		entity.MailDef{Key: "forum.reply", Module: "forum"},
		entity.MailDef{Key: "account.a", Weight: 1, Label: "old"},
	)
	RegisterMail(entity.MailDef{Key: "account.a", Weight: 1, Label: "new"})
	var keys []string
	for _, d := range MailDefs() {
		keys = append(keys, d.Key)
	}
	if got := strings.Join(keys, " "); got != "account.a account.b forum.reply polls.closed" {
		t.Errorf("order: %s", got)
	}
	if d, ok := MailDefByKey("account.a"); !ok || d.Label != "new" {
		t.Errorf("replaced: %+v %v", d, ok)
	}
	if _, ok := MailDefByKey("none"); ok {
		t.Error("an unknown key found")
	}
}

// TestMailServiceSend - an email by its key: its text (the stored one,
// a blank subject the default's; an email's own Load) with the
// placeholders filled in; an optional one switched off isn't sent, a
// required one is whatever its row says; the test email ignores the
// switch; an unknown key is an error.
func TestMailServiceSend(t *testing.T) {
	ctx := context.Background()
	rows := stubMailTemplates{
		"mod.optional": {Enabled: false},
		"mod.required": {Enabled: false, MailTemplate: entity.MailTemplate{Body: "Saved for [user:name]"}},
	}
	own := entity.MailTemplate{Subject: "Own [site:name]", Body: "Own body"}
	var saved entity.MailTemplate
	useMailDefs(t,
		entity.MailDef{Key: "mod.optional", Optional: true, Default: entity.MailTemplate{Subject: "Opt", Body: "Opt body"}},
		entity.MailDef{Key: "mod.required", Default: entity.MailTemplate{Subject: "Hello [user:name]", Body: "Default"}},
		entity.MailDef{Key: "mod.own", Optional: true,
			Load: func(context.Context) (entity.MailTemplate, error) { return own, nil },
			Save: func(_ context.Context, t entity.MailTemplate) error { saved = t; return nil }},
	)
	sender := &textSender{}
	svc, _ := newTestMailService(&stubMailRepo{s: entity.MailSettings{Enabled: true, Transport: entity.MailTransportResend, FromAddress: "a@example.com", ResendAPIKey: "re"}}, entity.MailSettings{})
	svc.newSender = func(entity.MailSettings) mailsender.EmailSender { return sender }
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	svc.SetTemplates(rows)
	data := mailtemplate.TokenData{UserName: "anna", SiteName: "Example"}

	for _, key := range []string{"mod.optional", "mod.required", "mod.own"} {
		if err := svc.Send(ctx, key, "anna@example.com", data); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	want := "anna@example.com|Hello anna|Saved for anna anna@example.com|Own Example|Own body"
	if got := strings.Join(sender.sent, " "); got != want {
		t.Errorf("sent %q, want %q", got, want)
	}
	if err := svc.Send(ctx, "none", "anna@example.com", data); err == nil {
		t.Error("an unknown email sent")
	}

	def, _ := MailDefByKey("mod.optional")
	if err := svc.SendSample(ctx, def, "admin@example.com", data); err != nil || !strings.HasPrefix(sender.sent[len(sender.sent)-1], "admin@example.com|Opt|") {
		t.Errorf("the test of a switched-off email: %v %v", err, sender.sent)
	}

	// Saving: the email's own Save, else the row; blank - the default.
	ownDef, _ := MailDefByKey("mod.own")
	if err := svc.SaveTemplate(ctx, ownDef, entity.MailTemplate{Subject: "S"}); err != nil || saved.Subject != "S" {
		t.Errorf("own save: %v %+v", err, saved)
	}
	if err := svc.SaveTemplate(ctx, def, entity.MailTemplate{Subject: "Mine"}); err != nil || rows["mod.optional"].Subject != "Mine" {
		t.Errorf("row save: %v %+v", err, rows["mod.optional"])
	}
	if err := svc.SetMailEnabled(ctx, "mod.optional", true); err != nil || !rows["mod.optional"].Enabled {
		t.Errorf("switched on: %v %+v", err, rows["mod.optional"])
	}
	if got, _ := svc.Template(ctx, def); got.Subject != "Mine" || got.Body != "Opt body" {
		t.Errorf("the text in effect: %+v", got)
	}
}

// TestMailServiceSendMailOff - with email off nothing is sent and Send
// says nothing; the test email says ErrMailOff.
func TestMailServiceSendMailOff(t *testing.T) {
	ctx := context.Background()
	useMailDefs(t, entity.MailDef{Key: "mod.x", Default: entity.MailTemplate{Subject: "S", Body: "B"}})
	svc, built := newTestMailService(&stubMailRepo{}, entity.MailSettings{})
	if err := svc.Send(ctx, "mod.x", "a@example.com", mailtemplate.TokenData{}); err != nil || len(*built) != 0 {
		t.Errorf("email off: %v, senders %d", err, len(*built))
	}
	def, _ := MailDefByKey("mod.x")
	if err := svc.SendSample(ctx, def, "a@example.com", mailtemplate.TokenData{}); !errors.Is(err, ErrMailOff) {
		t.Errorf("the test with email off: %v", err)
	}
}

// stubMailLog - the mail log in memory.
type stubMailLog struct {
	entries []entity.MailLogEntry
	before  []time.Time
}

func (l *stubMailLog) Add(_ context.Context, e entity.MailLogEntry) error {
	l.entries = append(l.entries, e)
	return nil
}

func (l *stubMailLog) List(context.Context, entity.MailLogFilter) ([]entity.MailLogEntry, int, error) {
	return l.entries, len(l.entries), nil
}

func (l *stubMailLog) LastSent(context.Context) (map[string]time.Time, error) { return nil, nil }

func (l *stubMailLog) DeleteBefore(_ context.Context, t time.Time) error {
	l.before = append(l.before, t)
	return nil
}

// failingSender refuses every message.
type failingSender struct{}

func (failingSender) SendPlain(string, string, string) error {
	return errors.New("550 mailbox unavailable")
}

// TestMailLog - every message is recorded: the site's email by its key,
// a plain one with none; sent, refused (with the error) or not sent with
// email off; nothing while the settings keep no log. The purge deletes
// what is older than the days kept, all of it with no log.
func TestMailLog(t *testing.T) {
	ctx := context.Background()
	useMailDefs(t, entity.MailDef{Key: "mod.x", Default: entity.MailTemplate{Subject: "Hi [user:name]", Body: "B"}})
	repo := &stubMailRepo{s: entity.MailSettings{Enabled: true, Transport: entity.MailTransportResend, FromAddress: "a@example.com", ResendAPIKey: "re", LogKeepDays: 30}}
	svc, _ := newTestMailService(repo, entity.MailSettings{})
	log := &stubMailLog{}
	svc.SetLog(log)
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	data := mailtemplate.TokenData{UserName: "anna"}
	if err := svc.Send(ctx, "mod.x", "anna@example.com", data); err != nil {
		t.Fatal(err)
	}
	if err := svc.SendPlain("site@example.com", "A form", "Body"); err != nil {
		t.Fatal(err)
	}
	svc.newSender = func(entity.MailSettings) mailsender.EmailSender { return failingSender{} }
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.Send(ctx, "mod.x", "bob@example.com", data); err == nil {
		t.Error("a refused email: no error")
	}
	repo.s.Enabled = false
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.Send(ctx, "mod.x", "cat@example.com", data); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range log.entries {
		got = append(got, e.Key+"|"+e.To+"|"+e.Subject+"|"+e.Status+"|"+e.Error)
	}
	want := "mod.x|anna@example.com|Hi anna|sent| |site@example.com|A form|sent| mod.x|bob@example.com|Hi anna|failed|550 mailbox unavailable mod.x|cat@example.com|Hi anna|unsent|"
	if strings.Join(got, " ") != want {
		t.Errorf("logged:\n%s\nwant\n%s", strings.Join(got, " "), want)
	}

	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	job := svc.LogPurgeJob(func() time.Time { return now })
	if err := job.Run(ctx); err != nil || !log.before[0].Equal(now.AddDate(0, 0, -30)) {
		t.Errorf("purged before %v: %v", log.before, err)
	}

	repo.s.LogKeepDays = 0
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.Send(ctx, "mod.x", "dan@example.com", data); err != nil || len(log.entries) != 4 {
		t.Errorf("no log: %v, %d entries", err, len(log.entries))
	}
	if err := job.Run(ctx); err != nil || !log.before[1].Equal(now) {
		t.Errorf("no log purges all: %v %v", log.before, err)
	}
}
