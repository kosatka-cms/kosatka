package entity

import "testing"

func TestAdminMenuCurrent(t *testing.T) {
	menu := []AdminMenuSection{
		{Label: "Overview", Path: "/admin", Exact: true},
		{Label: "Content", Items: []AdminMenuItem{{Label: "Content", Path: "/admin/content"}, {Label: "Comments", Path: "/admin/content/comment"}}},
		{Label: "Structure", Items: []AdminMenuItem{{Label: "Content types", Path: "/admin/config/content-types"}}},
		{Label: "Modules", Path: "/admin/config/modules"},
	}
	for path, want := range map[string]string{
		"/admin/content":                     "Content/Content",
		"/admin/content/comment/5":           "Content/Comments",
		"/admin/config/content-types/1":      "Structure/Content types",
		"/admin/config/modules/forum/delete": "Modules/Modules",
		"/admin/config/content-typesx":       "",
		"/admin/users":                       "",
		"/admin":                             "Overview/Overview",
	} {
		s, it, ok := AdminMenuCurrent(menu, path)
		got := ""
		if ok {
			got = s.Label + "/" + it.Label
		}
		if got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
}
