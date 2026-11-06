package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/mailsender"
)

type stubMailRepo struct{ s entity.MailSettings }

func (r *stubMailRepo) Get(context.Context) (*entity.MailSettings, error) {
	s := r.s
	return &s, nil
}

func (r *stubMailRepo) Update(_ context.Context, s *entity.MailSettings) error {
	r.s = *s
	return nil
}

// recordingSender records what it sends and the settings it was built
// with.
type recordingSender struct {
	built entity.MailSettings
	sent  []string
}

func (s *recordingSender) SendPlain(to, _, _ string) error {
	s.sent = append(s.sent, to)
	return nil
}

func newTestMailService(repo *stubMailRepo, config entity.MailSettings) (*MailService, *[]*recordingSender) {
	built := &[]*recordingSender{}
	svc := NewMailService(repo, config, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	svc.newSender = func(m entity.MailSettings) (mailsender.EmailSender, error) {
		s := &recordingSender{built: m}
		*built = append(*built, s)
		return s, nil
	}
	return svc, built
}

// TestMailServiceSettings - the config's transport wins over the admin's,
// without one the admin's apply, "off" sends nothing.
func TestMailServiceSettings(t *testing.T) {
	ctx := context.Background()
	useTestMailTransport(t)
	config := entity.MailSettings{Transport: testTransport, FromAddress: "config@example.com", TransportSettings: map[string]map[string]string{testTransport: {"key": "k_config"}}}
	repo := &stubMailRepo{s: entity.MailSettings{Enabled: true}}
	svc, built := newTestMailService(repo, config)

	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if st := svc.Status(); st.Source != MailSourceConfig || st.Transport != testTransport || !st.Enabled {
		t.Fatalf("empty row: %+v", st)
	}
	if err := svc.SendPlain("a@example.com", "s", "b"); err != nil || len((*built)[0].sent) != 1 {
		t.Fatalf("config transport: %v %+v", err, (*built)[0])
	}

	// Saved settings don't replace the config's transport.
	repo.s = entity.MailSettings{Enabled: true, Transport: entity.MailTransportSMTP, FromAddress: "admin@example.com", SMTPHost: "smtp.example.com", SMTPPort: 587}
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if st := svc.Status(); st.Source != MailSourceConfig || st.Transport != testTransport || st.FromAddress != "config@example.com" {
		t.Fatalf("admin settings over the config's: %+v", st)
	}

	// Without a transport in the config the admin's apply.
	svc, built = newTestMailService(repo, entity.MailSettings{})
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	last := (*built)[len(*built)-1]
	if st := svc.Status(); st.Source != MailSourceAdmin || st.Transport != entity.MailTransportSMTP {
		t.Fatalf("admin settings: %+v", st)
	}
	if last.built.SMTPHost != "smtp.example.com" || last.built.FromAddress != "admin@example.com" {
		t.Fatalf("rebuilt with: %+v", last.built)
	}

	repo.s.Enabled = false
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.SendPlain("a@example.com", "s", "b"); err != nil {
		t.Fatalf("off: %v", err)
	}
	if !errors.Is(svc.SendTest("a@example.com", "s", "b"), ErrMailOff) {
		t.Error("test email with email off")
	}
	if n := len(last.sent); n != 0 {
		t.Errorf("sent %d emails with email off", n)
	}

	// Off also switches the config's transport off.
	repo.s = entity.MailSettings{Enabled: false}
	svc, _ = newTestMailService(repo, config)
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if st := svc.Status(); st.Enabled || st.Source != MailSourceConfig {
		t.Errorf("off with the config's transport: %+v", st)
	}
}

// TestMailServiceNotConfigured - nothing in the admin or the config: no
// email, SendTest reports it.
func TestMailServiceNotConfigured(t *testing.T) {
	svc, built := newTestMailService(&stubMailRepo{s: entity.MailSettings{Enabled: true}}, entity.MailSettings{})
	if err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := svc.Status(); st.Source != MailSourceNone {
		t.Errorf("status: %+v", st)
	}
	if err := svc.SendPlain("a@example.com", "s", "b"); err != nil {
		t.Errorf("SendPlain: %v", err)
	}
	if !errors.Is(svc.SendTest("a@example.com", "s", "b"), ErrMailOff) || len(*built) != 0 {
		t.Error("a sender built without settings")
	}
}

// TestMailConfigured - SMTP with its server and port, a module's
// transport the binary has with its required settings, and a sender.
func TestMailConfigured(t *testing.T) {
	useTestMailTransport(t)
	withKey := map[string]map[string]string{testTransport: {"key": "k"}}
	cases := []struct {
		m    entity.MailSettings
		want bool
	}{
		{entity.MailSettings{}, false},
		{entity.MailSettings{Transport: testTransport, FromAddress: "a@example.com"}, false},
		{entity.MailSettings{Transport: testTransport, FromAddress: "a@example.com", TransportSettings: map[string]map[string]string{testTransport: {"region": "eu"}}}, false},
		{entity.MailSettings{Transport: testTransport, FromAddress: "a@example.com", TransportSettings: withKey}, true},
		{entity.MailSettings{Transport: "missing", FromAddress: "a@example.com", TransportSettings: map[string]map[string]string{"missing": {"key": "k"}}}, false},
		{entity.MailSettings{Transport: entity.MailTransportSMTP, FromAddress: "a@example.com", SMTPHost: "h"}, false},
		{entity.MailSettings{Transport: entity.MailTransportSMTP, FromAddress: "a@example.com", SMTPHost: "h", SMTPPort: 25}, true},
		{entity.MailSettings{Transport: entity.MailTransportSMTP, SMTPHost: "h", SMTPPort: 25}, false},
	}
	for i, c := range cases {
		if got := mailConfigured(c.m); got != c.want {
			t.Errorf("%d: %v, want %v", i, got, c.want)
		}
	}
}
