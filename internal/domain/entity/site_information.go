package entity

import "context"

// SiteInformation is the row from site_information (site name, slogan, admin contact email).
type SiteInformation struct {
	SiteName  string
	Slogan    string
	SiteEmail string
	// Logo - the site's logo; only the URI is stored (like User.Avatar).
	// Empty - the header logo block falls back to the theme's default logo.
	Logo File
	// Icon - the site's icon (favicon, the home screen's icon); only the
	// URI is stored. Empty - the theme's (theme.yaml "favicon",
	// service.SiteIconFor).
	Icon File
	// FrontFeedView - machine_name of the View that provides the front page
	// feed (and the RSS feed's items). Empty - no feed.
	FrontFeedView string
	// Timezone - the site's time zone (IANA name, e.g. "Europe/Berlin"):
	// dates are stored in UTC and shown in it (pkg/sitetime). Empty - UTC.
	Timezone string
	// DefaultLanguage - the site's interface language (a pkg/i18n catalog
	// language, e.g. "ru"; "en" - the source strings).
	DefaultLanguage string
	// Languages - the site's languages besides DefaultLanguage, each with
	// its addresses' prefix (SetSiteLanguages); none - one language.
	Languages []SiteLanguage
	// Page404, Page403 - paths of published nodes shown, with the error
	// status, for "not found" and "access denied", one per line (a random
	// one each time). Empty - the theme's or the engine's default page.
	Page404 string
	Page403 string
	// Theme - the active theme chosen on the Appearance page: a directory
	// under themes/, "" - the engine's own templates. Nil -
	// not chosen; the config's theme applies.
	Theme *string
	// BaseURL - the site's canonical origin (e.g. https://www.example.com,
	// no trailing slash) for absolute URLs: sitemap, RSS, canonical links,
	// OAuth callbacks. The config's public_base_url, when set, overrides
	// it (service.SiteURL). Empty - derived from the request.
	BaseURL string
	// RobotsExtra - the site's own robots.txt rules, added after the
	// engine's and the modules' (service.RobotsTxt).
	RobotsExtra string
	// SchemaOrg - who runs the site, for search engines (SetSchemaOrg).
	SchemaOrg SiteSchemaOrg
}

// SiteInformationRepository persists site-wide branding and contact.
type SiteInformationRepository interface {
	Get(ctx context.Context) (*SiteInformation, error)
	Update(ctx context.Context, s *SiteInformation) error
	SetTheme(ctx context.Context, theme string) error
	SetRobotsExtra(ctx context.Context, rules string) error
	SetLanguages(ctx context.Context, languages []SiteLanguage) error
	// SetSchemaOrg stores who runs the site (SiteInformation.SchemaOrg).
	SetSchemaOrg(ctx context.Context, o SiteSchemaOrg) error
}
