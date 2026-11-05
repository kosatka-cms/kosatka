package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestProfileFormSocialNetworks - one's own profile form lists the social
// networks offered: a connected one with its disconnect button, another
// with its connect link (back to the form), and the notice of the last
// change.
func TestProfileFormSocialNetworks(t *testing.T) {
	zzChdirToProjectRoot(t)
	path := "user/edit_form.html"
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse(path, strings.Split(path, "/"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse(path, nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		Name, Label string
		Connected   bool
	}
	data := map[string]any{
		"editUser":      &entity.User{ID: 3, Username: "ann", Email: "ann@example.com"},
		"isSelf":        true,
		"oauthServices": []row{{Name: "google", Label: "Google", Connected: true}, {Name: "yandex", Label: "Yandex"}},
		"oauthMessage": &struct {
			Text  string
			Error bool
		}{Text: "The social network is connected: you can sign in with it."},
	}
	var buf strings.Builder
	if err := set.ExecuteTemplate(&buf, "content", data); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`formaction="/auth/google/disconnect" formmethod="post"`,
		`href="/auth/yandex?destination=/me/edit"`,
		`alert-success" role="status">The social network is connected`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s", want)
		}
	}
	if strings.Contains(out, `/auth/google?destination`) || strings.Contains(out, `/auth/yandex/disconnect`) {
		t.Error("a connected network offered to connect, or one not connected to disconnect")
	}
}
