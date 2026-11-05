package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// postForm - a POST request carrying form.
func postForm(form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

// TestSEOSchemaFromForm - the type chosen and its own properties only
// (another type's left out, blank ones dropped); "none" - a content
// type's only; an unknown type - none.
func TestSEOSchemaFromForm(t *testing.T) {
	r := postForm(url.Values{"schema_type": {"Event"}, "schema_prop.Event.startDate": {" [node:field:date:iso] "},
		"schema_prop.Event.endDate": {""}, "schema_prop.Article.keywords": {"[node:field:tags]"}})
	if name, props := seoSchemaFromForm(r, false); name != "Event" || !reflect.DeepEqual(props, map[string]string{"startDate": "[node:field:date:iso]"}) {
		t.Errorf("event: %q %v", name, props)
	}
	if name, _ := seoSchemaFromForm(postForm(url.Values{"schema_type": {"none"}}), true); name != entity.SchemaNone {
		t.Errorf("a type's none: %q", name)
	}
	for _, form := range []url.Values{{"schema_type": {"none"}}, {"schema_type": {"Thing"}}} {
		if name, props := seoSchemaFromForm(postForm(form), false); name != "" || props != nil {
			t.Errorf("%v: %q %v", form, name, props)
		}
	}
}

// TestSchemaOrgFromForm - who runs the site: an unknown kind is an
// organization, the profiles web addresses only.
func TestSchemaOrgFromForm(t *testing.T) {
	got := schemaOrgFromForm(postForm(url.Values{"schema_org_kind": {"Spaceship"}, "schema_org_name": {" Ex "},
		"schema_org_same_as": {"https://t.me/ex\nnot a link\n http://vk.com/ex \n"}, "schema_org_search": {"1"}}))
	want := entity.SiteSchemaOrg{Kind: entity.SchemaOrgOrganization, Name: "Ex", SameAs: []string{"https://t.me/ex", "http://vk.com/ex"}, SearchBox: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%+v, want %+v", got, want)
	}
}

// TestSEOTypeSchemaRender - a content type's SEO page: the schema.org
// type (as for all content by default, naming the default's), each
// type's properties shown for it with the saved values of the chosen
// one; the newest item's markup and a test by Google.
func TestSEOTypeSchemaRender(t *testing.T) {
	out := renderPeople(t, "admin/seo/type_edit.html", map[string]any{
		"contentType":     entity.ContentTypeDef{ID: 4, MachineName: "event", Label: "Event"},
		"template":        entity.SEOTemplate{SchemaType: "Event", SchemaProps: map[string]string{"startDate": "[node:field:date:iso]"}},
		"defaultTemplate": entity.SEOTemplate{SchemaType: "Article"},
		"schemaTypes":     entity.SchemaTypes,
		"schemaSample":    &seoSchemaSample{JSON: `{"@type": "Event"}`, Title: "Meetup", URL: "/events/meetup", TestURL: "https://search.google.com/test/rich-results?url=x"},
	})
	for _, want := range []string{
		`<option value="">As for all content (Article)</option>`, `<option value="none">- none -</option>`,
		`<option value="Event" selected>Event (Event)</option>`,
		`<div class="admin-setting" data-show-when="seo-schema-type=Event">`,
		`name="schema_prop.Event.startDate" class="form-control font-monospace" value="[node:field:date:iso]"`,
		`name="schema_prop.Article.keywords" class="form-control font-monospace" value=""`,
		`The markup of "Meetup"`, `<pre class="admin-schema-json">{&#34;@type&#34;: &#34;Event&#34;}</pre>`,
		`href="https://search.google.com/test/rich-results?url=x"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}
