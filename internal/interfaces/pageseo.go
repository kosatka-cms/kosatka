package interfaces

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/metadescription"
)

// SEOProvider is implemented by entities that can provide SEO data.
type SEOProvider interface {
	GetNode() entity.Node
	GetContent() string
}

// PageSEO builds a PageSEO from an entity's data.
func PageSEO(provider SEOProvider) entity.PageSEO {
	node := provider.GetNode()
	pageSEO := entity.PageSEO{}

	title := node.Title
	if node.PageSEO.Title != "" {
		title = node.PageSEO.Title
	}

	desc := node.PageSEO.Description
	if desc == "" {
		desc = metadescription.Generate(provider.GetContent(), entity.MaxMetaDesc)
	}

	settings := entity.NewSystemSettings()
	pageSEO.Title = title
	pageSEO.Description = desc
	// node.URL - the module's prefix (a forum topic's) and the language's.
	pageSEO.OGURL = settings.Domain + node.URL()
	pageSEO.Canonical = settings.Domain + node.URL()

	return pageSEO
}

// withPreloadImage - p preloading pic (the page's main picture): its
// srcset too, so the browser fetches the one its <img> takes.
func withPreloadImage(p entity.PageSEO, pic imageurl.Picture) entity.PageSEO {
	if uri := strings.TrimSpace(pic.Src); uri != "" {
		p.PreloadImage = uri
		p.PreloadImageSrcset = pic.Srcset
	}
	return p
}

func siteTitleFromSiteInformation(site *entity.SiteInformation) string {
	if site == nil {
		return ""
	}
	return strings.TrimSpace(site.SiteName)
}

// cloneURLValues deep-copies url.Values (the []string slices are not shared).
func cloneURLValues(v url.Values) url.Values {
	if len(v) == 0 {
		return url.Values{}
	}
	out := make(url.Values, len(v))
	for k, vals := range v {
		cp := make([]string, len(vals))
		copy(cp, vals)
		out[k] = cp
	}
	return out
}

// canonicalURLForPaginatedPage builds a self-canonical URL for paginated lists:
// page is omitted on the first page and set to N for page > 1. Other non-empty
// query parameters (filters, sorting) are kept; empty ones (?publisher=) are
// dropped so they don't get a canonical of their own for identical content.
func canonicalURLForPaginatedPage(domain, path string, page int, query url.Values) string {
	v := cloneURLValues(query)
	for k, vals := range v {
		if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
			v.Del(k)
		}
	}
	v.Del("page")
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	enc := v.Encode()
	if enc == "" {
		return domain + path
	}
	return domain + path + "?" + enc
}

// filterParamActive reports whether any of the given query parameters is
// non-empty. Used to mark filtered catalog pages noindex,follow: they are a
// subset of already indexed content with unbounded filter combinations.
func filterParamActive(query url.Values, keys ...string) bool {
	for _, k := range keys {
		if strings.TrimSpace(query.Get(k)) != "" {
			return true
		}
	}
	return false
}

// seoLangContext - the language context of the SEO builders whose callers
// pass no request context: the site default language.
func seoLangContext() context.Context { return context.Background() }

// pagedTitle - a paginated page's title: "title - page N".
func pagedTitle(title string, page int) string {
	return i18n.T(seoLangContext(), "@title - page @page", "@title", title, "@page", page)
}

// PageSEOForPaginatedList - PageSEO for paginated lists (canonical/og:url,
// title on page 2+). path is the path without query (as r.URL.Path).
func PageSEOForPaginatedList(title, path string, page int, query url.Values) entity.PageSEO {
	settings := entity.NewSystemSettings()

	titleOut := title
	if page > 1 && strings.TrimSpace(title) != "" {
		titleOut = pagedTitle(titleOut, page)
	}

	fullURL := canonicalURLForPaginatedPage(settings.Domain, path, page, query)

	return entity.PageSEO{
		Title:     titleOut,
		Canonical: fullURL,
		OGURL:     fullURL,
		OGImage:   "",
	}
}

// ApplyPaginationToPageSEO resets Canonical and OGURL and adds a page suffix
// to Title for page > 1, for SEO built separately (e.g. term pages).
func ApplyPaginationToPageSEO(seo *entity.PageSEO, canonicalPath string, page int, query url.Values) {
	if seo == nil {
		return
	}
	settings := entity.NewSystemSettings()
	seo.Canonical = canonicalURLForPaginatedPage(settings.Domain, canonicalPath, page, query)
	seo.OGURL = seo.Canonical
	if page > 1 && strings.TrimSpace(seo.Title) != "" {
		seo.Title = pagedTitle(seo.Title, page)
	}
}

// PageSEOForList builds PageSEO for list pages without pagination in the URL.
// title is the page title; path is the page URL path (e.g. "/articles").
func PageSEOForList(title, path string) entity.PageSEO {
	return PageSEOForPaginatedList(title, path, 1, nil)
}

// pageSEOForEditorPage - SEO for the /create/{type} and /node/{id}/edit forms.
func pageSEOForEditorPage(ctx context.Context, actionTitle, path string, site *entity.SiteInformation) entity.PageSEO {
	settings := entity.NewSystemSettings()
	fullURL := fmt.Sprintf("%s%s", settings.Domain, path)
	sitePart := siteTitleFromSiteInformation(site)
	title := fmt.Sprintf("%s | %s", actionTitle, sitePart)
	desc := metadescription.Generate(
		i18n.T(ctx, "@action - creating or editing content on the site.", "@action", actionTitle),
		entity.MaxMetaDesc,
	)
	return entity.PageSEO{
		Title:       title,
		Description: desc,
		Canonical:   fullURL,
		OGURL:       fullURL,
		OGImage:     "",
	}
}

// EditorPageSEO fills site_name from settings (if the repository is set on BaseController).
func (c *BaseController) EditorPageSEO(r *http.Request, actionTitle string) entity.PageSEO {
	var site *entity.SiteInformation
	if c.siteInformationRepo != nil {
		if si, err := c.siteInformationRepo.Get(r.Context()); err == nil {
			site = si
		}
	}
	return pageSEOForEditorPage(r.Context(), actionTitle, r.URL.Path, site)
}

// ApplyEditorPageSEOFromTitle sets data["pageSEO"] from data["title"] (create/edit forms).
func (c *BaseController) ApplyEditorPageSEOFromTitle(data map[string]any, r *http.Request) {
	t, ok := data["title"].(string)
	if !ok || strings.TrimSpace(t) == "" {
		return
	}
	data["pageSEO"] = c.EditorPageSEO(r, t)
}

func profileSEOUserLabel(u *entity.User) string {
	if u == nil {
		return ""
	}
	if s := strings.TrimSpace(u.Username); s != "" {
		return s
	}
	return i18n.T(seoLangContext(), "User @id", "@id", u.ID)
}

// PageSEOForUserProfileTab - a profile's View tab (user_tab display).
func PageSEOForUserProfileTab(u *entity.User, tabLabel, path string) entity.PageSEO {
	p := PageSEOForUserProfile(u, path)
	name := profileSEOUserLabel(u)
	p.Title = tabLabel + " - " + name
	p.Description = metadescription.Generate(i18n.T(seoLangContext(), "@tab of user @name.", "@tab", tabLabel, "@name", name), entity.MaxMetaDesc)
	return p
}

// PageSEOForUserProfile - SEO for /me and /users/{alias} (its tabs:
// PageSEOForUserProfileTab).
func PageSEOForUserProfile(u *entity.User, path string) entity.PageSEO {
	settings := entity.NewSystemSettings()
	fullURL := fmt.Sprintf("%s%s", settings.Domain, path)
	name := profileSEOUserLabel(u)
	ctx := seoLangContext()
	title := i18n.T(ctx, "Profile - @name", "@name", name)
	descPlain := i18n.T(ctx, "Page of user @name: profile, content and activity on the site.", "@name", name)
	desc := metadescription.Generate(descPlain, entity.MaxMetaDesc)
	return entity.PageSEO{
		Title:       title,
		Description: desc,
		Canonical:   fullURL,
		OGURL:       fullURL,
		OGImage:     "",
	}
}

// PageSEOForProfileEdit - SEO for /me/edit and /user/{id}/edit.
func PageSEOForProfileEdit(u *entity.User, path string) entity.PageSEO {
	settings := entity.NewSystemSettings()
	fullURL := fmt.Sprintf("%s%s", settings.Domain, path)
	name := profileSEOUserLabel(u)
	ctx := seoLangContext()
	title := i18n.T(ctx, "Edit profile - @name", "@name", name)
	desc := metadescription.Generate(
		i18n.T(ctx, "Changing the profile of user @name: nickname, email, avatar, signature and settings.", "@name", name),
		entity.MaxMetaDesc,
	)
	return entity.PageSEO{
		Title:       title,
		Description: desc,
		Canonical:   fullURL,
		OGURL:       fullURL,
		OGImage:     "",
	}
}

// PageSEOForHome - the home page: title is "site_name | slogan", description
// is the slogan. For page > 1, canonical and og:url are https://…/?page=N.
func PageSEOForHome(site *entity.SiteInformation, page int) entity.PageSEO {
	settings := entity.NewSystemSettings()
	fullURL := settings.Domain + "/"
	if page > 1 {
		fullURL += "?page=" + strconv.Itoa(page)
	}
	siteName := ""
	slogan := ""
	if site != nil {
		siteName = strings.TrimSpace(site.SiteName)
		slogan = strings.TrimSpace(site.Slogan)
	}
	title := ""
	if siteName != "" && slogan != "" {
		title = siteName + " | " + slogan
	} else if siteName != "" {
		title = siteName
	} else if slogan != "" {
		title = slogan
	}
	if page > 1 && title != "" {
		title = pagedTitle(title, page)
	}
	return entity.PageSEO{
		Title:       title,
		Description: slogan,
		Canonical:   fullURL,
		OGURL:       fullURL,
		OGImage:     "",
	}
}
