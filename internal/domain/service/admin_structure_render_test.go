package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestAdminStructurePagesRender - the Structure section's lists on their
// real template sets: adding on a card over the list, the list on a table
// card; a sortable one dragged by its handle, its order saved in the
// card's foot, a tree's rows indented by --tree-indent.
func TestAdminStructurePagesRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	groupID := 4
	fields := entity.FieldInstanceDefs{
		{ID: 4, Label: "Specs", MachineName: "field_specs", FieldType: entity.FieldTypeGroup, UsableInForm: true, UsableInDetail: true},
		{ID: 5, Label: "Size", MachineName: "field_size", FieldType: entity.FieldTypeText, FormGroupID: &groupID, ParentGroupID: &groupID},
	}
	parent := 1
	for _, tc := range []struct {
		path string
		data map[string]any
		want []string
	}{
		{"admin/content-types/index.html", map[string]any{"contentTypes": entity.ContentTypeDefs{{ID: 2, MachineName: "article", Label: "Article", Status: 1}}},
			[]string{`class="admin-settings-card admin-add-card"`, `class="admin-table-title">Article</a>`, `<code>article</code>`, `form="ct-delete-2"`}},
		{"admin/content-types/fields.html", map[string]any{"contentType": &entity.ContentTypeDef{ID: 2, Label: "Article"}, "activeTab": "fields",
			"fields": fields, "orderedFields": fields, "fieldTypes": []string{"text", "group"}},
			[]string{`class="nav-link active" href="/admin/config/content-types/2/fields"`, `id="fields-attach"`, `data-member-of="4"`, `class="drag-handle admin-table-drag"`, `<button form="fields-reorder-form" type="submit" class="btn btn-primary">`,
				`<input type="hidden" name="usable_in_form" value="1">`}},
		{"admin/content-types/detail_display.html", map[string]any{"contentType": &entity.ContentTypeDef{ID: 2, Label: "Article"}, "activeTab": "detail-display",
			"fields": fields, "displayFields": fields},
			[]string{`id="detail-display-form" method="post" action="/admin/config/content-types/2/fields/display-detail" class="admin-table-card"`, `class="admin-table-foot"`}},
		{"admin/taxonomy/index.html", map[string]any{"vocabularies": []map[string]any{{"ID": 3, "Name": "Tags", "Slug": "tags", "Desc": ""}}},
			[]string{`data-sort-id="3"`, `href="/admin/config/taxonomy/3/terms" class="admin-table-title">Tags</a>`, `<button form="vocabularies-reorder-form"`}},
		{"admin/taxonomy/terms.html", map[string]any{"vocabulary": map[string]any{"ID": 3, "Name": "Tags"},
			"terms":            []map[string]any{{"ID": 1, "ParentID": 0, "Name": "News", "Slug": "news"}, {"ID": 2, "ParentID": 1, "Name": "Local", "Slug": "news/local"}},
			"termDisplayNames": map[int]string{1: "News", 2: "- Local"}, "termDepths": map[int]int{1: 0, 2: 1}},
			[]string{`data-tree-indent style="--tree-indent: 0px;"`, `data-tree-indent style="--tree-indent: 24px;"`, `<option value="1" selected>News</option>`}},
		{"admin/menus/index.html", map[string]any{"menus": []map[string]any{{"ID": 1, "Label": "Main", "MachineName": "main", "Description": ""}}},
			[]string{`href="/admin/config/menus/1/links" class="admin-table-title">Main</a>`}},
		{"admin/menus/links.html", map[string]any{"menu": map[string]any{"ID": 1, "Label": "Main"},
			"rows": []map[string]any{
				{"ID": 7, "ParentID": nil, "ParentIDValue": 0, "Depth": 0, "Title": "Home", "LinkType": "url", "URLOK": true, "ResolvedURL": "/", "Enabled": true, "DescendantIDs": map[int]bool{8: true}, "IndentedLabel": "Home"},
				{"ID": 8, "ParentID": &parent, "ParentIDValue": 7, "Depth": 1, "Title": "About", "LinkType": "node", "URLOK": false, "Enabled": false, "DescendantIDs": map[int]bool{}, "IndentedLabel": "- About"},
			}},
			[]string{`id="menu-links-reorder-form" method="post" action="/admin/config/menus/1/links/reorder" class="admin-table-card"`, `--tree-indent: 20px;`, `admin-pill admin-pill--warn`}},
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
		tc.data["pagination"] = pagination.New(1, 1, 25, "/", nil)
		var buf strings.Builder
		if err := set.ExecuteTemplate(&buf, "page_body", tc.data); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		for _, w := range tc.want {
			if !strings.Contains(buf.String(), w) {
				t.Errorf("%s: missing %q in\n%s", tc.path, w, buf.String())
			}
		}
	}
}
