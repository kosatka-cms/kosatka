package service

import (
	"context"
	"html/template"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/paths"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

// TestViewThemeTemplate renders a theme's View template
// (ViewDisplayDef.ThemeTemplate, the fixture's "cards") the way
// DynamicContentController does.
func TestViewThemeTemplate(t *testing.T) {
	zzChdirToProjectRoot(t)
	zzUseFixtureTheme(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	for _, tc := range []struct {
		items []entity.TeaserItem
		want  string
	}{
		{[]entity.TeaserItem{{Node: entity.Node{Title: "Foo Bar", Slug: "articles/foo"}}, {Node: entity.Node{Title: "Baz", Slug: "articles/baz"}, AltTitle: "Baz alt"}},
			`<li><a href="/articles/foo">Foo Bar</a></li><li><a href="/articles/baz">Baz alt</a></li>`},
		{nil, "<li>Nothing here yet</li>"},
	} {
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, "view_cards", map[string]any{"items": tc.items}, "views/cards.html"); err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(strings.Fields(buf.String()), ""); !strings.Contains(got, strings.Join(strings.Fields(tc.want), "")) {
			t.Errorf("missing %q in %s", tc.want, buf.String())
		}
	}
}

// TestViewListBodyThemeHTML - a View page with a theme template shows
// its HTML instead of the field grid (and the grid's own pager).
func TestViewListBodyThemeHTML(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	var buf strings.Builder
	data := map[string]any{"themeListHTML": template.HTML(`<div class="theme-list">x</div>`), "isViewList": true, "showPagination": true}
	if err := svc.RenderNamed(&buf, "view_list_body", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `<div class="theme-list">x</div>`) || strings.Contains(buf.String(), "Add at least one field") {
		t.Errorf("theme list body: %s", buf.String())
	}
}

// TestViewThemeTemplatesWithAllPartials - the controller parses a theme
// View template together with every partials/*.html; that set must parse
// and execute for each of the theme's templates (the fixture's).
func TestViewThemeTemplatesWithAllPartials(t *testing.T) {
	zzChdirToProjectRoot(t)
	t.Run(zzFixtureTheme, func(t *testing.T) { zzUseFixtureTheme(t); zzRenderViewTemplatesWithPartials(t) })
}

// zzRenderViewTemplatesWithPartials executes each views/*.html of the
// active theme with every partials/*.html and no data.
func zzRenderViewTemplatesWithPartials(t *testing.T) {
	t.Helper()
	partials, err := paths.GlobTemplates(filepath.Join("partials", "*.html"))
	if err != nil || len(partials) == 0 {
		t.Fatalf("partials: %v", err)
	}
	views, err := paths.GlobTemplates(filepath.Join("views", "*.html"))
	if err != nil || len(views) == 0 {
		t.Fatalf("view templates: %v %v", views, err)
	}
	for _, view := range views {
		name := strings.TrimSuffix(filepath.Base(view), ".html")
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		files := []string{"views/" + name + ".html", "pagination.html"}
		for _, p := range partials {
			files = append(files, filepath.Join("partials", filepath.Base(p)))
		}
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, "view_"+name, map[string]any{}, files...); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// fakeBlockViewService - RunConfig only (a theme-template View block
// needs nothing else).
type fakeBlockViewService struct {
	views.Service
	nodes entity.Nodes
}

func (f fakeBlockViewService) RunConfig(context.Context, views.Config, views.RunContext) (entity.Nodes, int, error) {
	return f.nodes, len(f.nodes), nil
}

// TestCacheViewBlockThemeTemplate - a View block with a theme template
// is rendered by it in the block cache, like on View pages.
func TestCacheViewBlockThemeTemplate(t *testing.T) {
	zzChdirToProjectRoot(t)
	zzUseFixtureTheme(t)
	ct := &fakeTeaserContentTypes{defs: entity.ContentTypeDefs{{ID: 1, MachineName: "articles", Label: "Articles"}}}
	s := &CacheService{
		viewService:     fakeBlockViewService{nodes: entity.Nodes{{ID: 1, Type: "articles", Title: "Foo Bar", Slug: "articles/foo"}}},
		templateService: NewTemplateService(false),
		teasers:         NewTeaserBuilder(ct, &fakeTeaserTaxonomy{}, fakeTeaserComments{}),
	}
	html, err := s.renderContentViewBlock(t.Context(), views.Config{View: entity.ViewDisplayDef{ID: 1, ThemeTemplate: "cards"}}, viewBlockPart{page: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `class="fixture-cards"`) || !strings.Contains(string(html), `<a href="/articles/foo">Foo Bar</a>`) {
		t.Fatalf("theme template not used:\n%s", html)
	}
}
