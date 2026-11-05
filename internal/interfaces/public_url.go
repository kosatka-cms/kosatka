package interfaces

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// publicBaseURL returns the canonical site origin for absolute URLs (sitemap, RSS, etc.).
// Uses the site URL (service.SiteURL: the config's or the admin's) when set;
// otherwise derives scheme and host from the request. Not for e-mails:
// mailBaseURL.
func publicBaseURL(r *http.Request) string {
	if u := entity.NewSystemSettings().Domain; u != "" {
		return u
	}
	scheme := "http"
	if httputil.IsHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// mailBaseURL - the site's origin for the links in e-mails: only the site
// URL set (service.SiteURL), never the request's host - a request with a
// forged Host would mail a password reset link to another site. "" - not
// set: e-mails with links aren't sent (the status report says so).
func mailBaseURL() string {
	return entity.NewSystemSettings().Domain
}
