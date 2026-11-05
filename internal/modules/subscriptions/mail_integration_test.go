//go:build dbintegration

package subscriptions

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
)

type fakeSite struct {
	modulekit.SiteInformationRepository
}

func (fakeSite) Get(context.Context) (*modulekit.SiteInformation, error) {
	return &modulekit.SiteInformation{SiteName: "Example", SiteEmail: "admin@example.com"}, nil
}

// fakeSender - the site's mail: the module's e-mail in its default text.
type fakeSender struct{ to, subjects, bodies []string }

func (f *fakeSender) Send(_ context.Context, key, to string, data mailtemplate.TokenData) error {
	if key != MailNotifications {
		return fmt.Errorf("e-mail %s", key)
	}
	tpl := defaultMailTemplate()
	f.to = append(f.to, to)
	f.subjects = append(f.subjects, mailtemplate.ReplacePlaceholders(tpl.Subject, data))
	f.bodies = append(f.bodies, mailtemplate.ReplacePlaceholders(tpl.Body, data))
	return nil
}

// TestMailer - the e-mails of the notifications: a user mailed at once
// gets one e-mail of what is waiting (an item each: its link through the
// notification, the unsubscribe link of the subscription it came from),
// one with the daily digest gets it once a day from DigestHour, one who
// turned the e-mails off none (theirs no longer wait); without the site's
// URL nothing is sent.
func TestMailer(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		f := newFixture(t, db)
		deps := testkit.Deps(db)
		repo := NewRepo(db.DB)
		comment := insertRow(t, db, "INSERT INTO comments (content, nid, created_by) VALUES ('<p>Fair winds</p>', ?, ?)", f.harbour, f.ann)
		for user, mode := range map[int]string{f.bob: MailNow, f.cat: MailDaily} {
			if err := repo.SaveUserSettings(ctx, user, UserSettings{Mail: mode, Replies: true}); err != nil {
				t.Fatal(err)
			}
		}
		for _, target := range []Target{{NodeID: f.harbour}, {TermID: f.sailing}} {
			if err := repo.Subscribe(ctx, f.bob, target); err != nil {
				t.Fatal(err)
			}
		}
		now := time.Now()
		if err := repo.AddNotifications(ctx, []Notification{
			{UserID: f.bob, Kind: KindComment, NodeID: f.harbour, CommentID: comment, ActorID: f.ann, CreatedAt: now},
			{UserID: f.bob, Kind: KindContent, NodeID: f.harbour, ActorID: f.ann, TermID: f.sailing, CreatedAt: now},
			{UserID: f.cat, Kind: KindReply, NodeID: f.harbour, CommentID: comment, ActorID: f.ann, CreatedAt: now},
		}, map[int]bool{f.bob: true, f.cat: true}); err != nil {
			t.Fatal(err)
		}

		mail := &fakeSender{}
		base := ""
		m := NewMailer(MailerDeps{Repo: repo, Describe: describeDeps{users: deps.Users, nodes: deps.NodeRepo, comments: deps.Comments, terms: deps.Taxonomy},
			Users: deps.Users, Site: fakeSite{}, Mail: mail, BaseURL: func() string { return base }, Logger: testkit.Logger()})
		if err := m.SendNow(ctx); err != nil || len(mail.to) != 0 {
			t.Fatalf("without the site's URL: %v %v", mail.to, err)
		}

		base = "https://example.com/"
		if err := m.SendNow(ctx); err != nil {
			t.Fatal(err)
		}
		if len(mail.to) != 1 || mail.to[0] != "bob@example.com" || !strings.Contains(mail.subjects[0], "2 new notifications") {
			t.Fatalf("at once: to %v, subjects %v", mail.to, mail.subjects)
		}
		subs, _ := repo.UserSubscriptions(ctx, f.bob)
		ids, _, _ := repo.Notifications(ctx, f.bob, 0, 10)
		body := mail.bodies[0]
		for _, want := range []string{
			"ann commented on \"Harbour\":", "\"Fair winds\"", "New content with \"Sailing\": \"Harbour\"",
			fmt.Sprintf("https://example.com/notifications/%d", ids[0].ID), fmt.Sprintf("https://example.com/notifications/%d", ids[1].ID),
			"https://example.com/subscriptions/unsubscribe/" + subs[0].Token, "https://example.com/subscriptions/unsubscribe/" + subs[1].Token,
			"https://example.com/me/notifications",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("the e-mail has no %q:\n%s", want, body)
			}
		}
		if err := m.SendNow(ctx); err != nil || len(mail.to) != 1 {
			t.Errorf("sent again: %v %v", mail.to, err)
		}

		day := time.Date(2026, 5, 4, DigestHour-1, 30, 0, 0, time.UTC)
		m.now = func() time.Time { return day }
		if err := m.SendDigests(ctx); err != nil || len(mail.to) != 1 {
			t.Fatalf("the digest before %d o'clock: %v %v", DigestHour, mail.to, err)
		}
		day = day.Add(time.Hour)
		for range 2 {
			if err := m.SendDigests(ctx); err != nil {
				t.Fatal(err)
			}
		}
		if len(mail.to) != 2 || mail.to[1] != "cat@example.com" || !strings.Contains(mail.subjects[1], "A reply to your comment - Harbour") {
			t.Errorf("the digest: to %v, subjects %v", mail.to, mail.subjects)
		}

		// Turned off: what waits is dropped.
		if err := repo.AddNotifications(ctx, []Notification{{UserID: f.cat, Kind: KindComment, NodeID: f.harbour, ActorID: f.ann, CreatedAt: now}},
			map[int]bool{f.cat: true}); err != nil {
			t.Fatal(err)
		}
		if err := repo.SaveUserSettings(ctx, f.cat, UserSettings{Mail: MailNever}); err != nil {
			t.Fatal(err)
		}
		if err := m.SendNow(ctx); err != nil {
			t.Fatal(err)
		}
		if waiting, _ := repo.WaitingMail(ctx); len(waiting) != 0 || len(mail.to) != 2 {
			t.Errorf("e-mails off: waiting %v, sent to %v", waiting, mail.to)
		}
	})
}
