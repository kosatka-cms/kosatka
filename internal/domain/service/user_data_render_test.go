package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestUserDataCreatableMaterials - window.USER_DATA.creatableMaterials
// (the mobile "Create" submenu, app.js) is a JS array of {url, label},
// not a JSON string, on the site's and the admin's pages.
func TestUserDataCreatableMaterials(t *testing.T) {
	zzChdirToProjectRoot(t)
	data := map[string]any{"user": &entity.User{Username: "ann"}, "creatableMaterials": []entity.CreatableMaterialLink{{URL: "/create/article", Label: "Article"}}}
	for _, tc := range []struct{ name, define, file string }{
		{"site", "html_start", "partials/html.html"},
		{"admin", "admin_header", "admin/header.html"},
	} {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, tc.define, data, tc.file, "partials/html.html", "partials/theme_head.html"); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if want := `creatableMaterials: [{"url":"/create/article","label":"Article"}]`; !strings.Contains(buf.String(), want) {
			t.Errorf("%s: no %s", tc.name, want)
		}
	}
}
