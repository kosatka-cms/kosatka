package forms

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestWriteWebformCSV - a BOM, a header with the elements' labels, a row
// per submission with the page it was sent from, several values joined
// and quoting where needed.
func TestWriteWebformCSV(t *testing.T) {
	wf := Webform{Elements: WebformElements{{ID: 1, Label: "Name"}, {ID: 2, Label: "Topics"}}}
	subs := []WebformSubmission{
		{ID: 2, Username: "ann", RemoteIP: "10.0.0.1", SourceURL: "/catalog/tank?size=200", CreatedAt: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), Values: map[int][]string{1: {"Ann, Jr."}, 2: {"Ads", "Errors"}}},
		{ID: 1, CreatedAt: time.Date(2026, 10, 15, 9, 30, 0, 0, time.UTC), Values: map[int][]string{1: {"Bob"}}},
	}
	each := func(_ context.Context, _ Webform, fn func(WebformSubmission) error) error {
		for _, s := range subs {
			if err := fn(s); err != nil {
				return err
			}
		}
		return nil
	}
	var b strings.Builder
	if err := writeWebformCSV(context.Background(), &b, wf, each); err != nil {
		t.Fatal(err)
	}
	want := "\ufeff#,Date,User,IP,Page,Name,Topics\n" +
		"2,2026-10-16 12:00:00,ann,10.0.0.1,/catalog/tank?size=200,\"Ann, Jr.\",Ads; Errors\n" +
		"1,2026-10-15 09:30:00,,,,Bob,\n"
	if b.String() != want {
		t.Errorf("csv:\n%q\nwant\n%q", b.String(), want)
	}
}

// TestWriteWebformCSVFormulas - values (and labels, sender names) that a
// spreadsheet would run as formulas are written as text.
func TestWriteWebformCSVFormulas(t *testing.T) {
	wf := Webform{Elements: WebformElements{{ID: 1, Label: "=Label"}}}
	subs := []WebformSubmission{{ID: 1, Username: "@ann", Values: map[int][]string{1: {`=HYPERLINK("http://evil.test","x")`}}}}
	each := func(_ context.Context, _ Webform, fn func(WebformSubmission) error) error {
		return fn(subs[0])
	}
	var b strings.Builder
	if err := writeWebformCSV(context.Background(), &b, wf, each); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"'=Label", "'@ann", `"'=HYPERLINK(""http://evil.test"",""x"")"`} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("no %s in %q", want, b.String())
		}
	}
	for in, want := range map[string]string{"-1": "'-1", "+7 900": "'+7 900", "\tx": "'\tx", "plain": "plain", "": "", "a=b": "a=b"} {
		if got := csvCell(in); got != want {
			t.Errorf("csvCell(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestWebformFromFormCommentMode - a new form gets the comments chosen in
// the editor by who administers content; without the choice, or from
// anyone else (the editor doesn't show it to them), it gets the new
// form's (hidden), not the node's default (open); an edit keeps the
// form's own. Likewise its address.
func TestWebformFromFormCommentMode(t *testing.T) {
	controller := func(admin bool) *WebformController {
		return &WebformController{BaseController: modulekit.NewBaseController(modulekit.BaseDeps{
			Auth: testkit.Auth{UserID: 3, Permissions: map[string]bool{modulekit.PermAdministerContent: admin}}, Logger: slog.New(slog.DiscardHandler),
		})}
	}
	user := &modulekit.User{ID: 3}
	r := httptest.NewRequest(http.MethodPost, "/create/form", nil)
	existing := &Webform{Node: modulekit.Node{ID: 9, Slug: "feedback", CommentMode: modulekit.CommentModeOpen}}
	for _, tc := range []struct {
		name     string
		admin    bool
		mode     string
		existing *Webform
		want     modulekit.CommentMode
	}{
		{"new, chosen by an administrator", true, string(modulekit.CommentModeOpen), nil, modulekit.CommentModeOpen},
		{"new, sent by an author", false, string(modulekit.CommentModeOpen), nil, modulekit.CommentModeHidden},
		{"new, not shown", true, "", nil, modulekit.CommentModeHidden},
		{"edit, not shown", true, "", existing, modulekit.CommentModeOpen},
		{"edit, sent by an author", false, string(modulekit.CommentModeHidden), existing, modulekit.CommentModeOpen},
	} {
		wf := controller(tc.admin).webformFromForm(r, &WebformForm{Title: "Feedback", Slug: "contact", CommentMode: tc.mode}, user, tc.existing)
		if !tc.admin {
			wantSlug := ""
			if tc.existing != nil {
				wantSlug = tc.existing.Node.Slug
			}
			if wf.Node.Slug != wantSlug {
				t.Errorf("%s: an author's address %q, want %q", tc.name, wf.Node.Slug, wantSlug)
			}
		}
		if wf.Node.CommentMode != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, wf.Node.CommentMode, tc.want)
		}
	}
}

// TestCleanWebformSource - a submission's source is a path of this site
// (with its query, without the fragment): an address of the site becomes
// its path; another site's address, a scheme other than http(s), a
// protocol-relative or broken value, or a too long one - none.
func TestCleanWebformSource(t *testing.T) {
	hosts := []string{"site.test", "https://www.site.test"}
	for raw, want := range map[string]string{
		"":                                    "",
		"/catalog/tank":                       "/catalog/tank",
		"/catalog/tank?size=200#specs":        "/catalog/tank?size=200",
		"https://site.test/news/one":          "/news/one",
		"http://WWW.site.test/a?b=1":          "/a?b=1",
		"https://evil.test/catalog":           "",
		"//evil.test/catalog":                 "",
		"javascript:alert(1)":                 "",
		"catalog/tank":                        "",
		"/a b":                                "",
		"/" + strings.Repeat("a", 500):        "",
		"/" + strings.Repeat("a", 498) + "?x": "",
	} {
		if got := CleanWebformSource(raw, hosts...); got != want {
			t.Errorf("%q: %q, want %q", raw, got, want)
		}
	}
}

// TestWebformSourceOfRequest - the page forms.js names wins; without it,
// the Referer, unless it's the form's own page.
func TestWebformSourceOfRequest(t *testing.T) {
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/order"}}
	req := func(form, referer string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "http://site.test/node/9/form", strings.NewReader(url.Values{"webform_source": {form}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if referer != "" {
			r.Header.Set("Referer", referer)
		}
		return r
	}
	for _, c := range []struct{ form, referer, want string }{
		{"/catalog/tank", "http://site.test/form/order", "/catalog/tank"},
		{"", "http://site.test/catalog/tank", "/catalog/tank"},
		{"", "http://site.test/form/order", ""},
		{"", "http://site.test/form/order?product=tank", ""},
		{"https://evil.test/x", "https://evil.test/y", ""},
	} {
		if got := webformSource(req(c.form, c.referer), wf); got != c.want {
			t.Errorf("%+v: %q", c, got)
		}
	}
}
