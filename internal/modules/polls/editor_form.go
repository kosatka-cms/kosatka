package polls

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// PollForm - the poll's create and edit form: the question, the options,
// the voting settings and the node's settings.
type PollForm struct {
	modulekit.NodeMetaFields
	NodeID        int
	Title         string
	Slug          string
	Description   string
	MaxAnswers    int
	AvailableFrom string // datetime-local ("2006-01-02T15:04") or empty
	AvailableTo   string
	ResultsOnly   bool
	AllowRevote   bool // allow changing the vote
	Status        int
	OnModeration  int
	OptionLabels  []string
	OptionIDs     []int // parallel to labels for edit; 0 = new
	// CommentMode - see modulekit.CommentMode's doc comment. Empty means
	// "keep the existing node's value" (or "open" on create).
	CommentMode string
}

// ParsePollForm fills the form from the request's fields (the description
// sanitized, options with empty labels dropped); it always returns nil.
func (f *PollForm) ParsePollForm(r *http.Request) error {
	f.Title = strings.TrimSpace(r.FormValue("title"))
	f.Slug = strings.TrimSpace(r.FormValue("slug"))
	// Output unescaped (processContent in poll/detail.html).
	f.Description = strings.TrimSpace(htmlsanitize.SanitizeAdmin(r.FormValue("description")))

	f.MaxAnswers = 1
	if m := r.FormValue("max_answers"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			f.MaxAnswers = v
		}
	}

	f.AvailableFrom = strings.TrimSpace(r.FormValue("available_from"))
	f.AvailableTo = strings.TrimSpace(r.FormValue("available_to"))
	f.ResultsOnly = r.FormValue("results_only") == "1" || r.FormValue("results_only") == "on"
	f.AllowRevote = r.FormValue("allow_revote") == "1" || r.FormValue("allow_revote") == "on"

	// Checkboxes from edit_node_settings (unchecked means absent from the form).
	f.Status = 0
	f.OnModeration = 0
	if statusStr := r.FormValue("status"); statusStr != "" {
		if v, err := strconv.Atoi(statusStr); err == nil {
			f.Status = v
		}
	}
	if modStr := r.FormValue("on_moderation"); modStr != "" {
		if v, err := strconv.Atoi(modStr); err == nil {
			f.OnModeration = v
		}
	}

	labels := r.Form["option_label"]
	ids := r.Form["option_id"]
	f.OptionLabels = make([]string, 0, len(labels))
	f.OptionIDs = make([]int, 0, len(labels))
	for i := range labels {
		lab := strings.TrimSpace(labels[i])
		if lab == "" {
			continue
		}
		id := 0
		if i < len(ids) && ids[i] != "" {
			if v, err := strconv.Atoi(ids[i]); err == nil {
				id = v
			}
		}
		f.OptionLabels = append(f.OptionLabels, lab)
		f.OptionIDs = append(f.OptionIDs, id)
	}
	f.CommentMode = r.FormValue("comment_mode")

	f.ParseNodeMeta(r)
	return nil
}

// Validate - the form's errors by field: no question, fewer than two
// options (empty - valid).
func (f *PollForm) Validate(ctx context.Context) map[string]string {
	errs := make(map[string]string)
	if f.Title == "" {
		errs["title"] = i18n.T(ctx, "Enter the question (title)")
	}
	if len(f.OptionLabels) < 2 {
		errs["options"] = i18n.T(ctx, "At least two answer options are required")
	}
	return errs
}

// ParsePollDatetimeLocal parses a datetime-local value (2006-01-02T15:04)
// in loc; an empty one is nil.
func ParsePollDatetimeLocal(s string, loc *time.Location) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	// datetime-local input in loc, the request's time zone
	// (see pkg/sitetime.FromContext).
	t, err := time.ParseInLocation("2006-01-02T15:04", s, loc)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
