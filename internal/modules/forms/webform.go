package forms

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Webform - a form of the forms module: a node of type "form" (title,
// address, SEO, comments) with its own elements.
type Webform struct {
	ID   int
	Node modulekit.Node
	// Description - the text above the form (plain text).
	Description string
	// Confirmation - the message after a submission (plain text; "" -
	// the default one).
	Confirmation string
	// ConfirmationURL - the site path a submission leads to instead of
	// the message ("" - the message).
	ConfirmationURL string
	// SubmitLabel - the submit button's text ("" - the default).
	SubmitLabel string
	// Closed - the form takes no submissions.
	Closed bool
	// NotifyEmail - addresses told about every submission, comma
	// separated ("" - nobody).
	NotifyEmail string
	// LimitPerUser - one submission per signed-in user; visitors sign in
	// to send it.
	LimitPerUser bool
	// MaxSubmissions - the form closes after this many (0 - no limit).
	MaxSubmissions int
	// ClosesAt - the form closes at this time (nil - never).
	ClosesAt *time.Time
	Elements WebformElements
}

// NotifyAddresses - NotifyEmail's addresses, trimmed, empty ones dropped.
func (w Webform) NotifyAddresses() []string {
	var out []string
	for _, a := range strings.FieldsFunc(w.NotifyEmail, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' }) {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

// WebformElementType - what an element asks for.
type WebformElementType string

// The element types (WebformElement.Type): a line of text, a longer
// text, an e-mail address, one choice from a list or of radio buttons,
// any of several checkboxes, and the ones described below.
const (
	WebformText       WebformElementType = "text"
	WebformTextarea   WebformElementType = "textarea"
	WebformEmail      WebformElementType = "email"
	WebformSelect     WebformElementType = "select"
	WebformRadios     WebformElementType = "radios"
	WebformCheckboxes WebformElementType = "checkboxes"
	// WebformConsent - a checkbox the sender must tick (consent to the
	// processing of personal data), with an optional link to the policy.
	WebformConsent WebformElementType = "consent"
	// WebformFile - a file sent with the form, kept in the private files
	// directory and given only to those who may view submissions; its
	// Options are the allowed extensions.
	WebformFile WebformElementType = "file"
	// WebformNumber - a number; its Options are the limits, "min=<n>" and
	// "max=<n>" (each optional).
	WebformNumber WebformElementType = "number"
	// WebformDate - a date (YYYY-MM-DD).
	WebformDate WebformElementType = "date"
	// WebformPhone - a phone number.
	WebformPhone WebformElementType = "tel"
)

// WebformElementTypes - the element types, in the editor's order.
var WebformElementTypes = []WebformElementType{WebformText, WebformTextarea, WebformEmail, WebformSelect, WebformRadios, WebformCheckboxes, WebformConsent, WebformFile, WebformNumber, WebformDate, WebformPhone}

// webformElementTypeLabels - the types as the editor names them
// (translation sources).
var webformElementTypeLabels = map[WebformElementType]string{
	WebformText:       i18n.N("Single line"),
	WebformTextarea:   i18n.N("Text"),
	WebformEmail:      i18n.N("E-mail"),
	WebformSelect:     i18n.N("Drop-down list"),
	WebformRadios:     i18n.N("One of the options"),
	WebformCheckboxes: i18n.N("Several of the options"),
	WebformConsent:    i18n.N("Consent (a checkbox to tick)"),
	WebformFile:       i18n.N("File"),
	WebformNumber:     i18n.N("Number"),
	WebformDate:       i18n.N("Date"),
	WebformPhone:      i18n.N("Phone"),
}

// InputType - the HTML input type of a one-line element.
func (t WebformElementType) InputType() string {
	switch t {
	case WebformEmail, WebformNumber, WebformDate, WebformPhone:
		return string(t)
	}
	return "text"
}

// Label - the type's name in the editor (a translation source).
func (t WebformElementType) Label() string { return webformElementTypeLabels[t] }

// HasOptions - the element is a choice among its Options.
func (t WebformElementType) HasOptions() bool {
	return t == WebformSelect || t == WebformRadios || t == WebformCheckboxes
}

// Multiple - the element takes several values (one per checked choice).
func (t WebformElementType) Multiple() bool { return t == WebformCheckboxes }

// Valid - a known element type.
func (t WebformElementType) Valid() bool {
	for _, v := range WebformElementTypes {
		if v == t {
			return true
		}
	}
	return false
}

// WebformElement - one element (question) of a form.
type WebformElement struct {
	ID int
	// MachineName - the element's key (exports, e-mails), unique in its
	// form and kept when the label changes.
	MachineName string
	Type        WebformElementType
	Label       string
	Description string
	Required    bool
	// Options - the choices of a select/radios/checkboxes element.
	Options []string
	Weight  int
	// VisibleIfElement / VisibleIfValue - shown only when the earlier
	// element with this machine name has this value ("" - any value);
	// VisibleIfElement "" - always shown.
	VisibleIfElement string
	VisibleIfValue   string
	// VisibleIfRef - the editor's reference to that element: its position
	// in the list plus one (0 - none); the service turns it into
	// VisibleIfElement once machine names are assigned.
	VisibleIfRef int
	// PrefillParam - the address's query parameter whose value fills the
	// element when the form opens ("" - none; forms.js fills it, the
	// server validates the answer as any other).
	PrefillParam string
}

// ConsentValue - the stored value of a ticked consent element.
const ConsentValue = "1"

// ConsentLink - a consent element's link to its policy ("" - none).
func (e WebformElement) ConsentLink() string {
	if e.Type == WebformConsent && len(e.Options) > 0 {
		return e.Options[0]
	}
	return ""
}

// OptionsText - Options one per line (the editor's textarea).
func (e WebformElement) OptionsText() string { return strings.Join(e.Options, "\n") }

// ParseWebformOptions - the non-empty trimmed lines of text.
func ParseWebformOptions(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// WebformElements - a form's elements, in their order (Webform.Elements).
type WebformElements []WebformElement

// WebformSubmission - one sent form: values by element ID (a checkboxes
// element has one per checked choice).
type WebformSubmission struct {
	ID        int
	WebformID int
	// UserID - the signed-in sender, nil - anonymous.
	UserID *int
	// Username, UserSlug - the sender's, when listed ("" - anonymous or
	// deleted).
	Username string
	UserSlug string
	RemoteIP string
	// SourceURL - the site's page (a path) the form was sent from: the
	// page it's embedded in, or the one linking to its own page ("" -
	// unknown).
	SourceURL string
	CreatedAt time.Time
	Values    map[int][]string
}

// WebformRepository - the forms module's storage.
type WebformRepository interface {
	ByNodeID(ctx context.Context, nodeID int) (*Webform, error)
	BySlug(ctx context.Context, slug string) (*Webform, error)
	// Create stores the form's node, form row and elements; returns the
	// node ID.
	Create(ctx context.Context, w Webform) (int, error)
	// Update stores the node, the form row and the elements: new ones are
	// added, changed ones updated, missing ones deleted with their values.
	Update(ctx context.Context, w Webform) error
	// AddSubmission stores a submission with its values; returns its ID.
	AddSubmission(ctx context.Context, s WebformSubmission) (int, error)
	// List - every form (node ID and title in Node), by title.
	List(ctx context.Context) ([]Webform, error)
	// SubmissionStats - per form ID: its submissions and the last one's
	// time (forms without any are absent).
	SubmissionStats(ctx context.Context) (map[int]WebformStats, error)
	// CountSubmissions - the form's submissions.
	CountSubmissions(ctx context.Context, webformID int) (int, error)
	// DeleteSubmissions deletes those of the submissions (IDs) that belong
	// to the form, with their values; returns how many.
	DeleteSubmissions(ctx context.Context, webformID int, ids []int) (int, error)
	// SubmissionValues - the stored values of these elements in these
	// submissions (the files to remove with them).
	SubmissionValues(ctx context.Context, submissionIDs, elementIDs []int) ([]string, error)
	// SubmissionValue - one value of a submission of the form (nil error,
	// "" - none).
	SubmissionValue(ctx context.Context, webformID, submissionID, elementID, delta int) (string, error)
	// AnswerCounts - per element of elementIDs: the submissions with an
	// answer; per element of choiceIDs also each stored value's count.
	AnswerCounts(ctx context.Context, elementIDs, choiceIDs []int) (WebformAnswerCounts, error)
	// SubmissionIDsBefore - the form's submissions sent before t.
	SubmissionIDsBefore(ctx context.Context, webformID int, t time.Time) ([]int, error)
	// Submission - one submission of the form with its values and sender
	// (nil, nil - none).
	Submission(ctx context.Context, webformID, id int) (*WebformSubmission, error)
	// SubmissionNeighbors - the submissions next to s in the list's order
	// (newest first): the newer and the older one (0 - none).
	SubmissionNeighbors(ctx context.Context, s WebformSubmission) (newer, older int, err error)
	// HasUserSubmission - whether the user has sent the form.
	HasUserSubmission(ctx context.Context, webformID, userID int) (bool, error)
	// Submissions - the form's submissions with their values and senders,
	// newest first.
	Submissions(ctx context.Context, webformID, limit, offset int) ([]WebformSubmission, error)
}

// WebformAnswerCounts - a form's answers counted (the analysis page).
type WebformAnswerCounts struct {
	// Answered - element ID → submissions with an answer.
	Answered map[int]int
	// Values - choice element ID → value → times chosen.
	Values map[int]map[string]int
}

// WebformStats - a form's submissions at a glance (the admin list).
type WebformStats struct {
	Count int
	Last  time.Time
}

// Visible - which elements are shown for these values (by element ID): an
// element with a condition is shown when the earlier element it names is
// shown and has the value (any value when VisibleIfValue is empty); a
// condition naming no earlier element never hides.
func (els WebformElements) Visible(values map[int][]string) map[int]bool {
	visible := make(map[int]bool, len(els))
	earlier := map[string]WebformElement{}
	for _, e := range els {
		show := true
		if e.VisibleIfElement != "" {
			if target, ok := earlier[e.VisibleIfElement]; ok {
				show = visible[target.ID] && webformHasValue(values[target.ID], e.VisibleIfValue)
			}
		}
		visible[e.ID] = show
		earlier[e.MachineName] = e
	}
	return visible
}

// webformHasValue - whether vals (non-empty ones) include want; "" - any.
func webformHasValue(vals []string, want string) bool {
	for _, v := range vals {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if want == "" || v == want {
			return true
		}
	}
	return false
}

// ElementIDByName - the ID of the element with this machine name (0 -
// none).
func (w Webform) ElementIDByName(name string) int {
	for _, e := range w.Elements {
		if e.MachineName == name {
			return e.ID
		}
	}
	return 0
}

// WebformDefaultFileExtensions - the extensions a file element allows when
// it lists none: documents and pictures.
var WebformDefaultFileExtensions = []string{"pdf", "doc", "docx", "odt", "rtf", "txt", "xls", "xlsx", "ods", "jpg", "jpeg", "png", "gif", "webp"}

// FileExtensions - a file element's allowed extensions (lowercase, no
// dot): its Options, else WebformDefaultFileExtensions.
func (e WebformElement) FileExtensions() []string {
	if len(e.Options) == 0 {
		return WebformDefaultFileExtensions
	}
	out := make([]string, 0, len(e.Options))
	for _, o := range e.Options {
		if o = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(o), ".")); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// NumberRange - a number element's limits as stored ("" - none).
func (e WebformElement) NumberRange() (lowest, highest string) {
	if e.Type != WebformNumber {
		return "", ""
	}
	for _, o := range e.Options {
		if v, ok := strings.CutPrefix(o, "min="); ok {
			lowest = v
		}
		if v, ok := strings.CutPrefix(o, "max="); ok {
			highest = v
		}
	}
	return lowest, highest
}

// NumberMin / NumberMax - NumberRange's parts (for templates).
func (e WebformElement) NumberMin() string { lowest, _ := e.NumberRange(); return lowest }

// NumberMax - the highest of NumberRange (for templates).
func (e WebformElement) NumberMax() string { _, highest := e.NumberRange(); return highest }

// FileTypes - the allowed extensions as the file input's accept and the
// hint show them: ".pdf, .docx".
func (e WebformElement) FileTypes() string {
	ext := e.FileExtensions()
	out := make([]string, len(ext))
	for i, x := range ext {
		out[i] = "." + x
	}
	return strings.Join(out, ", ")
}

// WebformUpload - a file sent for a file element.
type WebformUpload struct {
	Name string
	Data []byte
}

// WebformFileValue - the stored value of a sent file: its path in the
// private files directory and its original name.
func WebformFileValue(path, name string) string { return path + "\n" + name }

// ParseWebformFileValue - the path and the original name of a stored file
// value.
func ParseWebformFileValue(v string) (path, name string) {
	path, name, _ = strings.Cut(v, "\n")
	return path, name
}

// HasFileElement - whether the form sends files (a multipart form).
func (w Webform) HasFileElement() bool {
	for _, e := range w.Elements {
		if e.Type == WebformFile {
			return true
		}
	}
	return false
}

// FileName - a file element's value as shown: the sent file's original
// name (other elements: the value).
func (e WebformElement) FileName(v string) string {
	if e.Type != WebformFile {
		return v
	}
	_, name := ParseWebformFileValue(v)
	return name
}

// WebformSender - who sent a submission and from where: the signed-in
// user (nil - anonymous), the address, the site's page (a path, "" -
// unknown).
type WebformSender struct {
	UserID   *int
	RemoteIP string
	Source   string
}

// webformSourceMax - the longest source kept (the column's size).
const webformSourceMax = 500

// CleanWebformSource - raw (a path, or an address of one of hosts) as the
// path of the site's page a submission came from: "/…" with its query, no
// fragment; "" - not a page of this site, or too long.
func CleanWebformSource(raw string, hosts ...string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, " \\\t\r\n") {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme != "" || u.Host != "" {
		if (u.Scheme != "http" && u.Scheme != "https") || !sameWebformHost(u.Host, hosts) {
			return ""
		}
	}
	path := u.EscapedPath()
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return ""
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	if len(path) > webformSourceMax {
		return ""
	}
	return path
}

// sameWebformHost - host is one of hosts (a bare host or an address).
func sameWebformHost(host string, hosts []string) bool {
	for _, h := range hosts {
		if u, err := url.Parse(h); err == nil && u.Host != "" {
			h = u.Host
		}
		if h != "" && strings.EqualFold(h, host) {
			return true
		}
	}
	return false
}

// webformPrefillParam - a query parameter's name an element is filled
// from: letters, digits, "_" and "-".
var webformPrefillParam = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// CanPrefill - the element type can be filled from the address (not a
// file, not a consent).
func (t WebformElementType) CanPrefill() bool {
	return t != WebformFile && t != WebformConsent
}
