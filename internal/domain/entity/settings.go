package entity

import (
	"strings"
	"sync/atomic"
)

// Settings - the process's site-wide settings (NewSystemSettings): the
// site's canonical origin.
type Settings struct {
	// Domain - the site's canonical origin (config public_base_url /
	// PUBLIC_BASE_URL, else the address saved in Site information), no
	// trailing slash; empty - URLs stay site-relative.
	Domain string
}

var siteDomain atomic.Value

// SetSiteDomain sets the canonical origin NewSystemSettings returns: at
// startup from the config, then by service.SiteURL.Apply (the config's
// address, else the one saved in Site information).
func SetSiteDomain(domain string) {
	siteDomain.Store(strings.TrimRight(strings.TrimSpace(domain), "/"))
}

// NewSystemSettings - the settings as SetSiteDomain last set them.
func NewSystemSettings() Settings {
	d, _ := siteDomain.Load().(string)
	return Settings{Domain: d}
}
