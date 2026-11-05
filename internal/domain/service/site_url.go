package service

import (
	"net/url"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// SiteURL - the site's canonical origin for absolute URLs (sitemap, RSS,
// canonical links, OAuth callbacks): the config's public_base_url when
// set, else the one saved in Site information (entity.SiteInformation.
// BaseURL); empty - derived from the request. The config wins so that a
// copy of the production database on a staging or local site keeps that
// site's own URL. Apply publishes the result (entity.NewSystemSettings)
// and tells the listeners.
type SiteURL struct {
	override  string
	mu        sync.Mutex
	listeners []func(baseURL string)
}

// NewSiteURL - with the config's public_base_url ("" - none).
func NewSiteURL(override string) *SiteURL {
	return &SiteURL{override: NormalizeBaseURL(override)}
}

// Override - the config's URL; "" - the admin's applies.
func (s *SiteURL) Override() string { return s.override }

// Apply sets the URL saved in the admin; the config's, if any, stays.
func (s *SiteURL) Apply(saved string) {
	u := s.override
	if u == "" {
		u = NormalizeBaseURL(saved)
	}
	entity.SetSiteDomain(u)
	s.mu.Lock()
	listeners := s.listeners
	s.mu.Unlock()
	for _, f := range listeners {
		f(u)
	}
}

// OnChange adds a listener Apply calls with the effective URL.
func (s *SiteURL) OnChange(f func(baseURL string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, f)
}

// NormalizeBaseURL trims spaces and trailing slashes.
func NormalizeBaseURL(u string) string {
	return strings.TrimRight(strings.TrimSpace(u), "/")
}

// ValidBaseURL - u is empty or an origin: http(s), a host, no path,
// query or fragment.
func ValidBaseURL(u string) bool {
	u = NormalizeBaseURL(u)
	if u == "" {
		return true
	}
	p, err := url.Parse(u)
	return err == nil && (p.Scheme == "http" || p.Scheme == "https") && p.Host != "" &&
		p.Path == "" && p.RawQuery == "" && p.Fragment == "" && p.User == nil
}
