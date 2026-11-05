package forms

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestParseWebformForm - element rows come in their posted order; an
// unchecked "required" box doesn't shift the next rows; an empty new row is
// dropped; options are kept only for choice types.
func TestParseWebformForm(t *testing.T) {
	form := url.Values{
		"title":       {" Contact "},
		"element_row": {"12", "new1", "new2"},

		"element_type_12": {"text"}, "element_label_12": {"Name"}, "element_required_12": {"1"},
		"element_type_new1": {"radios"}, "element_label_new1": {"Topic"}, "element_options_new1": {"Ads\n\n Errors "},
		"element_type_new2": {"text"},
	}
	r, _ := http.NewRequest(http.MethodPost, "/create/form", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = r.ParseForm()
	var f WebformForm
	f.ParseWebformForm(r)
	if f.Title != "Contact" || len(f.Elements) != 2 {
		t.Fatalf("form: %+v", f)
	}
	if e := f.Elements[0]; e.ID != 12 || !e.Required || e.Label != "Name" {
		t.Errorf("first: %+v", e)
	}
	if e := f.Elements[1]; e.ID != 0 || e.Required || e.Type != WebformRadios {
		t.Errorf("second: %+v", e)
	}
	els := f.WebformElements()
	if len(els[1].Options) != 2 || els[1].Options[1] != "Errors" || els[0].Options != nil || els[1].Weight != 1 {
		t.Errorf("elements: %+v", els)
	}
	if errs := f.Validate(context.Background()); len(errs) != 0 {
		t.Errorf("valid form: %v", errs)
	}

	bad := WebformForm{Elements: []WebformElementInput{{Key: "new1", Type: WebformSelect, Label: "Pick"}, {Key: "new2", Type: "bogus", Label: "X"}}}
	errs := bad.Validate(context.Background())
	if errs["title"] == "" || errs["element_new1"] == "" || errs["element_new2"] == "" {
		t.Errorf("invalid form: %v", errs)
	}
}

// TestWebformNotifyValidation - every notification address must be valid.
func TestWebformNotifyValidation(t *testing.T) {
	ok := WebformForm{Title: "T", NotifyEmail: "a@example.test, b@example.test", Elements: []WebformElementInput{{Key: "1", Type: WebformText, Label: "Q"}}}
	if errs := ok.Validate(context.Background()); len(errs) != 0 {
		t.Errorf("valid: %v", errs)
	}
	bad := ok
	bad.NotifyEmail = "a@example.test, not an address"
	if errs := bad.Validate(context.Background()); errs["notify_email"] == "" {
		t.Errorf("invalid: %v", errs)
	}
}

// TestWebformLimitsAndConsent - the limits are read in the site's zone;
// a consent element is always required and keeps its link as its option;
// bad limits and links are refused.
func TestWebformLimitsAndConsent(t *testing.T) {
	moscow := time.FixedZone("MSK", 3*3600)
	f := WebformForm{Title: "T", MaxSubmissions: "100", ClosesAt: "2026-11-01T18:00", LimitPerUser: true,
		Elements: []WebformElementInput{{Key: "new1", Type: WebformConsent, Label: "I agree", Link: "/privacy"}}}
	if errs := f.Validate(context.Background()); len(errs) != 0 {
		t.Fatalf("valid: %v", errs)
	}
	limit, closes := f.Limits(moscow)
	if limit != 100 || closes == nil || !closes.Equal(time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)) {
		t.Errorf("limits: %d %v", limit, closes)
	}
	els := f.WebformElements()
	if !els[0].Required || els[0].ConsentLink() != "/privacy" {
		t.Errorf("consent: %+v", els[0])
	}
	back := WebformFormFromEntity(&Webform{MaxSubmissions: 100, ClosesAt: closes, Elements: els}, moscow)
	if back.MaxSubmissions != "100" || back.ClosesAt != "2026-11-01T18:00" || back.Elements[0].Link != "/privacy" || back.Elements[0].Options != "" {
		t.Errorf("from entity: %+v", back)
	}
	bad := WebformForm{Title: "T", MaxSubmissions: "-1", ClosesAt: "tomorrow",
		Elements: []WebformElementInput{{Key: "new1", Type: WebformConsent, Label: "I agree", Link: "javascript:alert(1)"}}}
	errs := bad.Validate(context.Background())
	if errs["max_submissions"] == "" || errs["closes_at"] == "" || errs["element_new1"] == "" {
		t.Errorf("invalid: %v", errs)
	}
}

// TestWebformFormConditions - a row's condition on an earlier row becomes
// its position; one on a later row is dropped; stored conditions come back
// as the target's row key.
func TestWebformFormConditions(t *testing.T) {
	f := WebformForm{Elements: []WebformElementInput{
		{Key: "7", ID: 7, Type: WebformRadios, Label: "Topic", Options: "Ads\nOther", IfKey: "new1"},
		{Key: "new1", Type: WebformText, Label: "Budget", IfKey: "7", IfValue: "Ads"},
	}}
	els := f.WebformElements()
	if els[0].VisibleIfRef != 0 || els[1].VisibleIfRef != 1 || els[1].VisibleIfValue != "Ads" {
		t.Errorf("refs: %+v", els)
	}
	back := WebformFormFromEntity(&Webform{Elements: WebformElements{
		{ID: 7, MachineName: "topic", Type: WebformRadios, Label: "Topic", Options: []string{"Ads"}},
		{ID: 8, MachineName: "budget", Type: WebformText, Label: "Budget", VisibleIfElement: "topic", VisibleIfValue: "Ads"},
	}}, time.UTC)
	if back.Elements[1].IfKey != "7" || back.Elements[1].IfValue != "Ads" || back.Elements[0].IfKey != "" {
		t.Errorf("from entity: %+v", back.Elements)
	}
}

// TestWebformFormFileExtensions - a file row's extensions, typed with
// dots, commas or spaces, are stored lowercase without dots and come back
// joined.
func TestWebformFormFileExtensions(t *testing.T) {
	f := WebformForm{Elements: []WebformElementInput{{Key: "new1", Type: WebformFile, Label: "CV", Ext: ".PDF, docx  jpg;"}}}
	els := f.WebformElements()
	if got := strings.Join(els[0].Options, " "); got != "pdf docx jpg" {
		t.Errorf("options: %q", got)
	}
	back := WebformFormFromEntity(&Webform{Elements: els}, time.UTC)
	if back.Elements[0].Ext != "pdf, docx, jpg" || back.Elements[0].Options != "" {
		t.Errorf("from entity: %+v", back.Elements[0])
	}
}

// TestWebformFormNumberAndConfirmation - a number row's limits are stored
// as min=/max= and come back; bad limits and a page after sending off the
// site are errors.
func TestWebformFormNumberAndConfirmation(t *testing.T) {
	f := WebformForm{Title: "T", Elements: []WebformElementInput{{Key: "new1", Type: WebformNumber, Label: "Age", Max: "99,5"}}}
	els := f.WebformElements()
	if strings.Join(els[0].Options, " ") != "max=99.5" {
		t.Errorf("options: %v", els[0].Options)
	}
	back := WebformFormFromEntity(&Webform{Elements: els}, time.UTC)
	if back.Elements[0].Min != "" || back.Elements[0].Max != "99.5" || back.Elements[0].Options != "" {
		t.Errorf("from entity: %+v", back.Elements[0])
	}
	ctx := context.Background()
	if errs := f.Validate(ctx); len(errs) != 0 {
		t.Errorf("valid: %v", errs)
	}
	for _, url := range []string{"thanks", "//evil.example", "https://evil.example", "/a b", `/\evil.example`} {
		g := f
		g.ConfirmationURL = url
		if g.Validate(ctx)["confirmation_url"] == "" {
			t.Errorf("%q must be refused", url)
		}
	}
	g := f
	g.ConfirmationURL = "/thanks?from=form"
	if errs := g.Validate(ctx); len(errs) != 0 {
		t.Errorf("site path: %v", errs)
	}
	g = f
	g.Elements = []WebformElementInput{{Key: "new1", Type: WebformNumber, Label: "Age", Min: "10", Max: "5"}}
	if g.Validate(ctx)["element_new1"] == "" {
		t.Error("min above max must be refused")
	}
}

// TestWebformFormPrefill - a question's address parameter is kept (a
// file's or a consent's dropped), checked (Latin letters, digits, "_",
// "-") and comes back to the editor.
func TestWebformFormPrefill(t *testing.T) {
	f := WebformForm{Title: "Order", Elements: []WebformElementInput{
		{Key: "new1", Type: WebformText, Label: "Product", Prefill: "product"},
		{Key: "new2", Type: WebformFile, Label: "Drawing", Prefill: "drawing"},
		{Key: "new3", Type: WebformSelect, Label: "Size", Options: "S\nM", Prefill: "size"},
	}}
	if errs := f.Validate(context.Background()); len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	els := f.WebformElements()
	if els[0].PrefillParam != "product" || els[1].PrefillParam != "" || els[2].PrefillParam != "size" {
		t.Errorf("params: %+v", els)
	}
	f.Elements[0].Prefill = "pro duct"
	if errs := f.Validate(context.Background()); errs["element_new1"] == "" {
		t.Error("an invalid parameter passed")
	}
	back := WebformFormFromEntity(&Webform{Elements: WebformElements{{ID: 4, Type: WebformText, Label: "Product", PrefillParam: "product"}}}, time.UTC)
	if back.Elements[0].Prefill != "product" {
		t.Errorf("from entity: %+v", back.Elements[0])
	}
}
