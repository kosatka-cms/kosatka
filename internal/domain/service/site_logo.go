package service

import (
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// SiteLogoFor - the logo of the pages in theme dir: the one uploaded on
// Site information (uploaded - its URI, entity.SiteInformation.Logo), in
// every theme; without one, the theme chain's (paths.ThemeChain) first
// logo (theme.yaml "logo"); "" - none. Templates get it as .siteLogoURI.
func SiteLogoFor(dir, uploaded string) string {
	if uploaded != "" {
		return imageurl.PublicURI(uploaded)
	}
	for _, theme := range paths.ThemeChain(dir) {
		if logo := themes.Logo(theme); logo != "" {
			return logo
		}
	}
	return ""
}
