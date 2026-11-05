package interfaces

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// NotFoundBreadcrumbs - the 404 page's trail (no JSON-LD). Other pages'
// trails come from service.BreadcrumbService via the BaseController
// helpers below. No request context here: labels use the site default
// language.
func NotFoundBreadcrumbs() []entity.BreadcrumbItem {
	ctx := context.Background()
	return []entity.BreadcrumbItem{
		{URL: "/", Label: i18n.T(ctx, "Home")},
		{Label: i18n.T(ctx, "Page not found")},
	}
}

// setTrail puts a page's trail and its BreadcrumbList JSON-LD into the
// template data (breadcrumbs_nav, header.html).
func (c *BaseController) setTrail(data map[string]any, trail []entity.BreadcrumbItem) {
	if c.breadcrumbs == nil || len(trail) == 0 {
		return
	}
	data["breadcrumbs"] = trail
	data["breadcrumbsJSONLD"] = c.breadcrumbs.JSONLD(trail)
}

// trailLangContext - the language context for a trail built without the
// request context (setPathTrail, setChildTrail): data's
// service.RequestLanguageKey, otherwise the site default language.
func trailLangContext(data map[string]any) context.Context {
	if lang, ok := data[service.RequestLanguageKey].(string); ok && lang != "" {
		return i18n.WithLang(context.Background(), lang)
	}
	return context.Background()
}

// setPathTrail - the trail of path's menu link, or "Home › title".
func (c *BaseController) setPathTrail(data map[string]any, path, title string) {
	if c.breadcrumbs != nil {
		c.setTrail(data, c.breadcrumbs.TrailOrTitle(trailLangContext(data), path, title))
	}
}

// setChildTrail - see BreadcrumbService.ChildTrail.
func (c *BaseController) setChildTrail(data map[string]any, parents []entity.BreadcrumbItem, title string) {
	if c.breadcrumbs != nil {
		c.setTrail(data, c.breadcrumbs.ChildTrail(trailLangContext(data), parents, title))
	}
}

// setNodeTrail - a node page's trail (dc fully loaded).
func (c *BaseController) setNodeTrail(ctx context.Context, data map[string]any, dc entity.DynamicContent, title string) {
	if c.breadcrumbs != nil {
		c.setTrail(data, c.breadcrumbs.TrailForNode(ctx, dc, title))
	}
}

// setNodeRefTrail - a node page without loaded fields (NodeTrail).
func (c *BaseController) setNodeRefTrail(ctx context.Context, data map[string]any, node entity.Node) {
	if c.breadcrumbs != nil {
		c.setTrail(data, c.breadcrumbs.NodeTrail(ctx, node))
	}
}

// setNodeSubpageTrail - see BreadcrumbService.NodeSubpageTrail.
func (c *BaseController) setNodeSubpageTrail(ctx context.Context, data map[string]any, node entity.Node, title string, extra ...entity.BreadcrumbItem) {
	if c.breadcrumbs != nil {
		c.setTrail(data, c.breadcrumbs.NodeSubpageTrail(ctx, node, title, extra...))
	}
}

// createMaterialsHubTitle - the GET /create page's title, also its label
// as the parent of a create form's trail.
func createMaterialsHubTitle(ctx context.Context) string {
	return i18n.T(ctx, "Content creation")
}

// setCreateFormTrail - a create form under the GET /create hub.
func (c *BaseController) setCreateFormTrail(data map[string]any, title string) {
	c.setChildTrail(data, []entity.BreadcrumbItem{{URL: "/create", Label: createMaterialsHubTitle(trailLangContext(data))}}, title)
}
