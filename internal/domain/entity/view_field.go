package entity

import (
	"time"
)

// ViewFieldDef - one field shown by a ViewDisplayDef (independent of
// filters). Either FieldStorageID (a generic field) or BuiltinField is
// set. The list template renders only these fields, so even the title
// must be added here to appear.
type ViewFieldDef struct {
	ID            int
	ViewDisplayID int
	// FieldStorageID nil means this field reads BuiltinField.
	FieldStorageID *int
	// BuiltinField - see ViewFieldBuiltinField. Used only when
	// FieldStorageID is nil.
	BuiltinField ViewFieldBuiltinField
	// Label - the field's label in this View (field storages are global
	// and carry no label). Empty falls back at render time
	// (views.BuildContentFieldInstances) to the first attached content
	// type's instance label, then to the machine name; builtins fall back
	// to their fixed name. Never stored as a default.
	Label     string
	ShowLabel bool
	// ImageStyle - image fields only: the ImageStyleDef.Name to render
	// with, like FieldInstanceDef.ListImageStyle.
	ImageStyle string
	// DateFormat - date fields and date builtins only: the date format
	// machine name; empty = the field instance's DateFormat, else the
	// default format.
	DateFormat string
	// TagFormat - taxonomy fields only, see ViewTagFormat. Empty behaves
	// as ViewTagFormatPlain.
	TagFormat ViewTagFormat
	// ImageDisplay - image fields only, see ViewImageDisplay.
	ImageDisplay ViewImageDisplay
	// Formatter, Layout - link fields only: the FieldFormatter showing
	// the field ("" - links) and how its several values show, like
	// FieldInstanceDef.DetailFormatter/DetailLayout.
	Formatter string
	Layout    FieldLayout
	// LinkTo - see ViewFieldLinkTarget. List items have no automatic
	// link, so a field must opt in for the item to be clickable.
	LinkTo ViewFieldLinkTarget
	// CSSClass - extra class on the field's wrapper (card / table cell),
	// alongside the default item-field-{FieldType} class.
	CSSClass string
	// RewriteHTML - admin HTML wrapping the field's rendered value via a
	// literal "{{value}}" token, substituted by plain string replace -
	// never parsed as a Go template. Sanitized via
	// htmlsanitize.SanitizeAdmin on save.
	RewriteHTML string
	// TrimLength - a text field (a content or a comment View): when >0,
	// the value as plain text (tags stripped, blocks apart) cut to this
	// many characters at a word (service.PlainExcerpt; stripping avoids
	// broken, dangling markup). 0 = show as-is.
	TrimLength int
	// SortOrder - the field's column/card position.
	SortOrder int
	// ExcludeFromDisplay - the value is still resolved (usable as a
	// {{field:<key>}} token in another field's RewriteHTML, see
	// rewriteFieldValue) but the field renders no slot of its own, e.g.
	// a badge overlaid inside another field.
	ExcludeFromDisplay bool
	// HideInOverflow - with ViewDisplayDef.OverflowAfter > 0: rendered in
	// the full items but skipped in the compact overflow list (e.g. an
	// image). Unlike ExcludeFromDisplay, which hides it everywhere.
	HideInOverflow bool
	// FallbackFieldKey - when this field's value is empty for a row, the
	// value of another field in the same row is used instead
	// (ApplyFieldFallbacks). Same key namespace as RewriteHTML's
	// {{field:<key>}} tokens: a field storage machine name or a builtin
	// key.
	FallbackFieldKey string
	// Params - a module's field's settings in this View
	// (ViewModuleField.Params); nil for the others.
	Params    map[string]string `json:"Params,omitempty"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FieldStorageIDValue - FieldStorageID, 0 when the field is a builtin.
func (f ViewFieldDef) FieldStorageIDValue() int {
	if f.FieldStorageID == nil {
		return 0
	}
	return *f.FieldStorageID
}

// ViewFieldBuiltinField - a ViewFieldDef target that isn't a generic
// field (node title and other node properties; pictures are ordinary
// image fields). views.BuildContentFieldInstances synthesizes them from the
// node itself, so they get label, CSS class and position like any other
// field.
type ViewFieldBuiltinField string

// The built-in View fields: the node's, then the comment base entity's.
const (
	ViewFieldBuiltinTitle ViewFieldBuiltinField = "title"
	// ViewFieldBuiltinContentType - the node's content type label
	// (ContentTypeDef.Label), resolved once per render, not per node.
	ViewFieldBuiltinContentType ViewFieldBuiltinField = "content_type"
	// ViewFieldBuiltinCreatedAt - the node's CreatedAt, rendered as a
	// date field.
	ViewFieldBuiltinCreatedAt ViewFieldBuiltinField = "created_at"
	// ViewFieldBuiltinViewCount - the node's total view counter, rendered
	// as a number.
	ViewFieldBuiltinViewCount ViewFieldBuiltinField = "view_count"
	// ViewFieldBuiltinCommentCount - the node's published comment count
	// (batched via CommentsService.CountByNodeIDs), rendered as a number.
	ViewFieldBuiltinCommentCount ViewFieldBuiltinField = "comment_count"
	// ViewFieldBuiltinReadingTime - the minutes reading the node takes
	// (ReadingMinutesOf its type's fields), as text: "5 min read"; none
	// without text to read.
	ViewFieldBuiltinReadingTime ViewFieldBuiltinField = "reading_time"
	// ViewFieldBuiltinAuthor - the node's author's name, linking their
	// profile.
	ViewFieldBuiltinAuthor ViewFieldBuiltinField = "author"
	// ViewFieldBuiltinLink - a link to the node, its text the field's
	// label ("Read more" when empty).
	ViewFieldBuiltinLink ViewFieldBuiltinField = "link"

	// Comment base entity builtins.
	ViewFieldBuiltinCommentSubject   ViewFieldBuiltinField = "comment_subject"
	ViewFieldBuiltinCommentContent   ViewFieldBuiltinField = "comment_content"
	ViewFieldBuiltinCommentAuthor    ViewFieldBuiltinField = "comment_author"
	ViewFieldBuiltinCommentCreatedAt ViewFieldBuiltinField = "comment_created_at"
	// ViewFieldBuiltinCommentNodeTitle - the parent node's title.
	ViewFieldBuiltinCommentNodeTitle ViewFieldBuiltinField = "comment_node_title"
)

// ViewTagFormat - how a taxonomy field renders: "plain" (comma-joined
// names), "linked" (comma-joined links) or "badges" (linked pills). See
// the dynamic_list_field_value block in dynamic/list.html.
type ViewTagFormat string

// The taxonomy fields' formats (ViewTagFormat).
const (
	ViewTagFormatPlain  ViewTagFormat = "plain"
	ViewTagFormatLinked ViewTagFormat = "linked"
	ViewTagFormatBadges ViewTagFormat = "badges"
)

// ViewImageDisplay - which images of an image field a View shows: the
// first (ViewImageFirst, "") or all of a field with several (ViewImageAll).
type ViewImageDisplay string

// The image fields' displays (ViewImageDisplay).
const (
	ViewImageFirst ViewImageDisplay = ""
	ViewImageAll   ViewImageDisplay = "all"
)

// ViewFieldLinkTarget - what a field's rendered value links to: nothing,
// the item ("content", any field type) or the original unstyled image
// ("file", image fields only - enforced in resolveViewFieldTarget).
type ViewFieldLinkTarget string

// The fields' link targets (ViewFieldLinkTarget).
const (
	ViewFieldLinkNone    ViewFieldLinkTarget = ""
	ViewFieldLinkContent ViewFieldLinkTarget = "content"
	ViewFieldLinkFile    ViewFieldLinkTarget = "file"
)

// ViewFieldDefs - a list of a display's fields.
type ViewFieldDefs []ViewFieldDef

// SortBySortOrder - see ViewFilterDefs.SortBySortOrder.
func (fs ViewFieldDefs) SortBySortOrder() ViewFieldDefs {
	sorted := make(ViewFieldDefs, len(fs))
	copy(sorted, fs)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			a, b := sorted[j-1], sorted[j]
			if a.SortOrder < b.SortOrder || (a.SortOrder == b.SortOrder && a.ID <= b.ID) {
				break
			}
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	return sorted
}

// View group heading formats for a date field (ViewDisplayDef.
// GroupDateFormat); "" - the rendered field.
const (
	ViewGroupDateYear  = "year"
	ViewGroupDateMonth = "month"
	ViewGroupDateDay   = "day"
)

// ValidViewGroupDateFormat - f is "" or one of ViewGroupDate*.
func ValidViewGroupDateFormat(f string) bool {
	switch f {
	case "", ViewGroupDateYear, ViewGroupDateMonth, ViewGroupDateDay:
		return true
	}
	return false
}

// IsDate - the builtin is a date (a date format applies to it).
func (b ViewFieldBuiltinField) IsDate() bool {
	return b == ViewFieldBuiltinCreatedAt || b == ViewFieldBuiltinCommentCreatedAt
}
