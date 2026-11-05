package service

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/recaptcha"
)

type fakeCaptchaRepo struct{ s entity.CaptchaSettings }

func (f *fakeCaptchaRepo) Get(context.Context) (*entity.CaptchaSettings, error) {
	c := f.s
	return &c, nil
}

func (f *fakeCaptchaRepo) Update(_ context.Context, s *entity.CaptchaSettings) error {
	f.s = *s
	return nil
}

// fakeVerifier accepts the token "ok" and remembers its secret.
type fakeVerifier struct{ secret string }

func (v fakeVerifier) Verify(_ context.Context, token, _ string) (bool, error) {
	return token == "ok", nil
}

func newTestCaptcha(repo *fakeCaptchaRepo, configSite, configSecret string) (*CaptchaService, *[]string) {
	s := NewCaptchaService(repo, configSite, configSecret, time.Second)
	var secrets []string
	s.newVerifier = func(secret string) recaptcha.Verifier {
		secrets = append(secrets, secret)
		return fakeVerifier{secret}
	}
	s.apply(entity.CaptchaSettings{})
	return s, &secrets
}

func TestCaptchaService(t *testing.T) {
	ctx := t.Context()
	allForms := map[string]bool{entity.CaptchaFormLogin: true, entity.CaptchaFormRegister: true, entity.CaptchaFormPassword: true, "error_report": true}

	// No keys anywhere: off everywhere, every response passes.
	off, _ := newTestCaptcha(&fakeCaptchaRepo{}, "", "")
	if off.SiteKey(entity.CaptchaFormLogin) != "" {
		t.Error("site key without keys")
	}
	if ok, _ := off.Verify(ctx, entity.CaptchaFormLogin, "", ""); !ok {
		t.Error("verification without keys must pass")
	}

	// Nothing saved: the config's keys, every form on.
	repo := &fakeCaptchaRepo{s: entity.CaptchaSettings{Enabled: true, Forms: allForms}}
	s, _ := newTestCaptcha(repo, "cfg-site", "cfg-secret")
	if err := s.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if s.SiteKey(entity.CaptchaFormRegister) != "cfg-site" || !s.KeysFromConfig() {
		t.Errorf("config keys: %q, from config %v", s.SiteKey(entity.CaptchaFormRegister), s.KeysFromConfig())
	}
	if ok, _ := s.Verify(ctx, entity.CaptchaFormLogin, "bad", ""); ok {
		t.Error("a wrong response passed")
	}
	if ok, _ := s.Verify(ctx, entity.CaptchaFormLogin, "ok", ""); !ok {
		t.Error("a right response failed")
	}

	// Saved keys don't replace the config's: the server's owner set them.
	repo.s = entity.CaptchaSettings{Enabled: true, SiteKey: "db-site", SecretKey: "db-secret", Forms: allForms}
	if err := s.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if s.SiteKey(entity.CaptchaFormRegister) != "cfg-site" || !s.KeysFromConfig() {
		t.Errorf("saved keys over the config's: %q", s.SiteKey(entity.CaptchaFormRegister))
	}

	// Without keys in the config the saved ones apply; a form switched
	// off has no key and lets everything through.
	repo.s = entity.CaptchaSettings{Enabled: true, SiteKey: "db-site", SecretKey: "db-secret", Forms: map[string]bool{entity.CaptchaFormLogin: false, entity.CaptchaFormRegister: true}}
	s, secrets := newTestCaptcha(repo, "", "")
	if err := s.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if s.SiteKey(entity.CaptchaFormRegister) != "db-site" || s.KeysFromConfig() {
		t.Errorf("saved keys: %q", s.SiteKey(entity.CaptchaFormRegister))
	}
	if s.SiteKey(entity.CaptchaFormLogin) != "" {
		t.Error("a form switched off has a site key")
	}
	if ok, _ := s.Verify(ctx, entity.CaptchaFormLogin, "bad", ""); !ok {
		t.Error("a form switched off checks responses")
	}
	if last := (*secrets)[len(*secrets)-1]; last != "db-secret" {
		t.Errorf("verifier built for %q", last)
	}

	// A half-saved pair (site key only) doesn't count: no CAPTCHA.
	repo.s = entity.CaptchaSettings{Enabled: true, SiteKey: "db-site", Forms: allForms}
	_ = s.Reload(ctx)
	if s.SiteKey(entity.CaptchaFormLogin) != "" {
		t.Errorf("half pair: %q", s.SiteKey(entity.CaptchaFormLogin))
	}

	// The master switch off: no CAPTCHA anywhere, keys and forms kept for
	// when it is switched back on.
	repo.s = entity.CaptchaSettings{Enabled: false, SiteKey: "db-site", SecretKey: "db-secret", Forms: allForms}
	_ = s.Reload(ctx)
	for _, f := range entity.CaptchaFormNames() {
		if s.SiteKey(f) != "" {
			t.Errorf("%s: a site key while switched off", f)
		}
	}
	if ok, _ := s.Verify(ctx, entity.CaptchaFormLogin, "bad", ""); !ok {
		t.Error("checks responses while switched off")
	}
	repo.s.Enabled = true
	_ = s.Reload(ctx)
	if s.SiteKey(entity.CaptchaFormLogin) != "db-site" {
		t.Error("switched back on without its keys")
	}
}

// TestCaptchaModuleFormDefaults - a form the saved settings don't mention
// follows its default: the core's and the error report form show the
// CAPTCHA, the forms module's doesn't until switched on.
func TestCaptchaModuleFormDefaults(t *testing.T) {
	repo := &fakeCaptchaRepo{s: entity.CaptchaSettings{Enabled: true, SiteKey: "k", SecretKey: "s", Forms: map[string]bool{}}}
	s, _ := newTestCaptcha(repo, "", "")
	if err := s.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	for form, want := range map[string]bool{entity.CaptchaFormLogin: true, "error_report": true, "webform": false} {
		if got := s.SiteKey(form) != ""; got != want {
			t.Errorf("%s: shown %v, want %v", form, got, want)
		}
	}
	repo.s.Forms = map[string]bool{"webform": true}
	_ = s.Reload(t.Context())
	if s.SiteKey("webform") == "" {
		t.Error("the forms module's CAPTCHA switched on must show")
	}
}
