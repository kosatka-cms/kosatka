package service

import (
	"path"
	"strings"
	"sync/atomic"

	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// The site's icon (favicon): the one uploaded
// on Site information (entity.SiteInformation.Icon) is the site's in
// every theme; without one, the theme's (theme.yaml "favicon", its base
// theme's for a subtheme). The pages' <head> carries it (template func
// siteIcon, the "site_icon" define), /favicon.ico leads to it.

// SiteIconBasePath - where an uploaded site icon is stored.
const SiteIconBasePath = "site/icon"

// SiteIcon - a page's icon: its URL, its media type ("" - unknown),
// Touch - also the home screen's icon (apple-touch-icon: a PNG).
type SiteIcon struct {
	URL   string
	Type  string
	Touch bool
}

// siteIconURI - the uploaded icon's URI (SetSiteIcon).
var siteIconURI atomic.Pointer[string]

// SetSiteIcon makes uri (an uploaded file's, "" - none) the site's icon:
// at startup and when Site information is saved.
func SetSiteIcon(uri string) {
	siteIconURI.Store(&uri)
}

// SiteIconFor - the icon of the pages in theme dir: the site's, else the
// theme chain's (paths.ThemeChain) first favicon; zero - none.
func SiteIconFor(dir string) SiteIcon {
	if uri := siteIconURI.Load(); uri != nil && *uri != "" {
		return newSiteIcon(imageurl.PublicURI(*uri))
	}
	for _, theme := range paths.ThemeChain(dir) {
		if favicon := themes.Favicon(theme); favicon != "" {
			return newSiteIcon(favicon)
		}
	}
	return SiteIcon{}
}

// siteIconTypes - an icon's media type by its extension.
var siteIconTypes = map[string]string{
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
	".gif":  "image/gif",
	".webp": "image/webp",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
}

func newSiteIcon(url string) SiteIcon {
	mime := siteIconTypes[strings.ToLower(path.Ext(url))]
	return SiteIcon{URL: url, Type: mime, Touch: mime == "image/png"}
}
