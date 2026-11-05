package service

import (
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// robotsCoreDisallow - the engine's pages crawlers have no business on:
// the admin, sign-in and account pages, content forms and actions.
var robotsCoreDisallow = []string{
	"/admin/",
	"/login",
	"/logout",
	"/user/",
	"/me/",
	"/auth/",
	"/create/",
	"/search?",
	"/node/*/edit",
	"/node/*/delete",
	"/node/*/publish",
	"/node/*/unpublish",
	"/comment/*/edit",
	"/comment/*/delete",
}

// RobotsTxt - the site's robots.txt: the engine's Disallow rules and
// those of the enabled modules (entity.Module.RobotsDisallow) for every
// crawler, the sitemap at baseURL ("" - none), then the site's own rules
// (entity.SiteInformation.RobotsExtra) as written.
func RobotsTxt(baseURL, extra string) string {
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	for _, p := range robotsCoreDisallow {
		b.WriteString("Disallow: " + p + "\n")
	}
	for _, m := range entity.Modules {
		if !ModuleEnabled(m.Name) {
			continue
		}
		for _, p := range m.RobotsDisallow {
			b.WriteString("Disallow: " + p + "\n")
		}
	}
	if baseURL = NormalizeBaseURL(baseURL); baseURL != "" {
		b.WriteString("\nSitemap: " + baseURL + "/sitemap.xml\n")
	}
	if extra = strings.TrimSpace(strings.ReplaceAll(extra, "\r\n", "\n")); extra != "" {
		b.WriteString("\n" + extra + "\n")
	}
	return b.String()
}
