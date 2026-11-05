package admin

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// visibilityContentTypeOptions - every content type, for the visibility
// checkboxes on the placement edit pages; an error is only logged (the
// widget renders empty).
func (c *Controller) visibilityContentTypeOptions(ctx context.Context) entity.ContentTypeDefs {
	contentTypes, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		c.LogFailure(err, "error listing content types for block visibility picker")
	}
	return contentTypes
}

// visibilityRoleOption - a role checkbox of the placement visibility.
type visibilityRoleOption struct {
	Name, Label string
}

// visibilityRoleOptions - guests plus every role, for the roles
// condition of placements; an error is only logged.
func (c *Controller) visibilityRoleOptions(ctx context.Context) []visibilityRoleOption {
	opts := []visibilityRoleOption{{Name: entity.RoleAnonymous, Label: i18n.T(ctx, "Guest (not signed in)")}}
	if c.permissions == nil {
		return opts
	}
	roles, err := c.permissions.Roles(ctx)
	if err != nil {
		c.LogFailure(err, "error listing roles for block visibility")
		return opts
	}
	for _, r := range roles {
		opts = append(opts, visibilityRoleOption{Name: r.Name, Label: r.Label()})
	}
	return opts
}

// visibilityRoleLabels - role name → label, for the blocks list summary.
func (c *Controller) visibilityRoleLabels(ctx context.Context) map[string]string {
	out := map[string]string{}
	for _, o := range c.visibilityRoleOptions(ctx) {
		out[o.Name] = o.Label
	}
	return out
}

var roleNameFormRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// parseVisibilityRoles reads the checked "visibility_roles" into the
// placement's comma-separated roles condition ("" - everyone).
func parseVisibilityRoles(r *http.Request) string {
	return parseFormRoles(r, "visibility_roles")
}

// parseFormRoles reads the checked role names of the form field name
// (well-formed ones only), comma-separated ("" - none).
func parseFormRoles(r *http.Request, name string) string {
	var roles []string
	for _, v := range strings.Split(joinFormList(r, name), ",") {
		if v = strings.TrimSpace(v); roleNameFormRe.MatchString(v) {
			roles = append(roles, v)
		}
	}
	return strings.Join(roles, ",")
}

// parseVisibilityContentTypes reads the "visibility_content_type" form
// value(s) into the placement's comma-separated column (see
// entity.BlockPlacementDef.VisibilityContentTypes); nil when empty.
func parseVisibilityContentTypes(r *http.Request) *string {
	v := joinFormList(r, "visibility_content_type")
	if v == "" {
		return nil
	}
	return &v
}

// joinFormList reads a list form field - repeated checkboxes, or a
// single comma-separated text field - into one deduplicated
// comma-separated string ("" when nothing was given).
func joinFormList(r *http.Request, key string) string {
	_ = r.ParseForm()
	var items []string
	seen := map[string]bool{}
	for _, raw := range r.Form[key] {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" && !seen[t] {
				seen[t] = true
				items = append(items, t)
			}
		}
	}
	return strings.Join(items, ",")
}

// setPlacementVisibilityFromForm sets p's visibility from a placement
// form: the scope (all pages unless homepage, content types or pages), its
// content types or its page list with the show/hide mode, and the roles.
func setPlacementVisibilityFromForm(r *http.Request, p *entity.BlockPlacementDef) {
	p.VisibilityScope = entity.BlockVisibilityScope(r.FormValue("visibility_scope"))
	if p.VisibilityScope != entity.BlockVisibilityHomepage && p.VisibilityScope != entity.BlockVisibilityContentType && p.VisibilityScope != entity.BlockVisibilityPages {
		p.VisibilityScope = entity.BlockVisibilityAll
	}
	p.VisibilityContentType = nil
	if p.VisibilityScope == entity.BlockVisibilityContentType {
		p.VisibilityContentType = parseVisibilityContentTypes(r)
	}
	p.VisibilityPages = ""
	p.VisibilityPagesMode = entity.BlockVisibilityPagesShow
	if p.VisibilityScope == entity.BlockVisibilityPages {
		p.VisibilityPages = strings.TrimSpace(r.FormValue("visibility_pages"))
		if r.FormValue("visibility_pages_mode") == string(entity.BlockVisibilityPagesHide) {
			p.VisibilityPagesMode = entity.BlockVisibilityPagesHide
		}
	}
	p.VisibilityRoles = parseVisibilityRoles(r)
}
