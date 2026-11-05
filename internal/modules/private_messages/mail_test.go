package privatemessages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
)

type fakeMailThreads struct {
	Service
	thread Thread
}

func (f fakeMailThreads) ThreadByID(context.Context, int) (*Thread, error) { return &f.thread, nil }

type fakeMailUsers struct{ modulekit.UserRepository }

// ByID - user<id>, active; user6 is blocked.
func (fakeMailUsers) ByID(_ context.Context, id int) (*modulekit.User, error) {
	status := modulekit.UserStatusActive
	if id == 6 {
		status = 0
	}
	return &modulekit.User{ID: id, Username: fmt.Sprintf("user%d", id), Email: fmt.Sprintf("user%d@example.com", id), Status: status}, nil
}

type fakeMailSite struct {
	modulekit.SiteInformationRepository
}

func (fakeMailSite) Get(context.Context) (*modulekit.SiteInformation, error) {
	return &modulekit.SiteInformation{SiteName: "Example", SiteEmail: "admin@example.com"}, nil
}

type fakeMailSender struct {
	to, keys []string
	data     []mailtemplate.TokenData
}

func (f *fakeMailSender) Send(_ context.Context, key, to string, data mailtemplate.TokenData) error {
	f.to = append(f.to, to)
	f.keys = append(f.keys, key)
	f.data = append(f.data, data)
	return nil
}

// TestMailerNotify - a new message mails each member once but its
// author, who deleted the thread, who turned the e-mail off and a blocked
// account, the module's e-mail with its placeholders' values; without the
// site URL, no one.
func TestMailerNotify(t *testing.T) {
	thread := Thread{ID: 7, Subject: "Hello", Members: []Member{
		{UserID: 1}, {UserID: 2}, {UserID: 3, Deleted: true}, {UserID: 4}, {UserID: 2}, {UserID: 5}, {UserID: 6},
	}}
	mail := &fakeMailSender{}
	m := NewMailer(MailerDeps{Threads: fakeMailThreads{thread: thread}, Users: fakeMailUsers{}, Site: fakeMailSite{},
		MailOff: func(context.Context, []int) (map[int]bool, error) { return map[int]bool{5: true}, nil },
		Mail:    mail, Logger: slog.New(slog.DiscardHandler)})
	if err := m.Notify(t.Context(), 7, 1, ""); err != nil || len(mail.to) != 0 {
		t.Fatalf("without the site URL: %v, mailed %v", err, mail.to)
	}
	if err := m.Notify(t.Context(), 7, 1, "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(mail.to) != "[user2@example.com user4@example.com]" {
		t.Errorf("mailed %v", mail.to)
	}
	// The module's placeholders: the sender, the thread's address.
	if extra := mail.data[0].Extra; mail.keys[0] != MailNewMessage || extra["[pm:sender]"] != "user1" ||
		extra["[pm:thread-url]"] != "https://example.com/me/messages/thread/7" {
		t.Errorf("e-mail %s: %v", mail.keys[0], extra)
	}
}

// TestThreadCreateActiveRecipients - a new thread's recipient is an active
// user: a blocked one is refused (ErrNoSuchRecipient) before anything is
// written.
func TestThreadCreateActiveRecipients(t *testing.T) {
	s := NewService(nil, nil, nil, fakeMailUsers{}, slog.New(slog.DiscardHandler))
	if _, err := s.ThreadCreate(t.Context(), 1, "Hi", []int{6}, "Hello"); !errors.Is(err, ErrNoSuchRecipient) {
		t.Errorf("a blocked recipient: %v, want ErrNoSuchRecipient", err)
	}
}
