package service

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

// linkTestTerms - a term of a vocabulary with public term pages and one
// of a vocabulary without (classification only: its URL is a 404).
func linkTestTerms() entity.TaxonomyTerms {
	return entity.TaxonomyTerms{
		{ID: 1, Name: "Go", Slug: "tags/go", Vocabulary: entity.TaxonomyVocabulary{ID: 1, PublicTermPages: true}},
		{ID: 2, Name: "Internal", Slug: "kinds/internal", Vocabulary: entity.TaxonomyVocabulary{ID: 2}},
	}
}

func assertTermLinks(t *testing.T, where, out string, want ...string) {
	t.Helper()
	if !strings.Contains(out, `<a href="/tags/go">Go</a>`) {
		t.Errorf("%s: the term with a public page isn't linked:\n%s", where, out)
	}
	if strings.Contains(out, "/kinds/internal") {
		t.Errorf("%s: the term without a public page is linked:\n%s", where, out)
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("%s: missing %q:\n%s", where, w, out)
		}
	}
}

// TestNodePageTermLinks - a taxonomy field on the content page links
// only the terms whose vocabulary has public term pages; the others are
// plain text (factBoxFieldLine in dynamic/page.html).
func TestNodePageTermLinks(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("dynamic/page.html", []string{"dynamic", "page.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("node-page-term-links-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	field := entity.FieldInstanceDef{ID: 3, MachineName: "tags", Label: "Tags", FieldType: entity.FieldTypeTaxonomy, ShowLabel: true}
	var buf strings.Builder
	if err := set.ExecuteTemplate(&buf, "factBoxFieldLine", map[string]any{"Field": field, "Terms": linkTestTerms()}); err != nil {
		t.Fatal(err)
	}
	assertTermLinks(t, "factBoxFieldLine", buf.String(), `<a href="/tags/go">Go</a>, Internal</p>`)
}

// TestViewListTermLinks - the "linked" and "badges" tag formats of a
// View's taxonomy field link only the terms with public pages.
func TestViewListTermLinks(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)
	for format, want := range map[string]string{
		"linked": `<a href="/tags/go">Go</a>, Internal</span>`,
		"badges": `<li class="taxonomy_term"><span>Internal</span></li>`,
	} {
		field := entity.FieldInstanceDef{ID: 4, MachineName: "tags", FieldType: entity.FieldTypeTaxonomy,
			Config: map[string]any{"tag_format": format}}
		listFields := entity.FieldInstanceDefs{field}
		item := views.ContentListItem{
			Node:               entity.Node{ID: 1, Slug: "news/x"},
			Fields:             listFields,
			Values:             map[int]any{4: []int{1, 2}},
			TaxonomyFieldTerms: map[int]entity.TaxonomyTerms{4: linkTestTerms()},
		}
		data := map[string]any{
			"items":          []views.ContentListItem{item},
			"listFieldsHead": listFields,
			"displayStyle":   "grid",
			"showPagination": false,
			"isViewList":     true,
		}
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		assertTermLinks(t, format, buf.String(), want)
	}
}
