package forms

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

type fakeWebformRepo struct {
	WebformRepository
	stored   Webform
	updated  Webform
	sub      *WebformSubmission
	existing *Webform
}

// Create and Update keep the form as ByNodeID then returns it.
func (f *fakeWebformRepo) Create(_ context.Context, w Webform) (int, error) {
	f.stored = w
	w.Node.ID = 7
	f.existing = &w
	return 7, nil
}

func (f *fakeWebformRepo) Update(_ context.Context, w Webform) error {
	f.updated = w
	f.existing = &w
	return nil
}

func (f *fakeWebformRepo) ByNodeID(context.Context, int) (*Webform, error) {
	return f.existing, nil
}

func (f *fakeWebformRepo) AddSubmission(_ context.Context, s WebformSubmission) (int, error) {
	f.sub = &s
	return 1, nil
}

func testWebformElements() WebformElements {
	return WebformElements{
		{ID: 1, Type: WebformText, Label: "Name", Required: true},
		{ID: 2, Type: WebformEmail, Label: "E-mail"},
		{ID: 3, Type: WebformRadios, Label: "Topic", Options: []string{"Ads", "Errors"}},
		{ID: 4, Type: WebformCheckboxes, Label: "Sections", Options: []string{"News", "Movies", "Games"}},
	}
}

// TestValidateWebformInput - values are trimmed and empty ones dropped; a
// required element needs one, an e-mail element a valid address, a choice
// one of its options, a single-value element one value.
func TestValidateWebformInput(t *testing.T) {
	values, errs := ValidateWebformInput(testWebformElements(), map[int][]string{
		1: {"  Ann "}, 2: {"ann@example.test"}, 3: {"Errors"}, 4: {"News", "Games", ""},
	})
	if len(errs) != 0 || values[1][0] != "Ann" || len(values[4]) != 2 {
		t.Fatalf("valid: %v %v", values, errs)
	}
	_, errs = ValidateWebformInput(testWebformElements(), map[int][]string{
		1: {"   "}, 2: {"not an address"}, 3: {"Ads", "Errors"}, 4: {"Books"},
	})
	want := map[int]string{1: "This field is required.", 2: "Enter a valid e-mail address.", 3: "Choose one value.", 4: "Choose one of the options."}
	for id, msg := range want {
		if errs[id] != msg {
			t.Errorf("element %d: %q, want %q", id, errs[id], msg)
		}
	}
	if _, errs := ValidateWebformInput(testWebformElements()[:1], map[int][]string{1: {strings.Repeat("a", webformValueMaxLen+1)}}); errs[1] != "The value is too long." {
		t.Errorf("too long: %v", errs)
	}
}

// TestWebformMachineNames - new elements get names from their labels
// (transliterated, unique, starting with a letter); existing ones keep
// theirs on update.
func TestWebformMachineNames(t *testing.T) {
	repo := &fakeWebformRepo{}
	svc := NewWebformService(repo, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := Webform{Node: modulekit.Node{Title: "T", Slug: "form/t"}, Elements: WebformElements{
		{Label: "Ваше имя"}, {Label: "Ваше имя"}, {Label: "2024 year"},
	}}
	if _, err := svc.Create(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	got := []string{repo.stored.Elements[0].MachineName, repo.stored.Elements[1].MachineName, repo.stored.Elements[2].MachineName}
	if got[0] != "vashe_imya" || got[1] != "vashe_imya_2" || got[2] != "element_2024_year" {
		t.Errorf("names: %v", got)
	}

	repo.existing = &Webform{Elements: WebformElements{{ID: 5, MachineName: "vashe_imya"}}}
	upd := Webform{Node: modulekit.Node{ID: 9}, Elements: WebformElements{{ID: 5, Label: "Имя"}, {Label: "Ваше имя"}}}
	if err := svc.Update(context.Background(), upd); err != nil {
		t.Fatal(err)
	}
	if repo.updated.Elements[0].MachineName != "vashe_imya" || repo.updated.Elements[1].MachineName != "vashe_imya_2" {
		t.Errorf("update names: %+v", repo.updated.Elements)
	}
}

// TestWebformSubmit - a closed or unpublished form takes nothing; field
// errors store nothing; valid values are stored with the sender.
func TestWebformSubmit(t *testing.T) {
	repo := &fakeWebformRepo{}
	svc := NewWebformService(repo, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	w := Webform{ID: 3, Node: modulekit.Node{ID: 9, Status: 1}, Elements: testWebformElements()}

	closed := w
	closed.Closed = true
	if _, err := svc.Submit(ctx, closed, nil, nil, WebformSender{}); !errors.Is(err, ErrWebformClosed) {
		t.Errorf("closed: %v", err)
	}
	hidden := w
	hidden.Node.Status = 0
	if _, err := svc.Submit(ctx, hidden, nil, nil, WebformSender{}); !errors.Is(err, ErrWebformUnavailable) {
		t.Errorf("unpublished: %v", err)
	}
	if errs, err := svc.Submit(ctx, w, map[int][]string{}, nil, WebformSender{}); err != nil || errs[1] == "" || repo.sub != nil {
		t.Errorf("missing required: %v %v %v", errs, err, repo.sub)
	}
	uid := 4
	if errs, err := svc.Submit(ctx, w, map[int][]string{1: {"Ann"}, 4: {"News"}}, nil, WebformSender{UserID: &uid, RemoteIP: "10.0.0.1"}); err != nil || len(errs) != 0 {
		t.Fatalf("valid: %v %v", errs, err)
	}
	if repo.sub == nil || repo.sub.WebformID != 3 || *repo.sub.UserID != 4 || repo.sub.RemoteIP != "10.0.0.1" || repo.sub.Values[1][0] != "Ann" {
		t.Errorf("stored: %+v", repo.sub)
	}
}

// TestWebformNotification - a submission e-mails every notify address
// the answered elements as "Label: value", the page it was sent from and
// the submissions link; no addresses or no notifier - nothing is sent.
func TestWebformNotification(t *testing.T) {
	repo := &fakeWebformRepo{}
	svc := NewWebformService(repo, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.wait = true
	type mail struct{ to, subject, body string }
	var sent []mail
	svc.SetNotifier(func(to, subject, body string) error {
		sent = append(sent, mail{to, subject, body})
		return nil
	})
	w := Webform{ID: 3, Node: modulekit.Node{ID: 9, Status: 1, Title: "Contact"}, Elements: testWebformElements(),
		NotifyEmail: "a@example.test, b@example.test"}
	if _, err := svc.Submit(context.Background(), w, map[int][]string{1: {"Ann"}, 4: {"News", "Games"}}, nil, WebformSender{Source: "/catalog/tank"}); err != nil {
		t.Fatal(err)
	}
	if repo.sub == nil || repo.sub.SourceURL != "/catalog/tank" {
		t.Errorf("the submission's source: %+v", repo.sub)
	}
	if len(sent) != 2 || sent[0].to != "a@example.test" || sent[1].to != "b@example.test" || sent[0].subject != "New submission: Contact" {
		t.Fatalf("sent: %+v", sent)
	}
	for _, want := range []string{"Name: Ann\n", "Sections: News, Games\n", "Sent from the page: " + modulekit.SiteDomain() + "/catalog/tank\n", "/node/9/form/submissions"} {
		if !strings.Contains(sent[0].body, want) {
			t.Errorf("body has no %q: %s", want, sent[0].body)
		}
	}
	if strings.Contains(sent[0].body, "E-mail:") {
		t.Error("an unanswered element must not be listed")
	}
	sent = nil
	w.NotifyEmail = ""
	if _, err := svc.Submit(context.Background(), w, map[int][]string{1: {"Ann"}}, nil, WebformSender{}); err != nil || len(sent) != 0 {
		t.Errorf("no addresses: %v %+v", err, sent)
	}
	if got := (Webform{NotifyEmail: " a@x.test;b@x.test\nc@x.test, "}).NotifyAddresses(); len(got) != 3 || got[2] != "c@x.test" {
		t.Errorf("addresses: %q", got)
	}
}

type fakeLimitRepo struct {
	fakeWebformRepo
	count int
	sent  bool
}

func (f *fakeLimitRepo) CountSubmissions(context.Context, int) (int, error) { return f.count, nil }
func (f *fakeLimitRepo) HasUserSubmission(context.Context, int, int) (bool, error) {
	return f.sent, nil
}

// TestWebformAvailability - a form takes no submission once its closing
// time has come or its limit is reached; a one-per-user form asks visitors
// to log in and refuses a user who has sent it.
func TestWebformAvailability(t *testing.T) {
	repo := &fakeLimitRepo{}
	svc := NewWebformService(repo, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	open := Webform{ID: 1, Node: modulekit.Node{Status: 1}}
	uid := 5
	if err := svc.Availability(ctx, open, nil, now); err != nil {
		t.Fatalf("open: %v", err)
	}
	past, future := now.Add(-time.Minute), now.Add(time.Hour)
	for name, tc := range map[string]struct {
		w    Webform
		user *int
		repo fakeLimitRepo
		want error
	}{
		"closing time passed":   {Webform{Node: modulekit.Node{Status: 1}, ClosesAt: &past}, nil, fakeLimitRepo{}, ErrWebformClosed},
		"closing time ahead":    {Webform{Node: modulekit.Node{Status: 1}, ClosesAt: &future}, nil, fakeLimitRepo{}, nil},
		"limit reached":         {Webform{Node: modulekit.Node{Status: 1}, MaxSubmissions: 3}, nil, fakeLimitRepo{count: 3}, ErrWebformClosed},
		"limit not reached":     {Webform{Node: modulekit.Node{Status: 1}, MaxSubmissions: 3}, nil, fakeLimitRepo{count: 2}, nil},
		"one per user, visitor": {Webform{Node: modulekit.Node{Status: 1}, LimitPerUser: true}, nil, fakeLimitRepo{}, ErrWebformLoginRequired},
		"one per user, sent":    {Webform{Node: modulekit.Node{Status: 1}, LimitPerUser: true}, &uid, fakeLimitRepo{sent: true}, ErrWebformAlreadySent},
		"one per user, first":   {Webform{Node: modulekit.Node{Status: 1}, LimitPerUser: true}, &uid, fakeLimitRepo{}, nil},
	} {
		*repo = tc.repo
		if err := svc.Availability(ctx, tc.w, tc.user, now); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", name, err, tc.want)
		}
	}
}

// TestWebformConsent - a consent element must be ticked (its one value
// "1"), required or not; the notification says "Yes".
func TestWebformConsent(t *testing.T) {
	els := WebformElements{{ID: 7, Type: WebformConsent, Label: "I agree"}}
	if _, errs := ValidateWebformInput(els, map[int][]string{}); errs[7] != "Tick the checkbox to send the form." {
		t.Errorf("unticked: %v", errs)
	}
	if _, errs := ValidateWebformInput(els, map[int][]string{7: {"yes"}}); errs[7] == "" {
		t.Error("a value other than 1 must be refused")
	}
	values, errs := ValidateWebformInput(els, map[int][]string{7: {"1"}})
	if len(errs) != 0 || values[7][0] != "1" {
		t.Errorf("ticked: %v %v", values, errs)
	}
	_, body := WebformNotification(Webform{Node: modulekit.Node{Title: "F"}, Elements: els}, values, "")
	if !strings.Contains(body, "I agree: Yes\n") {
		t.Errorf("notification: %s", body)
	}
}

// TestWebformConditions - an element shown only for an answer of an
// earlier one is checked and stored only when shown; a chain hides with
// its first link; editor references become machine names.
func TestWebformConditions(t *testing.T) {
	els := WebformElements{
		{ID: 1, MachineName: "topic", Type: WebformRadios, Label: "Topic", Options: []string{"Ads", "Other"}},
		{ID: 2, MachineName: "budget", Type: WebformText, Label: "Budget", Required: true, VisibleIfElement: "topic", VisibleIfValue: "Ads"},
		{ID: 3, MachineName: "details", Type: WebformText, Label: "Details", Required: true, VisibleIfElement: "budget"},
	}
	values, errs := ValidateWebformInput(els, map[int][]string{1: {"Other"}, 2: {"100"}, 3: {"x"}})
	if len(errs) != 0 || values[2] != nil || values[3] != nil || values[1][0] != "Other" {
		t.Errorf("hidden by Other: %v %v", values, errs)
	}
	_, errs = ValidateWebformInput(els, map[int][]string{1: {"Ads"}})
	if errs[2] == "" || errs[3] != "" {
		t.Errorf("Ads without budget: %v", errs)
	}
	values, errs = ValidateWebformInput(els, map[int][]string{1: {"Ads"}, 2: {"100"}, 3: {"soon"}})
	if len(errs) != 0 || values[3][0] != "soon" {
		t.Errorf("all shown: %v %v", values, errs)
	}

	repo := &fakeWebformRepo{}
	svc := NewWebformService(repo, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := Webform{Node: modulekit.Node{Title: "T", Slug: "form/t"}, Elements: WebformElements{
		{Label: "Topic"}, {Label: "Budget", VisibleIfRef: 1, VisibleIfValue: "Ads"}, {Label: "Self", VisibleIfRef: 3, VisibleIfValue: "x"},
	}}
	if _, err := svc.Create(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	got := repo.stored.Elements
	if got[1].VisibleIfElement != "topic" || got[1].VisibleIfValue != "Ads" || got[2].VisibleIfElement != "" || got[2].VisibleIfValue != "" {
		t.Errorf("resolved: %+v", got)
	}
}

// TestValidateWebformTypedInput - a number within its limits (a comma
// too), a real date, a phone of digits and the usual separators.
func TestValidateWebformTypedInput(t *testing.T) {
	els := WebformElements{
		{ID: 1, Type: WebformNumber, Label: "Age", Options: []string{"min=18", "max=99"}},
		{ID: 2, Type: WebformDate, Label: "Day"},
		{ID: 3, Type: WebformPhone, Label: "Phone"},
		{ID: 4, Type: WebformNumber, Label: "Any"},
	}
	for _, c := range []struct {
		in    map[int][]string
		wrong []int
	}{
		{map[int][]string{1: {"42"}, 2: {"2026-02-28"}, 3: {"+7 (900) 123-45-67"}, 4: {"-1,5"}}, nil},
		{map[int][]string{1: {"17"}, 2: {"2026-02-30"}, 3: {"call me"}, 4: {"1e999"}}, []int{1, 2, 3, 4}},
		{map[int][]string{1: {"100"}, 2: {"28.02.2026"}, 3: {"1234"}, 4: {"NaN"}}, []int{1, 2, 3, 4}},
	} {
		_, errs := ValidateWebformInput(els, c.in)
		if len(errs) != len(c.wrong) {
			t.Errorf("%v: errors %v, want on %v", c.in, errs, c.wrong)
		}
		for _, id := range c.wrong {
			if errs[id] == "" {
				t.Errorf("%v: no error on %d", c.in, id)
			}
		}
	}
}

// TestWebformActions - a form created is "Content created" in the action
// log, one published on save "Content published", the object the form's
// content.
func TestWebformActions(t *testing.T) {
	got := testkit.CaptureActions(t)
	repo := &fakeWebformRepo{}
	svc := NewWebformService(repo, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := svc.Create(context.Background(), Webform{Node: modulekit.Node{Title: "Feedback", Slug: "form/feedback", Type: "webform"}}); err != nil {
		t.Fatal(err)
	}
	upd := *repo.existing
	upd.Node.Status = 1
	if err := svc.Update(context.Background(), upd); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 || (*got)[0].Type != "node.create" || (*got)[1].Type != "node.publish" {
		t.Fatalf("actions: %+v", *got)
	}
	if o := (*got)[0].Object; o.Kind != "node" || o.ID != "7" || o.Label != "Feedback" {
		t.Errorf("object: %+v", o)
	}
}

// TestPreviewElements - a previewed form's elements: a saved one keeps
// its machine name, a new one is named and numbered after every other,
// a condition on it resolves to its name.
func TestPreviewElements(t *testing.T) {
	saved := WebformElements{{ID: 4, MachineName: "topic", Label: "Topic"}, {ID: 7, MachineName: "gone", Label: "Gone"}}
	elements := WebformElements{{ID: 4, Label: "Topic"}, {Label: "Budget", VisibleIfRef: 1, VisibleIfValue: "Ads"}, {Label: "Note", VisibleIfRef: 2}}
	previewElements(elements, saved)
	if elements[0].MachineName != "topic" || elements[0].ID != 4 {
		t.Errorf("the saved one: %+v", elements[0])
	}
	if elements[1].ID != 8 || elements[2].ID != 9 || elements[1].MachineName != "budget" {
		t.Errorf("the new ones: %+v %+v", elements[1], elements[2])
	}
	if elements[1].VisibleIfElement != "topic" || elements[2].VisibleIfElement != "budget" {
		t.Errorf("the conditions: %q, %q", elements[1].VisibleIfElement, elements[2].VisibleIfElement)
	}
}
