package forms

import (
	"context"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// WebformElementInput - one element row of the form editor.
type WebformElementInput struct {
	// Key - the row's key in the request: the element ID, or "new…" for
	// a row added in the editor.
	Key         string
	ID          int
	Type        WebformElementType
	Label       string
	Description string
	Required    bool
	// Options - one per line.
	Options string
	// Link - a consent element's policy link.
	Link string
	// Ext - a file element's allowed extensions, as typed ("pdf, docx").
	Ext string
	// Min / Max - a number element's limits, as typed ("" - none).
	Min, Max string
	// IfKey / IfValue - shown only when the earlier row with this key has
	// this value ("" key - always).
	IfKey   string
	IfValue string
	// Prefill - the address's query parameter that fills the element ("" -
	// none).
	Prefill string
}

// WebformForm - the create/edit form of a form (node/edit_webform.html).
// Element rows are posted as element_row (their keys, in order) and
// element_<field>_<key>, so an unchecked "required" box can't shift rows.
type WebformForm struct {
	modulekit.NodeMetaFields
	NodeID       int
	Title        string
	Slug         string
	Description  string
	Confirmation string
	// ConfirmationURL - the path a submission leads to ("" - the message).
	ConfirmationURL string
	SubmitLabel     string
	Closed          bool
	NotifyEmail     string
	LimitPerUser    bool
	// MaxSubmissions - as typed ("" - no limit).
	MaxSubmissions string
	// ClosesAt - datetime-local in the site's zone ("" - never).
	ClosesAt     string
	Status       int
	OnModeration int
	CommentMode  string
	Elements     []WebformElementInput
}

// ParseWebformForm reads the editor's fields.
func (f *WebformForm) ParseWebformForm(r *http.Request) {
	f.Title = strings.TrimSpace(r.FormValue("title"))
	f.Slug = strings.TrimSpace(r.FormValue("slug"))
	f.Description = strings.TrimSpace(r.FormValue("description"))
	f.Confirmation = strings.TrimSpace(r.FormValue("confirmation"))
	f.ConfirmationURL = strings.TrimSpace(r.FormValue("confirmation_url"))
	f.SubmitLabel = strings.TrimSpace(r.FormValue("submit_label"))
	f.Closed = r.FormValue("closed") == "1"
	f.NotifyEmail = strings.TrimSpace(r.FormValue("notify_email"))
	f.LimitPerUser = r.FormValue("limit_per_user") == "1"
	f.MaxSubmissions = strings.TrimSpace(r.FormValue("max_submissions"))
	f.ClosesAt = strings.TrimSpace(r.FormValue("closes_at"))
	f.Status, _ = strconv.Atoi(r.FormValue("status"))
	f.OnModeration, _ = strconv.Atoi(r.FormValue("on_moderation"))
	f.CommentMode = r.FormValue("comment_mode")
	f.Elements = nil
	for _, key := range r.Form["element_row"] {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		e := WebformElementInput{
			Key:         key,
			Type:        WebformElementType(r.FormValue("element_type_" + key)),
			Label:       strings.TrimSpace(r.FormValue("element_label_" + key)),
			Description: strings.TrimSpace(r.FormValue("element_description_" + key)),
			Required:    r.FormValue("element_required_"+key) == "1",
			Options:     strings.TrimSpace(r.FormValue("element_options_" + key)),
			Link:        strings.TrimSpace(r.FormValue("element_link_" + key)),
			Min:         strings.TrimSpace(r.FormValue("element_min_" + key)),
			Max:         strings.TrimSpace(r.FormValue("element_max_" + key)),
			Ext:         strings.TrimSpace(r.FormValue("element_ext_" + key)),
			IfKey:       strings.TrimSpace(r.FormValue("element_if_" + key)),
			IfValue:     strings.TrimSpace(r.FormValue("element_ifvalue_" + key)),
			Prefill:     strings.TrimSpace(r.FormValue("element_prefill_" + key)),
		}
		if id, err := strconv.Atoi(key); err == nil && id > 0 {
			e.ID = id
		}
		// A new row left empty is not an element.
		if e.ID == 0 && e.Label == "" && e.Options == "" && e.Description == "" && e.Link == "" {
			continue
		}
		f.Elements = append(f.Elements, e)
	}
	f.ParseNodeMeta(r)
}

// Validate - errors by field ("title", "elements", "element_<key>").
func (f *WebformForm) Validate(ctx context.Context) map[string]string {
	errs := map[string]string{}
	if f.Title == "" {
		errs["title"] = i18n.T(ctx, "Enter the form's title.")
	}
	for _, a := range (Webform{NotifyEmail: f.NotifyEmail}).NotifyAddresses() {
		if addr, err := mail.ParseAddress(a); err != nil || addr.Address != a {
			errs["notify_email"] = i18n.T(ctx, "Invalid address for notifications: @address", "@address", a)
			break
		}
	}
	if f.ConfirmationURL != "" && (!strings.HasPrefix(f.ConfirmationURL, "/") || strings.HasPrefix(f.ConfirmationURL, "//") || strings.ContainsAny(f.ConfirmationURL, " \\\t\r\n")) {
		errs["confirmation_url"] = i18n.T(ctx, "The page after sending is a path on this site, starting with /.")
	}
	if f.MaxSubmissions != "" {
		if n, err := strconv.Atoi(f.MaxSubmissions); err != nil || n < 0 {
			errs["max_submissions"] = i18n.T(ctx, "The submission limit is a whole number (empty - no limit).")
		}
	}
	if f.ClosesAt != "" {
		if _, err := time.Parse("2006-01-02T15:04", f.ClosesAt); err != nil {
			errs["closes_at"] = i18n.T(ctx, "Invalid closing date.")
		}
	}
	if len(f.Elements) == 0 {
		errs["elements"] = i18n.T(ctx, "Add at least one question.")
	}
	for _, e := range f.Elements {
		switch {
		case !e.Type.Valid():
			errs["element_"+e.Key] = i18n.T(ctx, "Choose the question's type.")
		case e.Label == "":
			errs["element_"+e.Key] = i18n.T(ctx, "Enter the question.")
		case e.Type.HasOptions() && len(ParseWebformOptions(e.Options)) == 0:
			errs["element_"+e.Key] = i18n.T(ctx, "Enter the options, one per line.")
		case e.Type == WebformConsent && e.Link != "" && !strings.HasPrefix(e.Link, "/") &&
			!strings.HasPrefix(e.Link, "https://") && !strings.HasPrefix(e.Link, "http://"):
			errs["element_"+e.Key] = i18n.T(ctx, "The policy link is an address on the site (/…) or a full address (https://…).")
		case e.Type == WebformNumber && !validNumberLimits(e.Min, e.Max):
			errs["element_"+e.Key] = i18n.T(ctx, "The limits are numbers, the lower not above the upper.")
		case e.Prefill != "" && e.Type.CanPrefill() && !webformPrefillParam.MatchString(e.Prefill):
			errs["element_"+e.Key] = i18n.T(ctx, "The address parameter is Latin letters, digits, \"_\" and \"-\".")
		}
	}
	return errs
}

// validNumberLimits - empty or numbers, min not above max.
func validNumberLimits(lowest, highest string) bool {
	parse := func(s string) (float64, bool) {
		if s == "" {
			return 0, true
		}
		f, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
		return f, err == nil
	}
	lo, okLo := parse(lowest)
	hi, okHi := parse(highest)
	return okLo && okHi && (lowest == "" || highest == "" || lo <= hi)
}

// WebformFormFromEntity - the editor's fields of a stored form (dates in
// loc, the site's zone).
func WebformFormFromEntity(w *Webform, loc *time.Location) *WebformForm {
	f := &WebformForm{
		NodeMetaFields:  modulekit.NodeMetaFromPageSEO(w.Node.PageSEO),
		NodeID:          w.Node.ID,
		Title:           w.Node.Title,
		Slug:            w.Node.Slug,
		Description:     w.Description,
		Confirmation:    w.Confirmation,
		ConfirmationURL: w.ConfirmationURL,
		SubmitLabel:     w.SubmitLabel,
		Closed:          w.Closed,
		NotifyEmail:     w.NotifyEmail,
		LimitPerUser:    w.LimitPerUser,
		Status:          w.Node.Status,
		OnModeration:    w.Node.OnModeration,
		CommentMode:     string(w.Node.CommentMode),
	}
	if w.MaxSubmissions > 0 {
		f.MaxSubmissions = strconv.Itoa(w.MaxSubmissions)
	}
	if w.ClosesAt != nil {
		f.ClosesAt = w.ClosesAt.In(loc).Format("2006-01-02T15:04")
	}
	for _, e := range w.Elements {
		in := WebformElementInput{
			Key: strconv.Itoa(e.ID), ID: e.ID, Type: e.Type, Label: e.Label,
			Description: e.Description, Required: e.Required, Options: e.OptionsText(),
			IfValue: e.VisibleIfValue, Prefill: e.PrefillParam,
		}
		if id := w.ElementIDByName(e.VisibleIfElement); e.VisibleIfElement != "" && id > 0 {
			in.IfKey = strconv.Itoa(id)
		}
		switch e.Type {
		case WebformConsent:
			in.Options, in.Link = "", e.ConsentLink()
		case WebformFile:
			in.Options, in.Ext = "", strings.Join(e.Options, ", ")
		case WebformNumber:
			in.Options = ""
			in.Min, in.Max = e.NumberRange()
		}
		f.Elements = append(f.Elements, in)
	}
	return f
}

// Limits - the form's limits of the editor's fields (after Validate): the
// submission limit and the closing time read in loc, the site's zone.
func (f *WebformForm) Limits(loc *time.Location) (maxSubmissions int, closesAt *time.Time) {
	maxSubmissions, _ = strconv.Atoi(f.MaxSubmissions)
	if t, err := time.ParseInLocation("2006-01-02T15:04", f.ClosesAt, loc); err == nil {
		closesAt = &t
	}
	return maxSubmissions, closesAt
}

// WebformElements - the form's elements of the editor's rows (machine
// names of existing ones are kept by the repository).
func (f *WebformForm) WebformElements() WebformElements {
	out := make(WebformElements, 0, len(f.Elements))
	position := map[string]int{}
	for i, e := range f.Elements {
		position[e.Key] = i
	}
	for i, e := range f.Elements {
		el := WebformElement{ID: e.ID, Type: e.Type, Label: e.Label, Description: e.Description, Required: e.Required, Weight: i}
		if e.Type.CanPrefill() {
			el.PrefillParam = e.Prefill
		}
		// A condition on an earlier row: its position plus one.
		if p, ok := position[e.IfKey]; ok && e.IfKey != "" && p < i {
			el.VisibleIfRef, el.VisibleIfValue = p+1, e.IfValue
		}
		switch {
		case e.Type.HasOptions():
			el.Options = ParseWebformOptions(e.Options)
		case e.Type == WebformConsent:
			el.Required = true
			if e.Link != "" {
				el.Options = []string{e.Link}
			}
		case e.Type == WebformNumber:
			if e.Min != "" {
				el.Options = append(el.Options, "min="+strings.Replace(e.Min, ",", ".", 1))
			}
			if e.Max != "" {
				el.Options = append(el.Options, "max="+strings.Replace(e.Max, ",", ".", 1))
			}
		case e.Type == WebformFile:
			// "pdf, .docx Jpg" → pdf, docx, jpg (empty - the defaults).
			for _, x := range strings.FieldsFunc(e.Ext, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' }) {
				if x = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(x), ".")); x != "" {
					el.Options = append(el.Options, x)
				}
			}
		}
		out = append(out, el)
	}
	return out
}
