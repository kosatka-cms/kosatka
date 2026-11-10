package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func TestUserFieldTemplatesRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	fields := entity.UserFieldDefs{
		{ID: 1, MachineName: "favorite_book", Label: "Favorite book", MaxLen: 255},
		{ID: 2, MachineName: "hobby", Label: "Hobby", MaxLen: 100, Weight: 10},
	}

	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	var buf strings.Builder
	if err := svc.RenderNamed(&buf, "content", map[string]any{"fields": fields, "error": "exists"}, "admin/people/user_fields.html", "admin/alerts.html", "admin/icons.html"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`action="/admin/users/fields/2/edit"`, `action="/admin/users/fields/1/delete"`, `value="Favorite book"`, "<code>hobby</code>", "already exists"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("admin page: missing %q", want)
		}
	}

	buf.Reset()
	data := map[string]any{
		"profileUser":   &entity.User{},
		"profileFields": []entity.UserFieldValue{{Field: fields[0], Value: "The Master and Margarita"}, {Field: fields[1]}},
	}
	if err := svc.RenderNamed(&buf, "profile_overview", data, "user/tabs/overview.html"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<label>Favorite book:</label>", "<p>The Master and Margarita</p>", "<label>Hobby:</label>", "<p>Not specified</p>"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("profile overview: missing %q", want)
		}
	}
}
