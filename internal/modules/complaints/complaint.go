// Package complaints - the complaints module: signed-in visitors report
// a content item or a comment (a reason, a note), moderators decide in
// the queue (Content → Complaints); enough complaints of different
// visitors hide the item until they do.
package complaints

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// What a complaint is about (Target.Kind).
const (
	KindNode    = "node"
	KindComment = "comment"
)

// A target's state in the queue (Target.Status).
const (
	StatusNew      = "new"
	StatusResolved = "resolved"
)

// What a moderator did (Target.Resolution, Decide's action).
const (
	ResolutionKept      = "kept"
	ResolutionSpam      = "spam"
	ResolutionDeleted   = "deleted"
	ResolutionUnpublish = "unpublished"
)

// DefaultReasons - the reasons offered while the settings name none.
var DefaultReasons = []string{i18n.N("Spam or advertising"), i18n.N("Insults"), i18n.N("Breaks the site's rules")}

// OtherReason - the reason always offered last, with a note.
var OtherReason = i18n.N("Other")

// Settings - the module's settings: whether comments may be reported,
// the content types whose items may, the reasons (one per line; none -
// the defaults), how many complaints of different visitors hide an item
// (0 - none hides).
type Settings struct {
	Comments  bool
	NodeTypes []string
	Reasons   string
	HideAfter int
}

// reasonMax - the longest reason (the complaints' column).
const reasonMax = 255

// ReasonList - the reasons offered (each cut to reasonMax characters),
// OtherReason last (translation sources when the defaults).
func (s Settings) ReasonList() []string {
	var out []string
	for _, line := range strings.Split(s.Reasons, "\n") {
		if line = strings.TrimSpace(line); utf8.RuneCountInString(line) > reasonMax {
			line = strings.TrimSpace(string([]rune(line)[:reasonMax]))
		}
		if line != "" && line != OtherReason && !slices.Contains(out, line) {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		out = slices.Clone(DefaultReasons)
	}
	return append(out, OtherReason)
}

// NodeTypeAllowed - whether typ's items may be reported.
func (s Settings) NodeTypeAllowed(typ string) bool { return slices.Contains(s.NodeTypes, typ) }

// Target - a content item or a comment complained about, in the queue:
// what it is (Kind, ObjectID, its content item NodeID, its author), its
// state, the complaints' count, reasons and notes.
type Target struct {
	ID         int
	Kind       string
	ObjectID   int
	NodeID     int
	AuthorID   int
	Status     string
	Hidden     bool
	Resolution string
	ResolvedBy int
	ResolvedAt *time.Time
	LastAt     time.Time
	CreatedAt  time.Time
	// Count - the complaints; Reasons - how many of each reason; Notes -
	// the newest notes visitors wrote.
	Count   int
	Reasons []ReasonCount
	Notes   []Note
}

// ReasonCount - how many complaints gave a reason.
type ReasonCount struct {
	Reason string
	Count  int
}

// Note - a visitor's note with their complaint.
type Note struct {
	UserID int
	Text   string
	At     time.Time
}
