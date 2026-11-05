package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/recaptcha"
)

// CaptchaService - the reCAPTCHA in effect: the settings saved in the
// admin (entity.CaptchaSettings), with the server config's keys when it
// has both (they win: the admin can't replace them). Reloaded on save,
// read on every form.
type CaptchaService struct {
	repo entity.CaptchaSettingsRepository
	// configSite/configSecret - the server config's keys.
	configSite, configSecret string
	// newVerifier builds the verifier for a secret (tests replace it).
	newVerifier func(secret string) recaptcha.Verifier

	mu       sync.RWMutex
	settings entity.CaptchaSettings
	// fromConfig - the keys in effect are the server config's.
	fromConfig bool
	verifier   recaptcha.Verifier
}

// NewCaptchaService - the service with the server config's keys (they
// win over the admin's); call Reload to read the saved settings.
func NewCaptchaService(repo entity.CaptchaSettingsRepository, configSiteKey, configSecretKey string, httpTimeout time.Duration) *CaptchaService {
	s := &CaptchaService{
		repo:         repo,
		configSite:   strings.TrimSpace(configSiteKey),
		configSecret: strings.TrimSpace(configSecretKey),
		newVerifier:  func(secret string) recaptcha.Verifier { return recaptcha.NewClient(secret, httpTimeout) },
	}
	s.apply(entity.CaptchaSettings{Enabled: true})
	return s
}

// Reload reads the saved settings and makes them current.
func (s *CaptchaService) Reload(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}
	saved, err := s.repo.Get(ctx)
	if err != nil {
		return err
	}
	s.apply(*saved)
	return nil
}

// Saved - the settings saved in the admin (not the server config's keys).
func (s *CaptchaService) Saved(ctx context.Context) (*entity.CaptchaSettings, error) {
	saved, err := s.repo.Get(ctx)
	return entity.Wrapped(saved, err, "loading the CAPTCHA settings")
}

// Save stores the settings and makes them current.
func (s *CaptchaService) Save(ctx context.Context, settings entity.CaptchaSettings) error {
	if err := s.repo.Update(ctx, &settings); err != nil {
		return fmt.Errorf("saving the CAPTCHA settings: %w", err)
	}
	s.apply(settings)
	return nil
}

func (s *CaptchaService) apply(saved entity.CaptchaSettings) {
	effective := entity.CaptchaSettings{
		SiteKey:   strings.TrimSpace(saved.SiteKey),
		SecretKey: strings.TrimSpace(saved.SecretKey),
		Forms:     map[string]bool{},
	}
	fromConfig := s.configSite != "" && s.configSecret != ""
	if fromConfig {
		effective.SiteKey, effective.SecretKey = s.configSite, s.configSecret
	}
	// A form the settings don't mention shows it by its default.
	for _, f := range entity.CaptchaForms() {
		on, set := saved.Forms[f.Key]
		if !set {
			on = f.Default
		}
		effective.Forms[f.Key] = on
	}
	effective.Enabled = saved.Enabled
	var verifier recaptcha.Verifier
	if effective.Enabled && effective.SiteKey != "" && effective.SecretKey != "" {
		verifier = s.newVerifier(effective.SecretKey)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings, s.fromConfig, s.verifier = effective, fromConfig, verifier
}

// SiteKey - the site key for a form (entity.CaptchaForm*) when the CAPTCHA
// is on for it; "" when it isn't (no keys or the form is off).
func (s *CaptchaService) SiteKey(form string) string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.verifier == nil || !s.settings.Forms[form] {
		return ""
	}
	return s.settings.SiteKey
}

// Verify checks a form's CAPTCHA response; true when the CAPTCHA is off
// for the form.
func (s *CaptchaService) Verify(ctx context.Context, form, token, remoteIP string) (bool, error) {
	if s == nil {
		return true, nil
	}
	s.mu.RLock()
	verifier, on := s.verifier, s.settings.Forms[form]
	s.mu.RUnlock()
	if verifier == nil || !on {
		return true, nil
	}
	return verifier.Verify(ctx, strings.TrimSpace(token), remoteIP)
}

// KeysFromConfig - whether the keys in effect come from the server config
// (it has both: the admin's aren't used).
func (s *CaptchaService) KeysFromConfig() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fromConfig
}
