package service

import (
	"context"
	"errors"
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func menuLabels(sections []entity.AdminMenuSection) (sectionLabels []string, items map[string][]string) {
	items = map[string][]string{}
	for _, s := range sections {
		sectionLabels = append(sectionLabels, s.Label)
		for _, it := range s.Items {
			items[s.Label] = append(items[s.Label], it.Label)
		}
	}
	return sectionLabels, items
}

func TestVisibleAdminMenu(t *testing.T) {
	withModules(t, nil)
	all := func(string) bool { return true }

	sections, items := menuLabels(VisibleAdminMenu(all))
	if strings.Join(sections, ",") != "Overview,Content,Structure,Appearance,Modules,Configuration,Users,Reports" {
		t.Fatalf("sections: %v", sections)
	}
	if !strings.Contains(strings.Join(items["Content"], ","), "Error reports") || !strings.Contains(strings.Join(items["Configuration"], ","), "Feeds") {
		t.Errorf("module items missing while modules are on: %v", items)
	}

	SetModules(map[string]bool{"error_reports": false, "feeds": false})
	_, items = menuLabels(VisibleAdminMenu(all))
	if strings.Contains(strings.Join(items["Content"], ","), "Error reports") || strings.Contains(strings.Join(items["Configuration"], ","), "Feeds") {
		t.Errorf("items of disabled modules shown: %v", items)
	}

	// Without the files permission the item goes; the section stays.
	_, items = menuLabels(VisibleAdminMenu(func(p string) bool { return p != PermAdministerUserFiles }))
	if strings.Contains(strings.Join(items["Content"], ","), "Files") || len(items["Content"]) == 0 {
		t.Errorf("content items: %v", items["Content"])
	}
}

// TestAdminTopBarRender - sections render from the menu: dropdowns with
// group titles, each group (its title and links) one block, plain links
// for Appearance and Modules, the create submenu in Content, an item's
// count by its link.
func TestAdminTopBarRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	withModules(t, nil)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	menu := VisibleAdminMenu(func(string) bool { return true })
	withAdminMenuCount(t, "/admin/error-reports", func(context.Context) (int, error) { return 3, nil })
	CountAdminMenu(context.Background(), menu, nil)
	data := map[string]any{
		"pendingPeopleCount": 0,
		"user":               &entity.User{ID: 1},
		"adminMenu":          menu,
		"creatableMaterials": []entity.CreatableMaterialLink{{URL: "/create/news", Label: "News"}},
	}
	var buf strings.Builder
	if err := svc.RenderNamed(&buf, "admin_top_bar", data, "partials/html.html"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`<a href="/admin/config/modules" class="admin-bar-item admin-bar-top-link">Modules</a>`,
		`<div class="admin-bar-group">Site</div>`,
		`<div class="admin-bar-group">Security</div>`,
		`<a href="/admin/config/appearance" class="admin-bar-item admin-bar-top-link">Appearance</a>`,
		`href="/admin/config/image-styles" class="admin-bar-link">Image styles</a>`,
		`href="/admin/error-reports" class="admin-bar-link">Error reports <span class="admin-bar-count">3</span></a>`,
		`href="/create/news"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("toolbar lacks %s", want)
		}
	}
	if groups, blocks := strings.Count(out, `<div class="admin-bar-group">`), strings.Count(out, `<div class="admin-bar-section"><div class="admin-bar-group">`); groups == 0 || blocks != groups {
		t.Errorf("%d groups, %d of them in their blocks", groups, blocks)
	}
	if opened, closed := strings.Count(out, "<div"), strings.Count(out, "</div>"); opened != closed {
		t.Errorf("%d divs opened, %d closed", opened, closed)
	}
	if strings.Contains(out, `"/admin/config/system"`) || strings.Contains(out, "/admin/config/dev") {
		t.Error("placeholder pages linked")
	}
	if strings.Index(out, ">Content ▾") > strings.Index(out, ">Structure ▾") {
		t.Error("sections out of order")
	}
}

// withAdminMenuCount registers a counter for the test, the previous one
// (or none) back after it.
func withAdminMenuCount(t *testing.T, path string, count func(context.Context) (int, error), label ...string) {
	t.Helper()
	adminMenuCountsMu.RLock()
	prev, had := adminMenuCounts[path]
	adminMenuCountsMu.RUnlock()
	RegisterAdminMenuCount(path, count, label...)
	t.Cleanup(func() {
		adminMenuCountsMu.Lock()
		defer adminMenuCountsMu.Unlock()
		if had {
			adminMenuCounts[path] = prev
		} else {
			delete(adminMenuCounts, path)
		}
	})
}

// TestCountAdminMenu - an item with a counter gets its number, one whose
// counter fails none; the others stay 0, the menu's source untouched.
func TestCountAdminMenu(t *testing.T) {
	menu := []entity.AdminMenuSection{{Label: "Content", Items: []entity.AdminMenuItem{
		{Label: "A", Path: "/a"}, {Label: "B", Path: "/b"}, {Label: "C", Path: "/c"},
	}}}
	withAdminMenuCount(t, "/a", func(context.Context) (int, error) { return 5, nil }, "New addresses")
	withAdminMenuCount(t, "/b", func(context.Context) (int, error) { return 7, errors.New("db down") })
	CountAdminMenu(context.Background(), menu, nil)
	if got := []int{menu[0].Items[0].Count, menu[0].Items[1].Count, menu[0].Items[2].Count}; got[0] != 5 || got[1] != 0 || got[2] != 0 {
		t.Errorf("counts: %v, want [5 0 0]", got)
	}
	if menu[0].Items[0].CountLabel != "New addresses" || menu[0].Items[2].CountLabel != "" {
		t.Errorf("count labels: %+v", menu[0].Items)
	}
	for _, s := range entity.AdminMenu() {
		for _, it := range s.Items {
			if it.Count != 0 {
				t.Errorf("the menu's source counted: %+v", it)
			}
		}
	}
}
