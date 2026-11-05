package views

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"path/filepath"

	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// ThemeTemplateExists - whether the theme ctx renders in (paths.ThemeFor)
// has views/<name>.html (ViewDisplayDef.ThemeTemplate).
func ThemeTemplateExists(ctx context.Context, name string) bool {
	p, err := paths.ResolveTemplateIn(paths.ThemeFor(ctx), filepath.Join("views", name+".html"))
	return err == nil && paths.TemplateExists(p)
}

// NamedRenderer - what RenderThemeTemplate renders with (the template
// service's RenderNamed).
type NamedRenderer interface {
	RenderNamed(w io.Writer, blockName string, data any, templateFiles ...string) error
}

// RenderThemeTemplate executes the theme's "view_<name>" with data plus the
// pager (nil - none); the theme's partials (partials/*.html) are
// available to it. Used by View pages, embedded Views and View blocks
// alike. The theme is ctx's (paths.ThemeFor); data names it for the
// renderer too (service.RequestThemeKey).
func RenderThemeTemplate(ctx context.Context, ts NamedRenderer, name string, data map[string]any, pager *pagination.Pagination) (template.HTML, error) {
	data["showPagination"] = pager != nil && pager.TotalPages > 1
	data["pagination"] = pager
	files := []string{"views/" + name + ".html", "pagination.html"}
	partials, _ := paths.GlobTemplatesIn(paths.ThemeFor(ctx), filepath.Join("partials", "*.html"))
	for _, p := range partials {
		files = append(files, filepath.Join("partials", filepath.Base(p)))
	}
	var buf bytes.Buffer
	if err := ts.RenderNamed(&buf, "view_"+name, data, files...); err != nil {
		return "", fmt.Errorf("error rendering theme view template %q: %w", name, err)
	}
	return template.HTML(buf.String()), nil
}
