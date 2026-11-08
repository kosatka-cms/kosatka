package service

import (
	"bytes"
	"context"
	"html/template"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestPublicPageTemplateSet_ParsesWithNewRegionWiring - parse-only smoke
// check that the full public page template set (as wired by
// pageFilesToParse) parses, catching template syntax errors (bad
// dict/region calls) that go build can't.
func TestPublicPageTemplateSet_ParsesWithNewRegionWiring(t *testing.T) {
	zzChdirToProjectRoot(t)
	for _, path := range []string{"dynamic/page.html", "block_demo.html", "admin/blocks/index.html", "dynamic/comment_list.html", "admin/views/index.html", "admin/views/page.html", "admin/content-types/field_edit.html", "admin/content-types/edit.html", "admin/taxonomy/vocabulary_edit.html", "admin/taxonomy/index.html", "admin/taxonomy/terms.html", "user/profile_page.html", "admin/date-formats/index.html", "admin/date-formats/edit.html"} {
		svc := &DefaultTemplateService{cacheEnabled: false, cache: make(map[string]*template.Template)}
		files, err := svc.pageFilesToParse(path, strings.Split(path, "/"))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		// getOrParse - the same parse the real Render does, including the
		// set-bound helpers (renderBlock/renderTeaser/renderSuggestion).
		if _, err := svc.getOrParse(path, nil, func() ([]string, error) { return files, nil }); err != nil {
			t.Fatalf("%s: template set failed to parse: %v", path, err)
		}
	}
}

// zzChdirToProjectRoot - paths.TemplatesDir only checks the project root
// and two levels up, while this package lives three levels deep, so chdir
// to the root for the test's duration (t.Chdir restores it).
func zzChdirToProjectRoot(t *testing.T) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
}

// zzFixtureTheme - the tests' own theme (testdata/themes/fixture): one of
// each thing a theme may override, for the engine's mechanisms
// (zzUseFixtureTheme).
const zzFixtureTheme = "fixture"

// zzUseFixtureTheme points the themes at testdata/themes and activates the
// fixture theme for the test. Call after zzChdirToProjectRoot.
func zzUseFixtureTheme(t *testing.T) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "themes"))
	if err != nil {
		t.Fatal(err)
	}
	paths.SetThemesDir(dir)
	t.Cleanup(func() { paths.SetThemesDir("") })
	zzUseTheme(t, zzFixtureTheme)
}

// zzUseTheme activates theme for the test (paths.SetTheme), restoring
// "no theme" afterwards.
func zzUseTheme(t *testing.T, theme string) {
	t.Helper()
	paths.SetTheme(theme)
	t.Cleanup(func() { paths.SetTheme("") })
}

var zzBlockTemplateFiles = []string{
	"partials/region_blocks.html",
	"blocks/logo.html",
	"blocks/search_form.html",
	"blocks/user_menu.html",
	"blocks/last_comments.html",
	"blocks/tag_cloud.html",
	"blocks/copyright.html",
}

func TestRegionBlocksDispatch_PluginAndCustomPlacements(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	pluginID := entity.PluginBlockSearchForm
	bp := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("header_search"): {{
			Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &pluginID, VisibilityScope: entity.BlockVisibilityAll},
		}},
	}
	ctx := map[string]any{"blockPlacements": bp, "isHomepage": false}
	data := map[string]any{"Region": entity.Region("header_search"), "Ctx": ctx}

	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`id="siteSearch"`)) {
		t.Fatalf("expected search form markup in output, got: %s", buf.String())
	}
}

// TestRegionBlocksDispatch_ViewPlacement - a "view" placement with
// ShowTitle=true renders ViewDisplay.Label as <h2> and its pre-rendered
// content from .Ctx.viewBlockContent by Placement.ID (built by
// CacheService.resolveViewBlockContent, supplied directly here). A nil
// ViewDisplay (deleted View) renders nothing rather than panicking.
func TestRegionBlocksDispatch_ViewPlacement(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	viewDisplay := entity.ViewDisplayDef{ID: 42, Label: "Recent comments"}
	bp := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("sidebar"): {{
			Placement:   entity.BlockPlacementDef{ID: 7, PlacementType: entity.BlockPlacementView, ViewDisplayID: intPtrRegionTest(42), VisibilityScope: entity.BlockVisibilityAll, ShowTitle: true},
			ViewDisplay: &viewDisplay,
		}},
	}
	ctx := map[string]any{
		"blockPlacements":  bp,
		"viewBlockContent": map[int]template.HTML{7: template.HTML("<ul><li>A comment</li></ul>")},
		"isHomepage":       false,
	}
	data := map[string]any{"Region": entity.Region("sidebar"), "Ctx": ctx}

	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("<h2>Recent comments</h2>")) {
		t.Fatalf("expected view label as <h2>, got: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("<li>A comment</li>")) {
		t.Fatalf("expected pre-rendered view block content, got: %s", buf.String())
	}

	// ViewDisplay nil (deleted View) - must not panic, renders nothing.
	bpDangling := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("sidebar"): {{
			Placement: entity.BlockPlacementDef{ID: 8, PlacementType: entity.BlockPlacementView, ViewDisplayID: intPtrRegionTest(99), VisibilityScope: entity.BlockVisibilityAll},
		}},
	}
	data2 := map[string]any{"Region": entity.Region("sidebar"), "Ctx": map[string]any{"blockPlacements": bpDangling, "isHomepage": false}}
	var buf2 bytes.Buffer
	if err := svc.RenderNamed(&buf2, "region_blocks", data2, zzBlockTemplateFiles...); err != nil {
		t.Fatalf("render error with dangling ViewDisplay: %v", err)
	}
	if bytes.Contains(buf2.Bytes(), []byte("<h2>")) {
		t.Fatalf("expected no output for a placement with nil ViewDisplay, got: %s", buf2.String())
	}
}

// TestRegionBlocksDispatch_ViewPlacement_TitleOverrideAndHide - a View
// placement's Placement.Title, when set, overrides ViewDisplay.Label in the
// <h2>; ShowTitle=false hides the heading regardless of Title/Label.
func TestRegionBlocksDispatch_ViewPlacement_TitleOverrideAndHide(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	viewDisplay := entity.ViewDisplayDef{ID: 42, Label: "Recent comments"}
	render := func(title string, showTitle bool) string {
		bp := map[entity.Region][]entity.BlockWithPlacement{
			entity.Region("sidebar"): {{
				Placement:   entity.BlockPlacementDef{ID: 7, PlacementType: entity.BlockPlacementView, ViewDisplayID: intPtrRegionTest(42), VisibilityScope: entity.BlockVisibilityAll, Title: title, ShowTitle: showTitle},
				ViewDisplay: &viewDisplay,
			}},
		}
		ctx := map[string]any{
			"blockPlacements":  bp,
			"viewBlockContent": map[int]template.HTML{7: template.HTML("<ul></ul>")},
			"isHomepage":       false,
		}
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "region_blocks", map[string]any{"Region": entity.Region("sidebar"), "Ctx": ctx}, zzBlockTemplateFiles...); err != nil {
			t.Fatalf("render error: %v", err)
		}
		return buf.String()
	}

	if got := render("Fresh discussions", true); !strings.Contains(got, "<h2>Fresh discussions</h2>") {
		t.Fatalf("expected Placement.Title to override ViewDisplay.Label, got: %s", got)
	}
	if got := render("", true); !strings.Contains(got, "<h2>Recent comments</h2>") {
		t.Fatalf("expected empty Title to fall back to ViewDisplay.Label, got: %s", got)
	}
	if got := render("Fresh discussions", false); strings.Contains(got, "<h2>") {
		t.Fatalf("expected ShowTitle=false to hide the heading regardless of Title, got: %s", got)
	}
}

func intPtrRegionTest(v int) *int { return &v }

// zzHeaderNavData - a "menu" placement in header_nav for a 2-level tree
// (MenuStartLevel=1, MenuLevelsToShow=2), as region_blocks gets it.
func zzHeaderNavData() map[string]any {
	menu := entity.MenuDef{ID: 1, MachineName: "main", Label: "Main menu"}
	tree := []MenuTreeItem{
		{
			Title: "Home", URL: "/", First: true,
			HasChildren: true,
			Children:    []MenuTreeItem{{Title: "Subsection", URL: "/sub", First: true, Last: true}},
		},
		{Title: "About us", URL: "/about", Last: true},
	}
	bp := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("header_nav"): {{
			Placement: entity.BlockPlacementDef{ID: 20, PlacementType: entity.BlockPlacementMenu, MenuID: intPtrRegionTest(1), MenuStartLevel: 1, MenuLevelsToShow: 2, VisibilityScope: entity.BlockVisibilityAll},
			Menu:      &menu,
		}},
	}
	ctx := map[string]any{
		"blockPlacements":    bp,
		"menuTreesAnnotated": map[string][]MenuTreeItem{"main": tree},
		"isHomepage":         false,
	}
	return map[string]any{"Region": entity.Region("header_nav"), "Ctx": ctx}
}

// zzRenderHeaderNav renders zzHeaderNavData with the active theme's
// header.html (its menu_block_header_nav define).
func zzRenderHeaderNav(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := NewTemplateService(false).RenderNamed(&buf, "region_blocks", zzHeaderNavData(), append(append([]string{}, zzBlockTemplateFiles...), "header.html")...); err != nil {
		t.Fatalf("render error: %v", err)
	}
	return buf.String()
}

// TestRegionBlocksDispatch_MenuPlacement_HeaderNav - a menu placed in
// header_nav renders through the theme's menu_block_header_nav (the
// fixture's), the generic menu block without one; MenuLevelsToShow caps
// the tree's depth either way.
func TestRegionBlocksDispatch_MenuPlacement_HeaderNav(t *testing.T) {
	zzChdirToProjectRoot(t)

	// Without a theme the engine has no menu_block_header_nav - the
	// generic menu block.
	var plain bytes.Buffer
	if err := NewTemplateService(false).RenderNamed(&plain, "region_blocks", zzHeaderNavData(), zzBlockTemplateFiles...); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(plain.String(), `<nav class="block block-menu"`) || strings.Contains(plain.String(), `fixture-nav`) {
		t.Fatalf("engine: expected the generic menu block, got: %s", plain.String())
	}

	zzUseFixtureTheme(t)
	out := zzRenderHeaderNav(t)
	for _, want := range []string{`<nav class="fixture-nav" aria-label="Main menu">`, `<ul class="menu ul-depth-0">`, `<ul class="menu ul-depth-1">`, "Home", "Subsection", "About us"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in header_nav output, got: %s", want, out)
		}
	}
	if strings.Contains(out, "block-menu") {
		t.Fatalf("the theme's menu block replaces the generic one: %s", out)
	}
	// MenuLevelsToShow=2 caps recursion at 2 levels - a 3rd level must never
	// appear; this asserts the cap itself, not just the fixture's depth.
	if strings.Contains(out, "ul-depth-2") {
		t.Fatalf("expected no ul-depth-2 (MenuLevelsToShow=2 caps at depth 1), got: %s", out)
	}
}

// TestRegionBlocksDispatch_MenuPlacement_GenericRegion - a menu placed
// outside header_nav gets the generic <nav class="block block-menu">
// wrapper, labelled with the menu's name, with an <h2> only when the
// placement shows its title; never the header_nav-specific markup (see
// region_blocks.html).
func TestRegionBlocksDispatch_MenuPlacement_GenericRegion(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	menu := entity.MenuDef{ID: 2, MachineName: "footer", Label: "Footer menu"}
	tree := []MenuTreeItem{{Title: "Item", URL: "/x", First: true, Last: true}}
	bp := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("sidebar"): {{
			Placement: entity.BlockPlacementDef{ID: 21, PlacementType: entity.BlockPlacementMenu, MenuID: intPtrRegionTest(2), MenuStartLevel: 1, MenuLevelsToShow: 0, VisibilityScope: entity.BlockVisibilityAll},
			Menu:      &menu,
		}},
	}
	ctx := map[string]any{
		"blockPlacements":    bp,
		"menuTreesAnnotated": map[string][]MenuTreeItem{"footer": tree},
		"isHomepage":         false,
	}
	data := map[string]any{"Region": entity.Region("sidebar"), "Ctx": ctx}

	render := func() string {
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
			t.Fatalf("render error: %v", err)
		}
		return buf.String()
	}
	out := render()
	if !strings.Contains(out, `<nav class="block block-menu" aria-label="Footer menu">`) || strings.Contains(out, "<h2>") {
		t.Fatalf("expected the generic menu block, labelled, without a heading, got: %s", out)
	}
	bp[entity.Region("sidebar")][0].Placement.ShowTitle = true
	if titled := render(); !strings.Contains(titled, "<h2>Footer menu</h2>") {
		t.Fatalf("expected the menu's name as <h2> with ShowTitle, got: %s", titled)
	}
	if strings.Contains(out, `id="menu-wrapper"`) || strings.Contains(out, `id="main-menu"`) {
		t.Fatalf("expected NO header-nav-specific ids outside header_nav, got: %s", out)
	}
}

// TestRegionBlocksDispatch_MenuPlacement_NilMenu - a placement whose Menu is
// nil (MenuDef deleted after placement) renders nothing instead of
// panicking.
func TestRegionBlocksDispatch_MenuPlacement_NilMenu(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	bp := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("sidebar"): {{
			Placement: entity.BlockPlacementDef{ID: 22, PlacementType: entity.BlockPlacementMenu, MenuID: intPtrRegionTest(99), VisibilityScope: entity.BlockVisibilityAll},
		}},
	}
	data := map[string]any{"Region": entity.Region("sidebar"), "Ctx": map[string]any{"blockPlacements": bp, "isHomepage": false}}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
		t.Fatalf("render error with dangling Menu: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("expected no output for a placement with nil Menu, got: %s", buf.String())
	}
}

// TestMenuTreeItems_RecursionDepthCap exercises "menu_tree_items" directly
// on a synthetic 3-level tree. Remaining=-1 (unlimited) renders all 3
// levels; Remaining=2 stops after 2; ul-depth-N collapses to ul-depth-1 for
// every N>=1 (see region_blocks.html).
func TestMenuTreeItems_RecursionDepthCap(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	tree := []MenuTreeItem{
		{
			Title: "L1", HasChildren: true, First: true, Last: true,
			Children: []MenuTreeItem{
				{
					Title: "L2", HasChildren: true, First: true, Last: true,
					Children: []MenuTreeItem{
						{Title: "L3", First: true, Last: true},
					},
				},
			},
		},
	}

	t.Run("unlimited renders all 3 levels", func(t *testing.T) {
		var buf bytes.Buffer
		data := map[string]any{"Items": tree, "Remaining": -1, "Depth": 0}
		if err := svc.RenderNamed(&buf, "menu_tree_items", data, "partials/region_blocks.html"); err != nil {
			t.Fatalf("render error: %v", err)
		}
		out := buf.String()
		for _, want := range []string{"L1", "L2", "L3", "ul-depth-0", "ul-depth-1"} {
			if !strings.Contains(out, want) {
				t.Fatalf("expected %q in output, got: %s", want, out)
			}
		}
		// depth 2 (L3's own <ul> level, were there one) still renders
		// as ul-depth-1, not ul-depth-2 - the collapse this test locks in.
		if strings.Contains(out, "ul-depth-2") {
			t.Fatalf("expected ul-depth-N to collapse to ul-depth-1 for N>=1, got: %s", out)
		}
	})

	t.Run("Remaining=2 stops recursion after 2 levels", func(t *testing.T) {
		var buf bytes.Buffer
		data := map[string]any{"Items": tree, "Remaining": 2, "Depth": 0}
		if err := svc.RenderNamed(&buf, "menu_tree_items", data, "partials/region_blocks.html"); err != nil {
			t.Fatalf("render error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "L1") || !strings.Contains(out, "L2") {
			t.Fatalf("expected L1/L2 to render, got: %s", out)
		}
		if strings.Contains(out, "L3") {
			t.Fatalf("expected L3 NOT to render (Remaining=2 caps at 2 levels), got: %s", out)
		}
	})
}

func TestRegionBlocksDispatch_CustomBlockTitleGuard(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	// ShowTitle=false must not render <h2>, even with a non-empty Title (Title
	// is always set - it is the admin identifier in the blocks list).
	bpEmpty := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("footer"): {{
			Block:     entity.BlockDef{Title: "Menu (footer)", ShowTitle: false, Body: "<p>footer content</p>"},
			Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementCustom, VisibilityScope: entity.BlockVisibilityAll},
		}},
	}
	data := map[string]any{"Region": entity.Region("footer"), "Ctx": map[string]any{"blockPlacements": bpEmpty, "isHomepage": false}}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("<h2>")) {
		t.Fatalf("expected no <h2> for a block with ShowTitle=false, got: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("footer content")) {
		t.Fatalf("expected block body to render, got: %s", buf.String())
	}

	// ShowTitle=true must render <h2>.
	bpTitled := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("sidebar"): {{
			Block:     entity.BlockDef{Title: "Donate", ShowTitle: true, Body: "<p>x</p>"},
			Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementCustom, VisibilityScope: entity.BlockVisibilityAll},
		}},
	}
	data2 := map[string]any{"Region": entity.Region("sidebar"), "Ctx": map[string]any{"blockPlacements": bpTitled, "isHomepage": false}}
	var buf2 bytes.Buffer
	if err := svc.RenderNamed(&buf2, "region_blocks", data2, zzBlockTemplateFiles...); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !bytes.Contains(buf2.Bytes(), []byte("<h2>Donate</h2>")) {
		t.Fatalf("expected <h2>Donate</h2> for a titled block, got: %s", buf2.String())
	}
}

func TestRegionBlocksDispatch_DemoModeOverlay(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	data := map[string]any{
		"Region": entity.Region("footer"),
		"Ctx": map[string]any{
			"blockPlacements": map[entity.Region][]entity.BlockWithPlacement{},
			"isHomepage":      false,
			"blockDemoMode":   true,
			"regionLabels":    map[entity.Region]string{entity.Region("footer"): "Site footer"},
		},
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("block-region-demo")) {
		t.Fatalf("expected demo overlay markup, got: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("footer")) {
		t.Fatalf("expected region machine name in demo label, got: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Site footer")) {
		t.Fatalf("expected region label in demo label, got: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("empty")) {
		t.Fatalf("expected empty-region marker, got: %s", out)
	}
}

// TestAdminBlocksListRendersPluginRows exercises admin/blocks/index.html's
// "content" block with a custom and a plugin placement - covers the
// system block branch and the PluginIDValue()-keyed pluginBlockLabels
// lookup.

// testCommentListItem mirrors interfaces.commentListItem's field names
// (Comment/Fields/Values) so comment_list.html can be exercised without
// importing the interfaces package - html/template resolves fields by name,
// so an identically-shaped local type is enough.
type testCommentListItem struct {
	Comment entity.Comment
	Fields  entity.FieldInstanceDefs
	Values  map[int]string
	Dates   map[int]time.Time
}

func commentListTestFixture() ([]testCommentListItem, entity.FieldInstanceDefs) {
	fields := entity.FieldInstanceDefs{
		{ID: 1, Label: "Subject", FieldType: entity.FieldTypeText},
		{ID: 2, Label: "Author", FieldType: entity.FieldTypeText},
		{ID: 3, Label: "Date", FieldType: entity.FieldTypeDate},
	}
	items := []testCommentListItem{
		{
			Comment: entity.Comment{ID: 1},
			Fields:  fields,
			Values:  map[int]string{1: "Re: test", 2: "ishmael", 3: "27.09.2026 12:00"},
		},
		{
			Comment: entity.Comment{ID: 2},
			Fields:  fields,
			Values:  map[int]string{1: "no subject", 2: "Guest", 3: "27.09.2026 13:00"},
		},
	}
	return items, fields
}

// TestCommentListTemplate_TableMode exercises comment_list.html's table
// branch (displayStyle=="table") - one <th> per listFieldsHead, one <td>
// per item field, first column linked to the comment.
func TestCommentListTemplate_TableMode(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	items, fields := commentListTestFixture()
	data := map[string]any{
		"items":          items,
		"listFieldsHead": fields,
		"displayStyle":   "table",
		"showPagination": true,
		"pagination":     pagination.New(1, 30, 30, "/comments-view", nil),
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "dynamic/comment_list.html", "pagination.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"<th>Subject</th>", "<th>Author</th>", "<th>Date</th>", "Re: test", "ishmael", "27.09.2026 12:00", `<a href="/comment/1#comment-1">`} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Fatalf("expected %q in table output, got: %s", want, out)
		}
	}
}

// TestCommentListTemplate_ListMode exercises the list branch
// (displayStyle=="list") - first field as the linked title, remaining
// fields inline.
func TestCommentListTemplate_ListMode(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	items, fields := commentListTestFixture()
	data := map[string]any{
		"items":          items,
		"listFieldsHead": fields,
		"displayStyle":   "list",
		"showPagination": false,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "dynamic/comment_list.html", "pagination.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`<a href="/comment/1#comment-1">`, "Re: test", "ishmael", "no subject", "Guest"} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Fatalf("expected %q in list output, got: %s", want, out)
		}
	}
	if bytes.Contains(buf.Bytes(), []byte("list-grid-item")) {
		t.Fatalf("expected no card markup in list mode, got: %s", out)
	}
}

// TestCommentListTemplate_GridMode exercises the grid branch
// (displayStyle=="grid") - same fields as list/table in dynamic/list.html's
// .list-grid/.list-grid-item card markup, as for Node views.
func TestCommentListTemplate_GridMode(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	items, fields := commentListTestFixture()
	data := map[string]any{
		"items":          items,
		"listFieldsHead": fields,
		"displayStyle":   "grid",
		"showPagination": false,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "dynamic/comment_list.html", "pagination.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"list-grid-item", "item-field-title", `<a href="/comment/1#comment-1">`, "Re: test", "ishmael", "27.09.2026 12:00"} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Fatalf("expected %q in grid output, got: %s", want, out)
		}
	}
}

// TestCommentListTemplate_NoFieldsPlaceholder - a Comment view with no
// fields configured (listFieldsHead empty) shows the "add a field" hint
// instead of panicking or rendering nothing, as dynamic/list.html does.
func TestCommentListTemplate_NoFieldsPlaceholder(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	data := map[string]any{
		"items":          []testCommentListItem{},
		"listFieldsHead": entity.FieldInstanceDefs(nil),
		"displayStyle":   "table",
		"showPagination": false,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "dynamic/comment_list.html", "pagination.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Add at least one field")) {
		t.Fatalf("expected the no-fields placeholder, got: %s", buf.String())
	}
}

func TestAdminBlocksListRendersPluginRows(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	pluginID := entity.PluginBlockSearchForm
	blocks := []entity.BlockWithPlacement{
		{
			Block:     entity.BlockDef{ID: 1, Title: "Donate", Status: true},
			Placement: entity.BlockPlacementDef{ID: 10, PlacementType: entity.BlockPlacementCustom, Region: entity.Region("sidebar"), VisibilityScope: entity.BlockVisibilityAll},
		},
		{
			Placement: entity.BlockPlacementDef{ID: 11, PlacementType: entity.BlockPlacementPlugin, PluginID: &pluginID, Region: entity.Region("header_search"), VisibilityScope: entity.BlockVisibilityAll},
		},
	}
	// The controller passes the labels translated (i18n.N sources).
	pluginLabels := make(map[entity.PluginBlockID]string, len(entity.PluginBlockLabels()))
	for id, l := range entity.PluginBlockLabels() {
		pluginLabels[id] = i18n.T(t.Context(), l)
	}
	data := map[string]any{
		"title":                  "Blocks",
		"allRegions":             zzFixtureThemeRegions(t).RegionIDs(),
		"defaultRegion":          zzFixtureThemeRegions(t).DefaultRegion,
		"allRegionsWithDisabled": zzFixtureThemeRegions(t).RegionIDsWithDisabled(),
		"regionLabels":           map[entity.Region]string{entity.Region("sidebar"): "Sidebar", entity.Region("header_search"): "Search (header)", entity.RegionDisabled: "Hidden"},
		"allPluginBlocks":        entity.PluginBlocks(),
		"pluginBlockLabels":      pluginLabels,
		"blocks":                 blocks,
		"blocksByRegion":         zzGroupBlocksByRegion(blocks),
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "admin/blocks/index.html", "admin/alerts.html", "admin/icons.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Donate")) {
		t.Fatalf("expected custom block title to render, got: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Search the site")) || !bytes.Contains(buf.Bytes(), []byte("(system)")) {
		t.Fatalf("expected plugin block label + system marker, got: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`/admin/config/blocks/placement/11/delete`)) {
		t.Fatalf("expected plugin placement delete form action, got: %s", out)
	}
}

// zzGroupBlocksByRegion mirrors admin.Controller.BlockList's grouping step
// (internal/interfaces/admin/blocks.go, BlockList) to build data["blocksByRegion"]
// without importing the interfaces package (service tests must not depend
// on interfaces).
func zzGroupBlocksByRegion(blocks []entity.BlockWithPlacement) map[entity.Region][]entity.BlockWithPlacement {
	byRegion := make(map[entity.Region][]entity.BlockWithPlacement)
	for _, b := range blocks {
		byRegion[b.Placement.Region] = append(byRegion[b.Placement.Region], b)
	}
	return byRegion
}

// TestAdminBlocksListRendersZones - admin/blocks/index.html renders a labeled zone
// per region, an empty-state placeholder for empty regions, and placements
// with Region == entity.RegionDisabled inside the hidden zone (RegionDisabled
// is absent from .allRegions, so they must come via
// .allRegionsWithDisabled/.blocksByRegion).
func TestAdminBlocksListRendersZones(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	blocks := []entity.BlockWithPlacement{
		{
			Block:     entity.BlockDef{ID: 1, Title: "Donate", Status: true},
			Placement: entity.BlockPlacementDef{ID: 10, PlacementType: entity.BlockPlacementCustom, Region: entity.Region("sidebar"), VisibilityScope: entity.BlockVisibilityAll},
		},
		{
			Block:     entity.BlockDef{ID: 2, Title: "Old ad", Status: false},
			Placement: entity.BlockPlacementDef{ID: 12, PlacementType: entity.BlockPlacementCustom, Region: entity.RegionDisabled, VisibilityScope: entity.BlockVisibilityAll},
		},
	}
	data := map[string]any{
		"title":                  "Blocks",
		"allRegions":             zzFixtureThemeRegions(t).RegionIDs(),
		"defaultRegion":          zzFixtureThemeRegions(t).DefaultRegion,
		"allRegionsWithDisabled": zzFixtureThemeRegions(t).RegionIDsWithDisabled(),
		"regionLabels":           regionLabelsForTest,
		"allPluginBlocks":        entity.PluginBlocks(),
		"pluginBlockLabels":      entity.PluginBlockLabels(),
		"blocks":                 blocks,
		"blocksByRegion":         zzGroupBlocksByRegion(blocks),
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "admin/blocks/index.html", "admin/alerts.html", "admin/icons.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`data-region="sidebar"`,
		`data-region="disabled"`,
		"Hidden",
		"Old ad",
	} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Fatalf("expected %q in output, got: %s", want, out)
		}
	}

	// header_search has no placements here - its zone must still render (an
	// always-present drop target) with the empty-state placeholder.
	if !bytes.Contains(buf.Bytes(), []byte(`data-region="header_search"`)) || !bytes.Contains(buf.Bytes(), []byte("- empty")) {
		t.Fatalf("expected an empty region's zone to still render with a placeholder, got: %s", out)
	}
}

// zzFixtureThemeRegions - the fixture theme's theme.yaml regions
// (activating the theme for the test): the region catalog the admin block
// pages list. Call after zzChdirToProjectRoot.
func zzFixtureThemeRegions(t *testing.T) entity.ThemeInfo {
	t.Helper()
	zzUseFixtureTheme(t)
	info, err := themes.LoadInfo()
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// regionLabelsForTest - a full regionLabels map (all theme regions plus
// the hidden zone), unlike TestAdminBlocksListRendersPluginRows' partial
// map.
var regionLabelsForTest = map[entity.Region]string{
	entity.Region("sidebar"):         "Sidebar",
	entity.Region("header_search"):   "Search (header)",
	entity.Region("header_usermenu"): "User menu (header)",
	entity.Region("header_nav"):      "Navigation (header)",
	entity.Region("footer"):          "Footer",
	entity.RegionDisabled:            "Hidden",
}

func TestPluginBlockDispatch_UserMenuAndLastComments(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	userMenuID := entity.PluginBlockUserMenu
	bp := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("header_usermenu"): {{
			Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &userMenuID, VisibilityScope: entity.BlockVisibilityAll},
		}},
	}
	// The visitor's own menu, rendered by the server: a guest's links, a
	// user's dropdown - never both.
	renderMenu := func(user *entity.User) string {
		t.Helper()
		ctx := map[string]any{"blockPlacements": bp, "isHomepage": false}
		if user != nil {
			ctx["user"] = user
		}
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "region_blocks", map[string]any{"Region": entity.Region("header_usermenu"), "Ctx": ctx}, zzBlockTemplateFiles...); err != nil {
			t.Fatalf("render error: %v", err)
		}
		return buf.String()
	}
	if out := renderMenu(nil); !strings.Contains(out, `href="/login"`) || strings.Contains(out, `id="user-menu-toggle"`) {
		t.Fatalf("a guest's menu: %s", out)
	}
	if out := renderMenu(&entity.User{ID: 3, Username: "ann"}); !strings.Contains(out, `id="user-menu-toggle"`) || !strings.Contains(out, `<span class="user-name">ann</span>`) || strings.Contains(out, `href="/login"`) {
		t.Fatalf("a user's menu: %s", out)
	}

	// The latest comments: this plugin block renders only its own column
	// (web/templates/blocks/last_comments.html); its heading is the
	// placement's, as every system block's - "Show title", the title or
	// else the block's name - once.
	lastCommentsID := entity.PluginBlockLastComments
	renderComments := func(title string, show bool) string {
		t.Helper()
		bp2 := map[entity.Region][]entity.BlockWithPlacement{
			entity.Region("widgets_col_1"): {{
				Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &lastCommentsID, VisibilityScope: entity.BlockVisibilityAll, Title: title, ShowTitle: show},
			}},
		}
		data2 := map[string]any{
			"Region": entity.Region("widgets_col_1"),
			"Ctx": map[string]any{
				"blockPlacements": bp2,
				"isHomepage":      false,
				"lastComments":    entity.Comments{{ID: 5, Content: "<p>Great issue</p>", Node: &entity.Node{ID: 1, Title: "The node"}}},
			},
		}
		var buf2 bytes.Buffer
		if err := svc.RenderNamed(&buf2, "region_blocks", data2, zzBlockTemplateFiles...); err != nil {
			t.Fatalf("render error: %v", err)
		}
		return buf2.String()
	}
	out := renderComments("", false)
	// Each comment's text linking it.
	if !strings.Contains(out, `<a href="/comment/5#comment-5">Great issue</a>`) {
		t.Errorf("a latest comment: %s", out)
	}
	if strings.Contains(out, "<h2") {
		t.Errorf("without Show title, no heading: %s", out)
	}
	if out := renderComments("", true); strings.Count(out, "<h2") != 1 || !strings.Contains(out, "<h2>Recent comments</h2>") {
		t.Errorf("Show title without a title - the block's name, once: %s", out)
	}
	if out := renderComments("Talked about", true); strings.Count(out, "<h2") != 1 || !strings.Contains(out, "<h2>Talked about</h2>") {
		t.Errorf("Show title - the placement's title, once: %s", out)
	}
}

// TestPluginBlockDispatch_ModuleBlock - a module's plugin block
// ("error_report_hint") renders from the module's own block template,
// with no data of its own.
func TestPluginBlockDispatch_ModuleBlock(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	pluginID := entity.PluginBlockID("error_report_hint")
	bp := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("content_bottom"): {{
			Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &pluginID, VisibilityScope: entity.BlockVisibilityAll},
		}},
	}
	data := map[string]any{
		"Region": entity.Region("content_bottom"),
		"Ctx":    map[string]any{"blockPlacements": bp, "isHomepage": false},
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`class="error-report-hint`)) {
		t.Fatalf("expected the module's block markup, got: %s", buf.String())
	}
}

// TestPluginBlockDispatch_Logo - the "logo" plugin block
// (entity.PluginBlockLogo) renders #logo-top/#logo-img from
// .Ctx.siteLogoURI/.Ctx.siteName as resolved by GetCommonDataWithUser, with
// no fallback of its own; covers both a default and a custom logo.
func TestPluginBlockDispatch_Logo(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	pluginID := entity.PluginBlockLogo
	render := func(logoURI, siteName string) string {
		bp := map[entity.Region][]entity.BlockWithPlacement{
			entity.Region("header_logo"): {{
				Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &pluginID, VisibilityScope: entity.BlockVisibilityAll},
			}},
		}
		data := map[string]any{
			"Region": entity.Region("header_logo"),
			"Ctx":    map[string]any{"blockPlacements": bp, "isHomepage": false, "siteLogoURI": logoURI, "siteName": siteName},
		}
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
			t.Fatalf("render error: %v", err)
		}
		return buf.String()
	}

	seededOut := render("/static/images/logo.png", "Example Site")
	for _, want := range []string{
		`id="logo-top"`,
		`<a href="/" title="">`,
		`id="logo-img" src="/static/images/logo.png" alt="Example Site"`,
	} {
		if !strings.Contains(seededOut, want) {
			t.Fatalf("seeded logo: expected %q in output, got: %s", want, seededOut)
		}
	}

	customOut := render("/uploads/site/logo/1-123.png", "Custom Site")
	if !strings.Contains(customOut, `src="/uploads/site/logo/1-123.png" alt="Custom Site"`) {
		t.Fatalf("custom logo: expected uploaded logo URL/site name in output, got: %s", customOut)
	}

	// No hardcoded fallback: an empty siteLogoURI (logo removed) renders
	// nothing.
	emptyOut := render("", "Example Site")
	if strings.Contains(emptyOut, `id="logo-top"`) {
		t.Fatalf("empty siteLogoURI: expected no logo markup at all, got: %s", emptyOut)
	}
}

// TestRegionBlocksStandalone_LogoPlacementDoesNotBreakIt - region_blocks
// rendered on its own (RenderNamed, e.g. GET /api/block-placement/{id} for
// a lazy placement) needs every plugin block partial region_blocks.html
// can dispatch to, or html/template fails with "no such template".
// RegionBlocksStandaloneFiles is the list every such caller passes.
func TestRegionBlocksStandalone_LogoPlacementDoesNotBreakIt(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)
	data := map[string]any{
		"Region": entity.Region("main_bottom"),
		"Ctx":    map[string]any{"blockPlacements": map[entity.Region][]entity.BlockWithPlacement{}, "isHomepage": true},
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "region_blocks", data, RegionBlocksStandaloneFiles...); err != nil {
		t.Fatalf("render error (RegionBlocksStandaloneFiles must list every plugin block region_blocks.html can dispatch to): %v", err)
	}
}

// TestLazyPlacementRender - a lazy placement renders as a placeholder
// place pointing at /api/block-placement/{id} with the page's path,
// holding the theme's block_placeholder_<region> (none in the engine);
// a hidden one renders nothing; LazyPlacement hands the endpoint the
// same placement with Lazy off and visibility "all", which renders the
// content.
func TestLazyPlacementRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	vd := entity.ViewDisplayDef{ID: 116, Label: "Premieres"}
	lazy := entity.BlockWithPlacement{
		Placement: entity.BlockPlacementDef{
			ID: 22, Region: entity.Region("main_bottom"), PlacementType: entity.BlockPlacementView,
			VisibilityScope: entity.BlockVisibilityPages, VisibilityPages: "coming-soon", VisibilityPagesMode: entity.BlockVisibilityPagesHide,
			Lazy: true,
		},
		ViewDisplay: &vd,
	}
	placements := map[entity.Region][]entity.BlockWithPlacement{entity.Region("main_bottom"): {lazy}}
	files := append([]string{"partials/block_placeholders.html"}, RegionBlocksStandaloneFiles...)
	render := func(t *testing.T, items map[entity.Region][]entity.BlockWithPlacement, path string) string {
		t.Helper()
		svc := NewTemplateService(false)
		data := map[string]any{"Region": entity.Region("main_bottom"), "Ctx": map[string]any{
			"blockPlacements":  items,
			"viewBlockContent": map[int]template.HTML{22: "<p>cards</p>"},
			"currentPath":      path,
		}}
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "region_blocks", data, files...); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}

	out := render(t, placements, "/movies")
	if !strings.Contains(out, `data-lazy-block="/api/block-placement/22?path=%2Fmovies"`) || strings.Contains(out, "cards") || strings.Contains(out, "skeleton") {
		t.Fatalf("engine: lazy placeholder without a theme placeholder:\n%s", out)
	}
	if out := render(t, placements, "/coming-soon"); strings.Contains(out, "data-lazy-block") {
		t.Fatalf("hidden on this page - no placeholder:\n%s", out)
	}

	zzUseFixtureTheme(t)
	if out := render(t, placements, "/movies"); !strings.Contains(out, "fixture-skeleton") {
		t.Fatalf("theme placeholder missing:\n%s", out)
	}

	item, ok := LazyPlacement(placements, 22)
	if !ok || item.Placement.Lazy || item.Placement.VisibilityScope != entity.BlockVisibilityAll {
		t.Fatalf("LazyPlacement = %+v, %v", item.Placement, ok)
	}
	if lazy := placements[entity.Region("main_bottom")][0].Placement; !lazy.Lazy {
		t.Fatal("LazyPlacement must not change the cached placement")
	}
	if out := render(t, map[entity.Region][]entity.BlockWithPlacement{entity.Region("main_bottom"): {item}}, ""); !strings.Contains(out, "cards") || strings.Contains(out, "data-lazy-block") {
		t.Fatalf("endpoint render:\n%s", out)
	}
	if _, ok := LazyPlacement(placements, 23); ok {
		t.Fatal("unknown id")
	}
	notLazy := map[entity.Region][]entity.BlockWithPlacement{entity.Region("main_bottom"): {{Placement: entity.BlockPlacementDef{ID: 5, Region: entity.Region("main_bottom")}}}}
	if _, ok := LazyPlacement(notLazy, 5); ok {
		t.Fatal("a placement that isn't lazy is not served")
	}
}

// TestPluginBlockDispatch_ShowTitle - a plugin placement's Title/ShowTitle
// (on BlockPlacementDef; plugin placements have no BlockDef) render an <h2>
// before the plugin's markup only when ShowTitle is true.
func TestPluginBlockDispatch_ShowTitle(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	pluginID := entity.PluginBlockSearchForm
	render := func(showTitle bool) string {
		bp := map[entity.Region][]entity.BlockWithPlacement{
			entity.Region("header_search"): {{
				Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &pluginID, VisibilityScope: entity.BlockVisibilityAll, Title: "Search the site", ShowTitle: showTitle},
			}},
		}
		data := map[string]any{
			"Region": entity.Region("header_search"),
			"Ctx":    map[string]any{"blockPlacements": bp, "isHomepage": false},
		}
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
			t.Fatalf("render error: %v", err)
		}
		return buf.String()
	}

	if got := render(true); !strings.Contains(got, "<h2>Search the site</h2>") {
		t.Fatalf("expected ShowTitle=true to render the <h2>, got: %s", got)
	}
	if got := render(false); strings.Contains(got, "<h2>Search the site</h2>") {
		t.Fatalf("expected ShowTitle=false to NOT render the <h2>, got: %s", got)
	}
}

// TestRegionBlocksDispatch_PagesVisibility - the classic "Pages"
// visibility condition (entity.BlockVisibilityPages), evaluated
// against .Ctx.currentPath via service.MatchesVisibilityPages.
func TestRegionBlocksDispatch_PagesVisibility(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	pluginID := entity.PluginBlockSearchForm
	render := func(mode entity.BlockVisibilityPagesMode, currentPath string) string {
		bp := map[entity.Region][]entity.BlockWithPlacement{
			entity.Region("header_search"): {{
				Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &pluginID, VisibilityScope: entity.BlockVisibilityPages, VisibilityPages: "news", VisibilityPagesMode: mode},
			}},
		}
		data := map[string]any{
			"Region": entity.Region("header_search"),
			"Ctx":    map[string]any{"blockPlacements": bp, "isHomepage": false, "currentPath": currentPath},
		}
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
			t.Fatalf("render error: %v", err)
		}
		return buf.String()
	}

	if got := render(entity.BlockVisibilityPagesShow, "news"); !strings.Contains(got, `id="siteSearch"`) {
		t.Fatalf("show mode on a listed page: expected the block to render, got: %s", got)
	}
	if got := render(entity.BlockVisibilityPagesShow, "forum"); strings.Contains(got, `id="siteSearch"`) {
		t.Fatalf("show mode off a listed page: expected the block to be hidden, got: %s", got)
	}
	if got := render(entity.BlockVisibilityPagesHide, "news"); strings.Contains(got, `id="siteSearch"`) {
		t.Fatalf("hide mode on a listed page: expected the block to be hidden, got: %s", got)
	}
	if got := render(entity.BlockVisibilityPagesHide, "forum"); !strings.Contains(got, `id="siteSearch"`) {
		t.Fatalf("hide mode off a listed page: expected the block to render, got: %s", got)
	}
}

// TestContentListTemplate_TitleFieldLinksToNode exercises
// CacheService.renderContentViewBlock's render path -
// views.BuildContentFieldInstances/views.BuildContentListItems feeding dynamic/list.html's
// "content" define. A missing views.ContentListItem field the template uses
// would only surface as an execution error, not a compile error.
func TestContentListTemplate_TitleFieldLinksToNode(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	viewFields := entity.ViewFieldDefs{
		{ID: 1, BuiltinField: entity.ViewFieldBuiltinTitle, LinkTo: entity.ViewFieldLinkContent},
	}
	listFields := views.BuildContentFieldInstances(context.Background(), viewFields, nil, nil, nil)
	nodes := entity.Nodes{
		{ID: 1, Title: "Who would win if", Slug: "kto-pobedit-esli"},
		{ID: 2, Title: "Second topic", Slug: "vtoroy-topik"},
	}
	items, err := views.BuildContentListItems(context.Background(), nil, nodes, listFields, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := map[string]any{
		"items":          items,
		"listFieldsHead": listFields,
		"displayStyle":   "list",
		"showPagination": false,
		"isViewList":     true,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`<a href="/kto-pobedit-esli" class="item-field-link">`,
		"Who would win if",
		`<a href="/vtoroy-topik" class="item-field-link">`,
	} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Fatalf("expected %q in output, got: %s", want, out)
		}
	}
	// isViewList suppresses list.html's automatic title link - the only link
	// must come from the Title field's LinkTo=content.
	if bytes.Contains(buf.Bytes(), []byte(`class="item-link"`)) {
		t.Fatalf("expected no auto item-link markup when isViewList is set, got: %s", out)
	}
}

// TestContentListTemplate_ModuleNodeLinkGetsPathPrefix - LinkTo=content
// must use entity.NodeRelPath: a module's node type with a PathPrefix
// stores its slugs without the prefix, so a bare "/"+Node.Slug link would
// be broken.
func TestContentListTemplate_ModuleNodeLinkGetsPathPrefix(t *testing.T) {
	zzChdirToProjectRoot(t)
	withExtraModules(t, entity.Module{Name: "boards", NodeTypes: []entity.ModuleNodeType{{Name: "board_topic", PathPrefix: "boards/"}}})
	svc := NewTemplateService(false)

	viewFields := entity.ViewFieldDefs{
		{ID: 1, BuiltinField: entity.ViewFieldBuiltinTitle, LinkTo: entity.ViewFieldLinkContent},
	}
	listFields := views.BuildContentFieldInstances(context.Background(), viewFields, nil, nil, nil)
	nodes := entity.Nodes{
		{ID: 1, Type: entity.ContentType("board_topic"), Title: "Spam topic", Slug: "24bba67"},
	}
	items, err := views.BuildContentListItems(context.Background(), nil, nodes, listFields, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := map[string]any{
		"items":          items,
		"listFieldsHead": listFields,
		"displayStyle":   "list",
		"showPagination": false,
		"isViewList":     true,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if want := `<a href="/boards/24bba67" class="item-field-link">`; !bytes.Contains(buf.Bytes(), []byte(want)) {
		t.Fatalf("expected %q in output, got: %s", want, buf.String())
	}
}

// TestContentListTemplate_ImageFieldLinksToNode - ViewFieldDef.LinkTo must
// apply to the main image builtin too (views.BuildContentFieldInstances), not
// only to Title, so the image renders as a link. Mirrors
// TestContentListTemplate_TitleFieldLinksToNode.
func TestContentListTemplate_ImageFieldLinksToNode(t *testing.T) {
	zzChdirToProjectRoot(t)
	coverStorageID := 11
	svc := NewTemplateService(false)

	viewFields := entity.ViewFieldDefs{
		{ID: 1, FieldStorageID: &coverStorageID, LinkTo: entity.ViewFieldLinkContent},
	}
	listFields := views.BuildContentFieldInstances(context.Background(), viewFields, map[int]entity.FieldStorageDef{coverStorageID: {ID: coverStorageID, MachineName: "cover_image", FieldType: entity.FieldTypeImage}}, nil, nil)
	nodes := entity.Nodes{
		{ID: 1, Title: "Heading", Slug: "foo-bar"},
	}
	items, err := views.BuildContentListItems(context.Background(), nil, nodes, listFields, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items[0].ImageFieldURLs[listFields[0].ID] = "/uploads/styles/thumbnail/cover.webp"
	items[0].ImageFieldPictures[listFields[0].ID] = imageurl.Picture{Src: "/uploads/styles/thumbnail/cover.webp"}

	data := map[string]any{
		"items":          items,
		"listFieldsHead": listFields,
		"displayStyle":   "list",
		"showPagination": false,
		"isViewList":     true,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte(`<a href="/foo-bar" class="item-field-link">`)) {
		t.Fatalf("expected the image field's link_to=content to wrap it in a link to the node, got: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`class="item-field-image"`)) {
		t.Fatalf("expected the image itself to still render, got: %s", out)
	}
}

// TestMenuPlacementEditRenders - parse+execute smoke check of
// admin/blocks/menu_placement_edit.html with real data: the levels and
// the block's own title, shown.
func TestMenuPlacementEditRenders(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	menu := entity.MenuDef{ID: 1, MachineName: "main", Label: "Main menu"}
	placement := entity.BlockPlacementDef{
		ID: 20, PlacementType: entity.BlockPlacementMenu, MenuID: intPtrRegionTest(1),
		Region: entity.Region("header_nav"), MenuStartLevel: 1, MenuLevelsToShow: 2,
		VisibilityScope: entity.BlockVisibilityAll, Title: "Sections", ShowTitle: true,
	}
	data := map[string]any{
		"title":        "Menu block",
		"placement":    placement,
		"menu":         &menu,
		"allRegions":   zzFixtureThemeRegions(t).RegionIDsWithDisabled(),
		"regionLabels": regionLabelsForTest,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "admin/blocks/menu_placement_edit.html", "admin/alerts.html", "admin/blocks/visibility_content_types.html", "admin/blocks/visibility_roles.html", "admin/blocks/visibility_scope.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	// The block's name is in the line under the page's title.
	if err := svc.RenderNamed(&buf, "page_meta", data, "admin/blocks/menu_placement_edit.html", "admin/alerts.html", "admin/blocks/visibility_content_types.html", "admin/blocks/visibility_roles.html", "admin/blocks/visibility_scope.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Main menu (main)",
		`action="/admin/config/blocks/menu/20/edit"`,
		`value="1"`, // start level
		`value="2"`, // levels to show
		`name="title" class="form-control" value="Sections" placeholder="Main menu"`,
		`id="menu-placement-show-title" name="show_title" value="1" class="form-check-input" checked`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got: %s", want, out)
		}
	}
}

// TestPluginPlacementEditRenders - smoke test of the plugin placement edit
// page (Title/ShowTitle and the shared VisibilityScope selector); asserts
// the right <option selected> per VisibilityScope value.
func TestPluginPlacementEditRenders(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	pluginID := entity.PluginBlockSearchForm
	placement := entity.BlockPlacementDef{
		ID: 7, PlacementType: entity.BlockPlacementPlugin, PluginID: &pluginID,
		Region: entity.Region("header_search"), Title: "Search", ShowTitle: true,
		VisibilityScope: entity.BlockVisibilityPages, VisibilityPages: "news\n<front>", VisibilityPagesMode: entity.BlockVisibilityPagesHide,
		VisibilityRoles: "moderator",
	}
	data := map[string]any{
		"title":           "System block",
		"placement":       placement,
		"visibilityRoles": []struct{ Name, Label string }{{"anonymous", "Guest"}, {"moderator", "Moderator"}},
		"pluginLabel":     "Search the site",
		"allRegions":      zzFixtureThemeRegions(t).RegionIDsWithDisabled(),
		"regionLabels":    regionLabelsForTest,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "admin/blocks/plugin_placement_edit.html", "admin/alerts.html", "admin/blocks/visibility_content_types.html", "admin/blocks/visibility_roles.html", "admin/blocks/visibility_scope.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	// The block's name is in the line under the page's title.
	if err := svc.RenderNamed(&buf, "page_meta", data, "admin/blocks/plugin_placement_edit.html", "admin/alerts.html", "admin/blocks/visibility_content_types.html", "admin/blocks/visibility_roles.html", "admin/blocks/visibility_scope.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Search the site",
		`action="/admin/config/blocks/plugin/7/edit"`,
		`value="Search"`,
		"news\n&lt;front&gt;",
		`value="moderator" id="plugin-placement-role-moderator" checked`,
		`value="anonymous" id="plugin-placement-role-anonymous" >`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got: %s", want, out)
		}
	}
	if !strings.Contains(out, `value="pages" selected`) {
		t.Fatalf(`expected visibility_scope "pages" option to be selected, got: %s`, out)
	}
	if !strings.Contains(out, `value="hide" checked`) {
		t.Fatalf(`expected visibility_pages_mode "hide" radio to be checked, got: %s`, out)
	}
}

// TestViewPlacementEditRenders - smoke test of the View placement edit
// page, same checks as TestPluginPlacementEditRenders.
func TestViewPlacementEditRenders(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	viewDisplay := entity.ViewDisplayDef{ID: 42, Label: "Recent comments"}
	placement := entity.BlockPlacementDef{
		ID: 9, PlacementType: entity.BlockPlacementView, ViewDisplayID: intPtrRegionTest(42),
		Region: entity.Region("sidebar"), Title: "", ShowTitle: true,
		VisibilityScope: entity.BlockVisibilityPages, VisibilityPages: "forum", VisibilityPagesMode: entity.BlockVisibilityPagesShow,
	}
	data := map[string]any{
		"title":        "View block",
		"placement":    placement,
		"viewDisplay":  &viewDisplay,
		"allRegions":   zzFixtureThemeRegions(t).RegionIDsWithDisabled(),
		"regionLabels": regionLabelsForTest,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "admin/blocks/view_placement_edit.html", "admin/alerts.html", "admin/blocks/visibility_content_types.html", "admin/blocks/visibility_roles.html", "admin/blocks/visibility_scope.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Recent comments",
		`action="/admin/config/blocks/view/9/edit"`,
		`placeholder="Recent comments"`,
		"forum",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got: %s", want, out)
		}
	}
	if !strings.Contains(out, `value="pages" selected`) {
		t.Fatalf(`expected visibility_scope "pages" option to be selected, got: %s`, out)
	}
	if !strings.Contains(out, `id="view-placement-show-title" name="show_title" value="1" class="form-check-input" checked`) {
		t.Fatalf(`expected show_title checkbox to be checked, got: %s`, out)
	}
}

// TestRegionBlocksMultiContentTypeVisibility - one placement visible on
// several content types (comma-separated VisibilityContentType).
func TestRegionBlocksMultiContentTypeVisibility(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	types := "article, group"
	bp := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("sidebar"): {{
			Placement: entity.BlockPlacementDef{ID: 11, PlacementType: entity.BlockPlacementCustom, Region: entity.Region("sidebar"),
				VisibilityScope: entity.BlockVisibilityContentType, VisibilityContentType: &types},
			Block: entity.BlockDef{ID: 1, Title: "More from the author", ShowTitle: true, Status: true, Body: "<p>x</p>"},
		}},
	}
	for nodeType, want := range map[string]bool{"article": true, "group": true, "video": false} {
		ctx := map[string]any{
			"blockPlacements": bp,
			"isHomepage":      false,
			"item":            &entity.DynamicContent{Node: entity.Node{Type: entity.ContentType(nodeType)}},
		}
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "region_blocks", map[string]any{"Region": entity.Region("sidebar"), "Ctx": ctx}, zzBlockTemplateFiles...); err != nil {
			t.Fatalf("render error: %v", err)
		}
		if got := strings.Contains(buf.String(), "More from the author"); got != want {
			t.Fatalf("type %q: rendered=%v, want %v; output: %s", nodeType, got, want, buf.String())
		}
	}
}

// TestViewAttachmentsRender - dynamic/page.html's attachment point:
// title (or not) + pre-rendered HTML per attachment, nothing at all
// when the page has none.
func TestViewAttachmentsRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	type att struct {
		MachineName string
		Title       string
		ShowTitle   bool
		HTML        template.HTML
		Module      string
	}
	var buf bytes.Buffer
	data := map[string]any{"viewAttachments": []att{
		{MachineName: "similar_news", Title: "Similar news", ShowTitle: true, HTML: "<ul><li>a</li></ul>"},
		{MachineName: "more", Title: "Hidden heading", ShowTitle: false, HTML: "<p>b</p>"},
	}}
	if err := svc.RenderNamed(&buf, "view_attachments", data, "partials/view_attachments.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"<h2>Similar news</h2>", "<li>a</li>", "<p>b</p>", "view-attachment-similar_news"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got: %s", want, out)
		}
	}
	if strings.Contains(out, "Hidden heading") {
		t.Fatalf("title must be hidden when ShowTitle=false, got: %s", out)
	}
	if strings.Index(out, "<li>a</li>") > strings.Index(out, "<p>b</p>") {
		t.Fatalf("attachments must keep their order, got: %s", out)
	}

	buf.Reset()
	if err := svc.RenderNamed(&buf, "view_attachments", map[string]any{}, "partials/view_attachments.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("expected empty output without attachments, got: %q", buf.String())
	}
}

// TestVisibilityContentTypesCustomName - the shared checkbox partial
// with a View attachment display and its own form field name.
func TestVisibilityContentTypesCustomName(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	view := entity.ViewDisplayDef{DisplayType: entity.ViewDisplayTypeAttachment, AttachContentTypes: "news"}
	types := entity.ContentTypeDefs{{MachineName: "news", Label: "News"}, {MachineName: "article", Label: "Articles"}}
	var buf bytes.Buffer
	data := map[string]any{"Placement": view, "ContentTypes": types, "IDPrefix": "settings-attach", "Name": "attach_content_types"}
	if err := svc.RenderNamed(&buf, "visibility_content_types", data, "admin/blocks/visibility_content_types.html", "admin/blocks/visibility_roles.html", "admin/blocks/visibility_scope.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `name="attach_content_types" value="news" id="settings-attach-ct-news" checked`) {
		t.Fatalf("news must be checked under the custom name, got: %s", out)
	}
	if strings.Contains(out, `value="article" id="settings-attach-ct-article" checked`) || strings.Contains(out, `name="visibility_content_type"`) {
		t.Fatalf("unexpected checked/default-named input, got: %s", out)
	}
}
