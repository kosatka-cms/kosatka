package service

import (
	"context"
	"encoding/json"
	"html/template"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// schemaHeadlineMax - the longest headline search engines take.
const schemaHeadlineMax = 110

// SetSite gives the JSON-LD the site's information: its name, logo and
// who runs it (wiring).
func (s *SEOService) SetSite(site func(ctx context.Context) (*entity.SiteInformation, error)) {
	s.site = site
}

// schemaFor - dc's schema.org type and its properties' templates: its
// content type's, else the default scope's; ok false - none.
func (s *SEOService) schemaFor(ctx context.Context, dc entity.DynamicContent) (entity.SchemaTypeDef, map[string]string, bool) {
	tpl, err := s.Templates(ctx)
	if err != nil {
		LogFallback(s.logger, err, "error loading seo templates")
		return entity.SchemaTypeDef{}, nil, false
	}
	chosen := tpl[entity.SEOTemplateTypeScope(string(dc.Node.Type))]
	if chosen.SchemaType == "" {
		chosen = tpl[entity.SEOTemplateDefaultScope]
	}
	def, ok := entity.SchemaTypeByName(chosen.SchemaType)
	return def, chosen.SchemaProps, ok
}

// schemaBase - the site's address for the JSON-LD's ("" - none: no
// JSON-LD).
func schemaBase() string {
	return NormalizeBaseURL(entity.NewSystemSettings().Domain)
}

// NodeJSONLD - dc's page as its schema.org type (schemaFor), for a
// <script type="application/ld+json">: its name (an article's headline),
// the description and image of p (the page's SEO), its address; a
// creative work's dates; an article's author and the site as its
// publisher, an event's organizer; the type's properties from their
// templates (one rendering empty - left out). Empty - no type, or no
// site address.
func (s *SEOService) NodeJSONLD(ctx context.Context, dc entity.DynamicContent, p entity.PageSEO) template.JS {
	def, props, ok := s.schemaFor(ctx, dc)
	base := schemaBase()
	if !ok || base == "" {
		return ""
	}
	url := base + dc.Node.URL()
	obj := map[string]any{"@context": "https://schema.org", "@type": def.Name, "url": url}
	title := strings.TrimSpace(dc.Node.Title)
	if def.Article {
		obj["headline"] = cutRunes(title, schemaHeadlineMax)
		obj["mainEntityOfPage"] = url
	} else {
		obj["name"] = title
	}
	if d := strings.TrimSpace(p.Description); d != "" {
		obj["description"] = d
	}
	if img := absoluteURL(base, p.OGImage); img != "" {
		obj["image"] = img
	}
	if def.Creative {
		if !dc.Node.CreatedAt.IsZero() {
			obj["datePublished"] = schemaTime(dc.Node.CreatedAt)
		}
		if !dc.Node.UpdatedAt.IsZero() {
			obj["dateModified"] = schemaTime(dc.Node.UpdatedAt)
		}
	}
	if def.Article {
		if author := dc.Node.CreatedBy; author.Username != "" {
			person := map[string]any{"@type": "Person", "name": author.Username}
			if author.Slug != "" {
				person["url"] = base + "/" + strings.TrimPrefix(author.Slug, "/")
			}
			obj["author"] = person
		}
		if org := s.organization(ctx, base); org != nil {
			obj["publisher"] = org
		}
	}
	if def.Name == "Event" {
		if org := s.organization(ctx, base); org != nil {
			obj["organizer"] = org
		}
	}
	for _, prop := range def.Props {
		tpl := strings.TrimSpace(props[prop.Name])
		if tpl == "" {
			continue
		}
		if v := strings.TrimSpace(s.Render(ctx, tpl, dc)); v != "" {
			setSchemaProp(obj, def, prop.Name, v)
		}
	}
	return marshalJSONLD(obj)
}

// SiteJSONLD - the front page's JSON-LD: the site (WebSite, its search
// as a SearchAction when SiteSchemaOrg.SearchBox) and who runs it. Empty -
// no site address.
func (s *SEOService) SiteJSONLD(ctx context.Context) template.JS {
	base := schemaBase()
	if base == "" {
		return ""
	}
	site := s.siteInfo(ctx)
	website := map[string]any{"@type": "WebSite", "url": base + "/"}
	if site != nil {
		website["name"] = site.SiteName
		if site.SchemaOrg.SearchBox {
			website["potentialAction"] = map[string]any{
				"@type":       "SearchAction",
				"target":      base + "/search?q={search_term_string}",
				"query-input": "required name=search_term_string",
			}
		}
	}
	graph := []any{website}
	if org := s.organization(ctx, base); org != nil {
		graph = append(graph, org)
	}
	return marshalJSONLD(map[string]any{"@context": "https://schema.org", "@graph": graph})
}

// siteInfo - the site's information (nil - none, a failure logged).
func (s *SEOService) siteInfo(ctx context.Context) *entity.SiteInformation {
	if s.site == nil {
		return nil
	}
	site, err := s.site(ctx)
	if err != nil {
		LogFallback(s.logger, err, "loading the site for its JSON-LD")
		return nil
	}
	return site
}

// organization - who runs the site (SiteSchemaOrg): its name (the
// site's by default), address, logo, profiles and contacts; nil - no
// site information.
func (s *SEOService) organization(ctx context.Context, base string) map[string]any {
	site := s.siteInfo(ctx)
	if site == nil {
		return nil
	}
	o := site.SchemaOrg
	name := strings.TrimSpace(o.Name)
	if name == "" {
		name = site.SiteName
	}
	org := map[string]any{"@type": o.TypeName(), "name": name, "url": base + "/"}
	if logo := absoluteURL(base, imageurl.PublicURI(site.Logo.URI)); logo != "" {
		if o.TypeName() == entity.SchemaOrgPerson {
			org["image"] = logo
		} else {
			org["logo"] = map[string]any{"@type": "ImageObject", "url": logo}
			if o.TypeName() == entity.SchemaOrgLocalBusiness {
				org["image"] = logo
			}
		}
	}
	if len(o.SameAs) > 0 {
		org["sameAs"] = o.SameAs
	}
	if o.Phone != "" {
		org["telephone"] = o.Phone
	}
	if o.Email != "" {
		org["email"] = o.Email
	}
	if o.Address != "" && o.TypeName() == entity.SchemaOrgLocalBusiness {
		org["address"] = o.Address
	}
	return org
}

// setSchemaProp sets the property name of obj to v: a dotted one
// ("offers.price") in its object, made of def's type for it ("Offer").
func setSchemaProp(obj map[string]any, def entity.SchemaTypeDef, name, v string) {
	parent, child, nested := strings.Cut(name, ".")
	if !nested {
		obj[name] = v
		return
	}
	inner, _ := obj[parent].(map[string]any)
	if inner == nil {
		inner = map[string]any{"@type": def.Objects[parent]}
		obj[parent] = inner
	}
	inner[child] = v
}

// absoluteURL - uri with base when it is a path of the site; "" stays "".
func absoluteURL(base, uri string) string {
	uri = strings.TrimSpace(uri)
	if strings.HasPrefix(uri, "/") && !strings.HasPrefix(uri, "//") {
		return base + uri
	}
	return uri
}

// schemaTime - t in the site's time zone, ISO 8601.
func schemaTime(t time.Time) string { return sitetime.In(t).Format(time.RFC3339) }

// cutRunes - s cut to at most n characters.
func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// marshalJSONLD - obj as JSON for a script (html-safe: <, > and &
// escaped); empty on a failure.
func marshalJSONLD(obj map[string]any) template.JS {
	out, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	return template.JS(out)
}
