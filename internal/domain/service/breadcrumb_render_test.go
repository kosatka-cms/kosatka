package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// breadcrumbTestTerm mirrors the admin controller's term option (Indent
// is a method there too).
type breadcrumbTestTerm struct {
	ID       int
	Name     string
	Selected bool
}

func (breadcrumbTestTerm) Indent() string { return "- " }

// TestBreadcrumbTemplatesRender executes the breadcrumb partial and the
// breadcrumb admin pages on their real template sets.
func TestBreadcrumbTemplatesRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	fieldID := 109
	rule := &entity.BreadcrumbRuleDef{ID: 5, ContentType: "article", FieldStorageID: &fieldID, TermIDs: "2856", MenuLinkID: 68}
	common := map[string]any{
		"contentTypes": entity.ContentTypeDefs{{MachineName: "article", Label: "Article"}},
		"fields":       []map[string]any{{"ContentType": "article", "FieldStorageID": 109, "Label": "Section"}},
		"menuLinks":    []BreadcrumbMenuLinkOption{{ID: 67, Menu: "main", Title: "Alpha section", URL: "/alpha-articles"}, {ID: 68, Menu: "main", Title: "Main", URL: "/alpha-articles", Depth: 1}},
		"error":        "",
	}
	for _, tc := range []struct {
		path, define string
		data         map[string]any
		want         []string
	}{
		{"dynamic/page.html", "breadcrumbs_nav", map[string]any{"breadcrumbs": items("/", "Home", "/alpha-articles", "Alpha section", "", "Foo Bar")},
			[]string{`<a href="/alpha-articles">Alpha section</a>`, "<span>Foo Bar</span>", `separator`}},
		{"admin/breadcrumbs/index.html", "content", map[string]any{"rows": []map[string]any{{"Rule": *rule, "TypeLabel": "Article", "Field": "Section", "Terms": []string{"Alpha"}, "Link": "Alpha section › Main"}}},
			[]string{"Section: Alpha", "Alpha section › Main", `data-rule-id="5"`, "[main] - Main (/alpha-articles)"}},
		{"admin/breadcrumbs/edit.html", "content", map[string]any{"rule": rule, "linkFound": true, "terms": []breadcrumbTestTerm{{ID: 2856, Name: "Alpha", Selected: true}}},
			[]string{`<option value="2856" selected>- Alpha</option>`, `<option value="68" selected>`, `<option value="109" data-filter-value="article" selected>`}},
		{"admin/breadcrumbs/delete.html", "content", map[string]any{"rule": rule, "link": "Alpha section › Main"},
			[]string{"/admin/config/breadcrumbs/5/delete"}},
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
		data := map[string]any{}
		for k, v := range common {
			data[k] = v
		}
		for k, v := range tc.data {
			data[k] = v
		}
		var buf strings.Builder
		if err := set.ExecuteTemplate(&buf, tc.define, data); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		for _, w := range tc.want {
			if !strings.Contains(buf.String(), w) {
				t.Errorf("%s: missing %q in\n%s", tc.path, w, buf.String())
			}
		}
	}
}

// TestBreadcrumbPagesParse - every page that renders breadcrumbs_nav
// parses on its real template set and renders the trail it is given.
func TestBreadcrumbPagesParse(t *testing.T) {
	zzChdirToProjectRoot(t)
	trail := items("/", "Home", "/galleries", "Galleries", "", "Gallery")
	for _, path := range []string{
		"comments/recent.html", "create/materials_hub.html",
		"user/profile_page.html", "user/profile_tab_forbidden.html", "user/edit_form.html", "user/login.html", "user/register.html",
		"user/register_success.html", "user/register_disabled.html", "user/password_request.html", "user/password_reset.html", "user/complete_profile.html",
		"node/galleries_list.html", "node/edit_gallery.html", "node/edit_dynamic.html", "node/delete_confirm.html", "node/edit_poll.html",
		"gallery/view.html", "gallery/delete_confirm.html", "poll/detail.html", "poll/votes.html", "webform/detail.html", "webform/submissions.html", "webform/analysis.html", "webform/submission.html", "errors/too_large.html", "node/edit_webform.html", "comment/edit.html", "comment/delete.html",
	} {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		files, err := svc.pageFilesToParse(path, strings.Split(path, "/"))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		set, err := svc.getOrParse(path, nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		var buf strings.Builder
		if err := set.ExecuteTemplate(&buf, "breadcrumb", map[string]any{"breadcrumbs": trail}); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !strings.Contains(buf.String(), `<a href="/galleries">Galleries</a>`) {
			t.Errorf("%s: breadcrumb block does not render the trail: %q", path, buf.String())
		}
	}
}
