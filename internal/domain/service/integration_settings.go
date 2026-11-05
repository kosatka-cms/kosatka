package service

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// Integration settings errors.
var (
	ErrIntegrationMetrikaID = errors.New("yandex metrika counter must be a number")
	ErrIntegrationGAID      = errors.New("google analytics id: latin letters, digits and dashes, e.g. G-XXXXXXX")
	ErrConsentPolicyURL     = errors.New("the cookie banner's page: a path from / or an http(s) address")
)

// consentTextLimit - how long the banner's own text may be.
const consentTextLimit = 2000

var integrationGAIDRe = regexp.MustCompile(`^[A-Za-z0-9-]{4,32}$`)

// IntegrationSettingsService - analytics counters and custom code: read
// on every page (cached in memory), saved from /admin/config/integrations.
type IntegrationSettingsService struct {
	repo   entity.IntegrationSettingsRepository
	logger *slog.Logger

	mu     sync.RWMutex
	cached *entity.IntegrationSettings
}

// NewIntegrationSettingsService - the integration settings service on
// repo (the stored settings) and logger.
func NewIntegrationSettingsService(repo entity.IntegrationSettingsRepository, logger *slog.Logger) *IntegrationSettingsService {
	return &IntegrationSettingsService{repo: repo, logger: logger.With("component", "integrations.service")}
}

// Current - the settings for rendering a page (zero value - nothing is
// wired - when they can't be loaded).
func (s *IntegrationSettingsService) Current(ctx context.Context) entity.IntegrationSettings {
	s.mu.RLock()
	c := s.cached
	s.mu.RUnlock()
	if c != nil {
		return *c
	}
	loaded, err := s.repo.Get(ctx)
	if err != nil || loaded == nil {
		LogFallback(s.logger, err, "error loading integration settings")
		return entity.IntegrationSettings{}
	}
	s.mu.Lock()
	s.cached = loaded
	s.mu.Unlock()
	return *loaded
}

// Save validates and stores in; without canEditCode (not a full-access
// role) the raw code fields keep their stored values. The consent's
// version stays the stored one (ResetConsent raises it).
func (s *IntegrationSettingsService) Save(ctx context.Context, in entity.IntegrationSettings, canEditCode bool) error {
	in.MetrikaID = strings.TrimSpace(in.MetrikaID)
	if in.MetrikaID != "" && in.MetrikaCounter() == 0 {
		return ErrIntegrationMetrikaID
	}
	in.GAID = strings.TrimSpace(in.GAID)
	if in.GAID != "" && !integrationGAIDRe.MatchString(in.GAID) {
		return ErrIntegrationGAID
	}
	var tags []string
	for _, m := range in.VerificationMetas() {
		tags = append(tags, m.Name+"="+m.Content)
	}
	in.VerificationTags = strings.Join(tags, "\n")
	if err := normalizeConsent(&in.Consent); err != nil {
		return err
	}
	current, err := s.repo.Get(ctx)
	if err != nil {
		return err
	}
	in.Consent.Version = max(current.Consent.Version, 1)
	if !canEditCode {
		in.HeadCode, in.BodyEndCode, in.BodyEndSkipServicePages = current.HeadCode, current.BodyEndCode, current.BodyEndSkipServicePages
		in.Consent.HeadGroup, in.Consent.BodyEndGroup = current.Consent.HeadGroup, current.Consent.BodyEndGroup
	}
	if err := s.repo.Update(ctx, in); err != nil {
		return err
	}
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
	return nil
}

// normalizeConsent - c with a known mode and groups, its text trimmed
// (at most consentTextLimit characters); ErrConsentPolicyURL for a page
// that is neither a site path nor an http(s) address.
func normalizeConsent(c *entity.Consent) error {
	if c.Mode != entity.ConsentNotice {
		c.Mode = entity.ConsentAsk
	}
	c.Text = strings.TrimSpace(c.Text)
	if r := []rune(c.Text); len(r) > consentTextLimit {
		c.Text = string(r[:consentTextLimit])
	}
	c.PolicyURL = strings.TrimSpace(c.PolicyURL)
	if u := c.PolicyURL; u != "" && !consentPageURL(u) {
		return ErrConsentPolicyURL
	}
	for _, g := range []*entity.ConsentGroup{&c.MetrikaGroup, &c.GAGroup, &c.HeadGroup, &c.BodyEndGroup} {
		*g = entity.ParseConsentGroup(string(*g))
	}
	return nil
}

// ResetConsent raises the consent's version: every visitor's choice is
// forgotten, the banner asks again (after the site's policy changed).
func (s *IntegrationSettingsService) ResetConsent(ctx context.Context) error {
	current, err := s.repo.Get(ctx)
	if err != nil {
		return err
	}
	current.Consent.Version = max(current.Consent.Version, 1) + 1
	if err := s.repo.Update(ctx, *current); err != nil {
		return err
	}
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
	return nil
}

// consentPageURL - whether u is a path on the site (/…, not //…) or an
// http(s) address.
func consentPageURL(u string) bool {
	if strings.HasPrefix(u, "/") {
		return !strings.HasPrefix(u, "//")
	}
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}
