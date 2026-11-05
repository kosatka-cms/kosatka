package form

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestParseLinkRows - the rows' addresses and titles paired in order;
// an empty row left out, an address without a scheme gets https://.
func TestParseLinkRows(t *testing.T) {
	form := url.Values{
		"field_4_url":   {" https://a.example/1 ", "", "b.example/2", ""},
		"field_4_title": {"A", "", "", "Title only"},
	}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	got := ParseLinkRows(r, 4)
	want := []entity.Link{{URL: "https://a.example/1", Title: "A"}, {URL: "https://b.example/2"}, {Title: "Title only"}}
	if !slices.Equal(got, want) {
		t.Errorf("ParseLinkRows = %v, want %v", got, want)
	}
}

// TestValidateLinks - a link field's checks: required, how many, a web
// address, an allowed site, a title where required.
func TestValidateLinks(t *testing.T) {
	ctx := context.Background()
	field := entity.FieldInstanceDef{Label: "Trailer", FieldType: entity.FieldTypeLink, Required: true,
		Config:       map[string]any{"multiple": true, "allowed_hosts": []any{"video.example"}, "link_title": "required"},
		WidgetConfig: map[string]any{"max_values": float64(2)}}
	ok := entity.Link{URL: "https://video.example/1", Title: "One"}
	for name, tc := range map[string]struct {
		field entity.FieldInstanceDef
		links []entity.Link
		want  string
	}{
		"valid":              {field, []entity.Link{ok}, ""},
		"required":           {field, nil, "Trailer is required"},
		"too many":           {field, []entity.Link{ok, ok, ok}, "Trailer: at most 2 links."},
		"one value":          {entity.FieldInstanceDef{Label: "Site", FieldType: entity.FieldTypeLink}, []entity.Link{ok, ok}, "Site: at most 1 link."},
		"not an address":     {field, []entity.Link{{URL: "javascript:alert(1)", Title: "x"}}, `Trailer: "javascript:alert(1)" is not a web address (it starts with http:// or https://).`},
		"no address":         {field, []entity.Link{{Title: "x"}}, `Trailer: "" is not a web address (it starts with http:// or https://).`},
		"another site":       {field, []entity.Link{{URL: "https://other.example/1", Title: "x"}}, "Trailer: other.example is not among the sites allowed here (video.example)."},
		"title required":     {field, []entity.Link{{URL: "https://video.example/1"}}, "Trailer: every link needs a title."},
		"not required, none": {entity.FieldInstanceDef{Label: "Site", FieldType: entity.FieldTypeLink}, nil, ""},
	} {
		if got := validateLinks(ctx, tc.field, tc.links); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}
