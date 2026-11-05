package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// seoTestTermOption mirrors the admin controller's term option.
type seoTestTermOption struct {
	ID       int
	Name     string
	Selected bool
}

func (seoTestTermOption) Indent() string { return "" }

// TestSEOAdminPagesRender executes the SEO admin pages on their real
// template sets.
func TestSEOAdminPagesRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	storyKind := 106
	rule := &entity.SEORuleDef{ID: 3, ContentType: "story", FieldStorageID: &storyKind, TermIDs: "1285", TitleTemplate: "[node:title] - book review"}
	typeFields := entity.FieldInstanceDefs{{MachineName: "story_kind", Label: "Review kind"}}
	for _, tc := range []struct {
		path string
		data map[string]any
		want []string
	}{
		{"admin/seo/index.html", map[string]any{
			"defaultTemplate": entity.SEOTemplate{TitleTemplate: "[node:title] | [site:name]"},
			"types":           []map[string]any{{"Type": entity.ContentTypeDef{ID: 13, MachineName: "video", Label: "Video"}, "Template": entity.SEOTemplate{TitleTemplate: "[node:field:title_alt]"}}},
			"rules":           []map[string]any{{"Rule": *rule, "TypeLabel": "Review", "Field": "Review kind", "Terms": []string{"Books"}}},
			"contentTypes":    entity.ContentTypeDefs{{MachineName: "story", Label: "Review"}},
			"fields":          []map[string]any{{"ContentType": "story", "FieldStorageID": 106, "Label": "Review kind"}},
			"error":           "",
		}, []string{`value="[node:title] | [site:name]"`, `/admin/config/seo/types/13/edit`, `<code>[node:field:title_alt]</code>`, "Review kind: Books", `data-rule-id="3"`, "[node:field:<i>machine_name</i>:year]"}},
		{"admin/seo/type_edit.html", map[string]any{
			"contentType": &entity.ContentTypeDef{ID: 13, Label: "Video"}, "template": entity.SEOTemplate{TitleTemplate: "[node:title]"},
			"defaultTemplate": entity.SEOTemplate{}, "typeFields": typeFields,
		}, []string{`action="/admin/config/seo/types/13"`, `value="[node:title]"`, "<code>story_kind</code> (Review kind)"}},
		{"admin/seo/rule_edit.html", map[string]any{
			"rule": rule, "contentTypes": entity.ContentTypeDefs{{MachineName: "story", Label: "Review"}},
			"fields": []map[string]any{{"ContentType": "story", "FieldStorageID": 106, "Label": "Review kind"}},
			"terms":  []seoTestTermOption{{ID: 1285, Name: "Books", Selected: true}}, "typeFields": typeFields, "error": "",
		}, []string{`<option value="1285" selected>Books</option>`, `<option value="106" data-filter-value="story" selected>`, `book review`}},
		{"admin/seo/rule_delete.html", map[string]any{"rule": rule}, []string{"/admin/config/seo/rules/3/delete"}},
	} {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		files, err := svc.pageFilesToParse(tc.path, strings.Split(tc.path, "/"))
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		set, err := svc.getOrParse(tc.path, nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		var buf strings.Builder
		if err := set.ExecuteTemplate(&buf, "content", tc.data); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		for _, w := range tc.want {
			if !strings.Contains(buf.String(), w) {
				t.Errorf("%s: missing %q", tc.path, w)
			}
		}
	}
}

// TestSearchAdminPagesRender executes the search admin pages.
func TestSearchAdminPagesRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	def := entity.SearchTypeDef{NodeType: "article", Limit: 15, Label: "Article", MatchFields: []string{"title_alt", "subtitle"}, TitleField: "title_alt", SubtitleField: "subtitle", ImageStyle: "article_list", BadgeVocabularyID: 31}
	for _, tc := range []struct {
		path string
		data map[string]any
		want []string
	}{
		{"admin/search/index.html", map[string]any{
			"types": []map[string]any{{"Def": def, "TypeLabel": "Articles"},
				{"Def": entity.SearchTypeDef{NodeType: "page", AllFields: true}, "TypeLabel": "Pages", "TextFields": true},
				{"Def": entity.SearchTypeDef{NodeType: "poll", AllFields: true}, "TypeLabel": "Polls"}},
			"available": []map[string]any{{"MachineName": "news", "Label": "News"}},
			"settings":  entity.SearchSettings{PlaceholderImage: "/static/images/no_cover.png", CategoriesMenu: "search"},
			"menus":     entity.MenuDefs{{MachineName: "search", Label: "Search"}, {MachineName: "main", Label: "Main"}},
			"error":     "",
		}, []string{`data-type="article"`, "<code>title_alt</code>, <code>subtitle</code>", `<option value="news">News</option>`, `<option value="search" selected>Search</option>`,
			// All text fields - for a type with them; a module's type: the title.
			"<td>title, all text fields</td>", "<td>title</td>"}},
		{"admin/search/type_edit.html", map[string]any{
			"def": &def, "match": map[string]bool{"title_alt": true, "subtitle": true},
			"textFields":   []map[string]any{{"MachineName": "title_alt", "Label": "Title"}, {"MachineName": "subtitle", "Label": "Name"}},
			"imageFields":  []map[string]any{},
			"imageStyles":  []map[string]any{{"Name": "article_list", "Label": "Article list"}},
			"vocabularies": entity.TaxonomyVocabularies{{ID: 31, Name: "Sections"}},
		}, []string{`value="title_alt" checked`, `id="search-match-chosen" value="chosen" checked`, `<option value="subtitle" selected>Name</option>`, `<option value="article_list" selected>`, `<option value="31" selected>Sections</option>`}},
	} {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		files, err := svc.pageFilesToParse(tc.path, strings.Split(tc.path, "/"))
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		set, err := svc.getOrParse(tc.path, nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		var buf strings.Builder
		if err := set.ExecuteTemplate(&buf, "content", tc.data); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		for _, w := range tc.want {
			if !strings.Contains(buf.String(), w) {
				t.Errorf("%s: missing %q", tc.path, w)
			}
		}
	}
}
