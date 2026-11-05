package service

import (
	"html/template"
	"strings"
	"testing"
)

// TestProfileViewTabsRender - the profile's View tabs: label with count,
// the active one marked.
func TestProfileViewTabsRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("user/profile_page.html", []string{"user", "profile_page.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("user/profile_page.html", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	data := map[string]any{"profileViewTabs": []map[string]any{
		{"Label": "Content", "URL": "/users/ishmael/content", "Count": 12, "Active": true},
		{"Label": "In moderation", "URL": "/users/ishmael/moderation", "Count": 0, "Active": false},
	}}
	if err := set.ExecuteTemplate(&buf, "profile_view_tabs", data); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{`<a href="/users/ishmael/content" class="tab-item active"> Content (12) </a>`, `<a href="/users/ishmael/moderation" class="tab-item "> In moderation (0) </a>`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}
