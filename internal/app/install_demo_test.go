package app

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/app/starter"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/demo"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

func loadDemoContent(lang string) (demoContent, error) {
	var d demoContent
	err := demo.Load(demoFS, lang, &d)
	return d, err
}

// TestDemoContentFiles - every language's demo content parses, has the
// same articles as English, and titles giving distinct slugs.
func TestDemoContentFiles(t *testing.T) {
	en, err := loadDemoContent("en")
	if err != nil {
		t.Fatal(err)
	}
	files, err := fs.Glob(demoFS, "demo/*.json")
	if err != nil || len(files) == 0 {
		t.Fatal(files, err)
	}
	for _, f := range files {
		lang := strings.TrimSuffix(strings.TrimPrefix(f, "demo/"), ".json")
		d, err := loadDemoContent(lang)
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Articles) != len(en.Articles) || d.Page.Title == "" || d.Page.Body == "" || d.FooterMenu == "" {
			t.Errorf("%s: %d articles (en %d), page %q, footer menu %q", lang, len(d.Articles), len(en.Articles), d.Page.Title, d.FooterMenu)
		}
		slugs := map[string]bool{}
		for _, a := range d.Articles {
			s := slug.Generate(a.Title)
			if s == "" || slugs[s] || a.Body == "" {
				t.Errorf("%s: article %q (slug %q)", lang, a.Title, s)
			}
			slugs[s] = true
		}
	}
	// A language without its own demo content gets English.
	if d, err := loadDemoContent("xx"); err != nil || d.Page.Title != en.Page.Title {
		t.Errorf("fallback: %+v %v", d.Page, err)
	}
}

// TestDemoStarterNames - the starter set has what the demo content fills.
func TestDemoStarterNames(t *testing.T) {
	set, err := starter.Load()
	if err != nil {
		t.Fatal(err)
	}
	has := map[string]bool{}
	for _, ct := range set.ContentTypes {
		has["type "+ct.Name] = true
	}
	for _, fs := range set.FieldStorages {
		has["storage "+fs.Name] = true
	}
	for _, v := range set.Views {
		has["view "+v.Name] = true
	}
	for _, m := range set.Menus {
		has["menu "+m.Name] = true
	}
	for _, want := range []string{"type " + articleType, "type " + pageType, "storage " + bodyStorage, "storage " + imageStorage, "view " + frontpageView, "menu " + mainMenu} {
		if !has[want] {
			t.Errorf("the starter set has no %s", want)
		}
	}
}
