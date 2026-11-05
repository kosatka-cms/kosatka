package entity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"
)

// RevisionKind - how a revision came about.
type RevisionKind string

const (
	// RevisionCreate - the node's first save.
	RevisionCreate RevisionKind = "create"
	// RevisionEdit - a later save: the form, publishing, moderation.
	RevisionEdit RevisionKind = "edit"
	// RevisionRevert - an earlier revision brought back
	// (NodeRevision.RevertedFromAt).
	RevisionRevert RevisionKind = "revert"
	// RevisionBaseline - the node as it was before its first save with the
	// history on: no author, the node's last update time.
	RevisionBaseline RevisionKind = "baseline"
)

// The keys of a NodeSnapshot's own parts, as NodeSnapshot.Diff names them;
// a field is SnapshotFieldKey(its machine name).
const (
	SnapshotTitle           = "title"
	SnapshotStatus          = "status"
	SnapshotOnModeration    = "on_moderation"
	SnapshotPromote         = "promote"
	SnapshotSticky          = "sticky"
	SnapshotCommentMode     = "comment_mode"
	SnapshotSlug            = "slug"
	SnapshotAuthor          = "author"
	SnapshotMetaTitle       = "meta_title"
	SnapshotMetaDescription = "meta_description"
	snapshotFieldPrefix     = "field:"
)

// SnapshotFieldKey - the Diff key of the field machineName.
func SnapshotFieldKey(machineName string) string { return snapshotFieldPrefix + machineName }

// SnapshotFieldName - the field machine name of a Diff key; false - one of
// the node's own parts.
func SnapshotFieldName(key string) (string, bool) {
	if len(key) > len(snapshotFieldPrefix) && key[:len(snapshotFieldPrefix)] == snapshotFieldPrefix {
		return key[len(snapshotFieldPrefix):], true
	}
	return "", false
}

// NodeSnapshot - a node of an admin-created type as a revision keeps it:
// its own fields, SEO and field values. Values are JSON, keyed by the
// field's machine name (a field added or removed later doesn't shift
// them), so two snapshots compare by their bytes.
type NodeSnapshot struct {
	Title           string                     `json:"title"`
	Status          int                        `json:"status"`
	OnModeration    int                        `json:"on_moderation"`
	Promote         bool                       `json:"promote"`
	Sticky          bool                       `json:"sticky"`
	CommentMode     string                     `json:"comment_mode"`
	Slug            string                     `json:"slug"`
	AuthorID        int                        `json:"author_id"`
	MetaTitle       string                     `json:"meta_title,omitempty"`
	MetaDescription string                     `json:"meta_description,omitempty"`
	Fields          map[string]json.RawMessage `json:"fields,omitempty"`
	TextFormats     map[string]int             `json:"text_formats,omitempty"`
}

// SnapshotOf - the snapshot of dc (its values by dc.Fields); a value of a
// type it can't encode is left out.
func SnapshotOf(dc DynamicContent) NodeSnapshot {
	n := dc.Node
	s := NodeSnapshot{
		Title:           n.Title,
		Status:          n.Status,
		OnModeration:    n.OnModeration,
		Promote:         n.Promote,
		Sticky:          n.Sticky,
		CommentMode:     string(n.CommentMode.OrDefault()),
		Slug:            n.Slug,
		AuthorID:        n.CreatedBy.ID,
		MetaTitle:       n.PageSEO.Title,
		MetaDescription: n.PageSEO.Description,
	}
	s.Fields, s.TextFormats = snapshotValues(dc.Fields, dc.Values, dc.TextFormats)
	return s
}

// snapshotValues - values (and their textFormats) of fields as a snapshot
// keeps them, by machine name; a value of a type it can't encode is left
// out. A blocks field's items - their own (BlockItem.MarshalJSON).
func snapshotValues(fields FieldInstanceDefs, values map[int]any, textFormats map[int]int) (map[string]json.RawMessage, map[string]int) {
	out, formats := map[string]json.RawMessage{}, map[string]int{}
	for _, f := range fields {
		v, ok := values[f.ID]
		if !ok || v == nil {
			continue
		}
		if t, ok := v.(time.Time); ok {
			// One form for an instant, whatever the location it was read in.
			v = t.UTC().Format(time.RFC3339Nano)
		}
		raw, err := json.Marshal(v)
		if err != nil || string(raw) == "null" || string(raw) == "[]" || string(raw) == `""` {
			continue
		}
		out[f.MachineName] = raw
		if id, ok := textFormats[f.ID]; ok {
			formats[f.MachineName] = id
		}
	}
	return out, formats
}

// Values - the snapshot's values of fields, typed as
// DynamicContentRepository reads them, and their text formats, keyed by
// field instance ID. A field the snapshot hasn't, or whose value doesn't
// decode as the field's type (its type changed since), is left out.
func (s NodeSnapshot) Values(fields FieldInstanceDefs) (map[int]any, map[int]int) {
	values, formats := map[int]any{}, map[int]int{}
	for _, f := range fields {
		raw, ok := s.Fields[f.MachineName]
		if !ok {
			continue
		}
		if v, ok := decodeSnapshotValue(f, raw); ok {
			values[f.ID] = v
		}
		if id, ok := s.TextFormats[f.MachineName]; ok {
			formats[f.ID] = id
		}
	}
	return values, formats
}

// decodeSnapshotValue - raw as f's value type (its type's
// fieldTypeDef.decode; false - none, or not that type's value).
func decodeSnapshotValue(f FieldInstanceDef, raw json.RawMessage) (any, bool) {
	decode := fieldTypeDefs[f.FieldType].decode
	if decode == nil {
		return nil, false
	}
	return decode(f, raw)
}

// Diff - the keys of what differs between s and next: their own parts
// (SnapshotTitle…) in that order, then the fields (SnapshotFieldKey) by
// machine name; a field's value (a text's white space aside) or its text
// format.
func (s NodeSnapshot) Diff(next NodeSnapshot) []string {
	var out []string
	add := func(key string, differ bool) {
		if differ {
			out = append(out, key)
		}
	}
	add(SnapshotTitle, s.Title != next.Title)
	add(SnapshotStatus, s.Status != next.Status)
	add(SnapshotOnModeration, s.OnModeration != next.OnModeration)
	add(SnapshotPromote, s.Promote != next.Promote)
	add(SnapshotSticky, s.Sticky != next.Sticky)
	add(SnapshotCommentMode, s.CommentMode != next.CommentMode)
	add(SnapshotSlug, s.Slug != next.Slug)
	add(SnapshotAuthor, s.AuthorID != next.AuthorID)
	add(SnapshotMetaTitle, s.MetaTitle != next.MetaTitle)
	add(SnapshotMetaDescription, s.MetaDescription != next.MetaDescription)
	names := slices.Sorted(maps.Keys(s.Fields))
	for name := range next.Fields {
		if _, ok := s.Fields[name]; !ok {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		add(SnapshotFieldKey(name), !sameValue(s.Fields[name], next.Fields[name]) || s.TextFormats[name] != next.TextFormats[name])
	}
	return out
}

// sameValue - a and b are one value: equal, or texts that differ only in
// white space (an editor's line breaks between tags).
func sameValue(a, b json.RawMessage) bool {
	if bytes.Equal(a, b) {
		return true
	}
	var as, bs string
	if json.Unmarshal(a, &as) != nil || json.Unmarshal(b, &bs) != nil {
		return false
	}
	return normalizeSpace(as) == normalizeSpace(bs)
}

// tagGapRe - white space between two tags.
var tagGapRe = regexp.MustCompile(`>\s+<`)

// normalizeSpace - s without the white space between tags, its other
// runs of white space one space.
func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(tagGapRe.ReplaceAllString(s, "><")), " ")
}

// Encode - the snapshot as stored (NodeRevision's snapshot column).
func (s NodeSnapshot) Encode() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("encoding the node snapshot: %w", err)
	}
	return string(b), nil
}

// DecodeNodeSnapshot - a stored snapshot.
func DecodeNodeSnapshot(s string) (NodeSnapshot, error) {
	var out NodeSnapshot
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return NodeSnapshot{}, fmt.Errorf("decoding the node snapshot: %w", err)
	}
	return out, nil
}

// NodeRevision - one saved state of a node (NodeSnapshot), who saved it
// (nil - RevisionBaseline or a deleted account) and when.
type NodeRevision struct {
	ID        int
	NodeID    int
	User      *User
	CreatedAt time.Time
	Kind      RevisionKind
	// RevertedFromAt - when the revision a RevisionRevert brought back was
	// saved.
	RevertedFromAt time.Time
	// Changed - the snapshot's keys that differ from the revision before
	// (NodeSnapshot.Diff); empty for the first one.
	Changed  []string
	Snapshot NodeSnapshot
}

// NodeRevisionRepository - the nodes' revisions, newest last by ID.
type NodeRevisionRepository interface {
	Create(ctx context.Context, rev NodeRevision) (int, error)
	// Latest - the node's newest revision; nil without one.
	Latest(ctx context.Context, nodeID int) (*NodeRevision, error)
	// ByID - the revision id of the node nodeID (ErrEntityNotFound for
	// another node's).
	ByID(ctx context.Context, nodeID, id int) (*NodeRevision, error)
	// List - a page of the node's revisions, newest first, and how many
	// it has.
	List(ctx context.Context, nodeID, offset, limit int) ([]NodeRevision, int, error)
	// Prune deletes the node's oldest revisions beyond keep.
	Prune(ctx context.Context, nodeID, keep int) error
	// Recent - a page of the revisions of every node the filter shows,
	// newest first, and how many there are.
	Recent(ctx context.Context, f RecentRevisionsFilter) ([]RecentRevision, int, error)
}

// RecentRevisionsFilter - the revisions the recent changes show: Title -
// a part of the node's title, Type - its type, Actor - a part of the
// saver's name, From/To - the days (To - the day after the last; zero -
// none). AnyTypes, OwnTypes - what the viewer may see: every node of
// AnyTypes, the published nodes of OwnTypes that OwnerID created.
type RecentRevisionsFilter struct {
	Title, Type, Actor string
	From, To           time.Time
	AnyTypes, OwnTypes []string
	OwnerID            int
	Offset, Limit      int
}

// RecentRevision - a revision as the recent changes list it: its node's
// title, type and address (as they are now) and the node's revision before it
// (PreviousID, 0 - none kept). Snapshot holds only Status and
// OnModeration, and only when they alone changed.
type RecentRevision struct {
	NodeRevision
	NodeTitle  string
	NodeType   string
	NodeURL    string
	PreviousID int
}
