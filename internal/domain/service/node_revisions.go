package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/textdiff"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Permissions of the nodes' history, on top of editing the node.
const (
	PermViewRevisions   = "view content revisions"
	PermRevertRevisions = "revert content revisions"
)

// revisionTypes - a node type's definition and fields.
type revisionTypes interface {
	ContentTypes(ctx context.Context) (entity.ContentTypeDefs, error)
	ContentTypeByMachineName(ctx context.Context, machineName string) (*entity.ContentTypeDef, error)
	FieldsByContentTypeID(ctx context.Context, contentTypeID int) (entity.FieldInstanceDefs, error)
}

// revisionNodes - the nodes: the one saved, the ones referenced, its SEO.
type revisionNodes interface {
	ByID(ctx context.Context, nodeID int) (*entity.Node, error)
	ByIDs(ctx context.Context, ids []int) (map[int]entity.Node, error)
	UpsertPageSEO(ctx context.Context, nodeID int, metaTitle, metaDescription string) error
}

// revisionTerms - the terms referenced and a node's terms index.
type revisionTerms interface {
	TermsByIDs(ctx context.Context, ids []int) (map[int]entity.TaxonomyTerm, error)
	UpdateNodeTerms(ctx context.Context, nodeID int, termIDs []int) error
	FieldTermIDsForNode(ctx context.Context, nodeID int) ([]int, error)
}

type revisionFiles interface {
	ByIDs(ctx context.Context, ids []int) (map[int]entity.File, error)
}

type revisionUsers interface {
	ByIDs(ctx context.Context, ids []int) (map[int]entity.User, error)
}

// NodeRevisionDeps - what NodeRevisionService works with; Dynamic - the
// repository of a type's nodes (DynamicContentRepository).
type NodeRevisionDeps struct {
	Revisions entity.NodeRevisionRepository
	Nodes     revisionNodes
	Types     revisionTypes
	Dynamic   func(entity.ContentTypeDef, entity.FieldInstanceDefs) entity.DynamicContentRepository
	Terms     revisionTerms
	Files     revisionFiles
	Users     revisionUsers
	UoW       entity.UnitOfWork
	Logger    *slog.Logger
}

// nodePublishedHooks - what runs when a node becomes public
// (RegisterNodePublished): the modules', by module.
var (
	nodePublishedMu    sync.Mutex
	nodePublishedHooks []nodePublishedHook
)

type nodePublishedHook struct {
	module string
	fn     func(ctx context.Context, node entity.Node) error
}

// RegisterNodePublished registers fn, called when a node becomes public
// (published and not on moderation) - created so, or changed by its form,
// its publishing, the bulk actions, the schedule - while module is
// enabled ("" - always). Its error is logged; the change stands.
func RegisterNodePublished(module string, fn func(ctx context.Context, node entity.Node) error) {
	nodePublishedMu.Lock()
	defer nodePublishedMu.Unlock()
	nodePublishedHooks = append(nodePublishedHooks, nodePublishedHook{module: module, fn: fn})
}

// ResetNodePublished drops the hooks RegisterNodePublished added (the
// modules' hooks are registered again with each wiring).
func ResetNodePublished() {
	nodePublishedMu.Lock()
	defer nodePublishedMu.Unlock()
	nodePublishedHooks = nil
}

// NodeRevisionService - the changes of nodes: every change goes through
// it (Track, Created). The history of the nodes of admin-created types
// that keep it (ContentTypeDef.Revisions): every save becomes a revision
// (NodeSnapshot), the state before the first one too, so it can be
// brought back; two revisions compare field by field. And a node that
// becomes public is announced (RegisterNodePublished).
type NodeRevisionService struct {
	d   NodeRevisionDeps
	now func() time.Time
}

// NewNodeRevisionService - the node revision service on d (the revisions,
// the nodes, their types and values, the transactions).
func NewNodeRevisionService(d NodeRevisionDeps) *NodeRevisionService {
	return &NodeRevisionService{d: d, now: time.Now}
}

// nodeState - a node with its type and values.
type nodeState struct {
	def    entity.ContentTypeDef
	fields entity.FieldInstanceDefs
	dc     *entity.DynamicContent
}

// typeOf - the node's admin-created type and its fields; false for a
// module's type (no content_types row).
func (s *NodeRevisionService) typeOf(ctx context.Context, nodeType string) (*entity.ContentTypeDef, entity.FieldInstanceDefs, bool, error) {
	def, err := s.d.Types.ContentTypeByMachineName(ctx, nodeType)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil, false, nil
		}
		return nil, nil, false, fmt.Errorf("loading content type %q: %w", nodeType, err)
	}
	fields, err := s.d.Types.FieldsByContentTypeID(ctx, def.ID)
	if err != nil {
		return nil, nil, false, fmt.Errorf("loading the fields of %q: %w", nodeType, err)
	}
	return def, fields, true, nil
}

// state - the node nodeID as saved, when its type keeps a history.
func (s *NodeRevisionService) state(ctx context.Context, nodeID int) (*nodeState, error) {
	node, err := s.d.Nodes.ByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("loading node %d: %w", nodeID, err)
	}
	def, fields, ok, err := s.typeOf(ctx, node.Type.ToString())
	if err != nil || !ok || !def.Revisions {
		return nil, err
	}
	dc, err := s.d.Dynamic(*def, fields).ByNodeID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("loading node %d with its values: %w", nodeID, err)
	}
	return &nodeState{def: *def, fields: fields, dc: dc}, nil
}

// Type - nodeType's definition; false for a module's type, which keeps
// no history.
func (s *NodeRevisionService) Type(ctx context.Context, nodeType string) (*entity.ContentTypeDef, bool, error) {
	def, _, ok, err := s.typeOf(ctx, nodeType)
	return def, ok, err
}

// Enabled - nodeType keeps a history.
func (s *NodeRevisionService) Enabled(ctx context.Context, nodeType string) (bool, error) {
	def, ok, err := s.Type(ctx, nodeType)
	return ok && def.Revisions, err
}

// Created records the first revision of the node nodeID user just
// created (in the caller's transaction).
func (s *NodeRevisionService) Created(ctx context.Context, nodeID int, user *entity.User) error {
	if err := s.record(ctx, nodeID, user, entity.RevisionCreate, time.Time{}); err != nil {
		return err
	}
	if node, err := s.d.Nodes.ByID(ctx, nodeID); err == nil {
		AuditNodeCreated(ctx, *node)
	}
	return s.announce(ctx, nodeID, false)
}

// announce runs the RegisterNodePublished hooks when the node nodeID is
// public now and wasn't (wasPublic); a hook's error is logged.
func (s *NodeRevisionService) announce(ctx context.Context, nodeID int, wasPublic bool) error {
	if wasPublic {
		return nil
	}
	nodePublishedMu.Lock()
	hooks := append([]nodePublishedHook(nil), nodePublishedHooks...)
	nodePublishedMu.Unlock()
	if len(hooks) == 0 {
		return nil
	}
	node, err := s.d.Nodes.ByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("loading node %d: %w", nodeID, err)
	}
	if !node.IsPublic() {
		return nil
	}
	for _, h := range hooks {
		if h.module != "" && !ModuleEnabled(h.module) {
			continue
		}
		LogFallback(s.d.Logger, h.fn(ctx, *node), "error telling of a published node", "nid", nodeID, "module", h.module)
	}
	return nil
}

// Track runs save, a change of the node nodeID by user, and records the
// result as a revision - in one transaction (the caller's when ctx has
// one). A node with no revision yet gets one of its state before; a node
// it makes public is announced; what the save changed of its status,
// author and address goes to the action log (audit).
func (s *NodeRevisionService) Track(ctx context.Context, nodeID int, user *entity.User, save func(ctx context.Context) error) error {
	var before, after *entity.Node
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if before, err = s.d.Nodes.ByID(ctx, nodeID); err != nil {
			return fmt.Errorf("loading node %d: %w", nodeID, err)
		}
		if err := s.baseline(ctx, nodeID); err != nil {
			return err
		}
		if err := save(ctx); err != nil {
			return err
		}
		if err := s.record(ctx, nodeID, user, entity.RevisionEdit, time.Time{}); err != nil {
			return err
		}
		if after, err = s.d.Nodes.ByID(ctx, nodeID); err != nil {
			return fmt.Errorf("loading node %d: %w", nodeID, err)
		}
		return s.announce(ctx, nodeID, before.IsPublic())
	})
	if err == nil {
		AuditNodeChange(ctx, *before, *after)
	}
	return err
}

// baseline records the node's current state when it has no revision yet:
// what it was before its first save with the history on.
func (s *NodeRevisionService) baseline(ctx context.Context, nodeID int) error {
	st, err := s.state(ctx, nodeID)
	if err != nil || st == nil {
		return err
	}
	latest, err := s.d.Revisions.Latest(ctx, nodeID)
	if err != nil || latest != nil {
		return err
	}
	at := st.dc.Node.UpdatedAt
	if at.IsZero() {
		at = s.now()
	}
	_, err = s.d.Revisions.Create(ctx, entity.NodeRevision{
		NodeID:    nodeID,
		CreatedAt: at,
		Kind:      entity.RevisionBaseline,
		Snapshot:  entity.SnapshotOf(*st.dc),
	})
	return err
}

// record saves the node's current state as a revision of kind by user,
// unless nothing changed since the latest one, and prunes the oldest
// beyond the type's keep.
func (s *NodeRevisionService) record(ctx context.Context, nodeID int, user *entity.User, kind entity.RevisionKind, revertedFromAt time.Time) error {
	st, err := s.state(ctx, nodeID)
	if err != nil || st == nil {
		return err
	}
	snapshot := entity.SnapshotOf(*st.dc)
	latest, err := s.d.Revisions.Latest(ctx, nodeID)
	if err != nil {
		return err
	}
	var changed []string
	if latest != nil {
		if changed = latest.Snapshot.Diff(snapshot); len(changed) == 0 {
			return nil
		}
	}
	if _, err := s.d.Revisions.Create(ctx, entity.NodeRevision{
		NodeID:         nodeID,
		User:           user,
		CreatedAt:      s.now(),
		Kind:           kind,
		RevertedFromAt: revertedFromAt,
		Changed:        changed,
		Snapshot:       snapshot,
	}); err != nil {
		return err
	}
	return s.d.Revisions.Prune(ctx, nodeID, st.def.RevisionsKeep)
}

// RevisionSummary - what a revision changed, as the history lists it.
type RevisionSummary struct {
	Kind entity.RevisionKind
	// Status - "published", "unpublished", "moderation_on",
	// "moderation_off" when only the publishing changed; else empty.
	Status string
	// Labels - the changed parts and fields.
	Labels []string
	// RevertedFromAt - when the revision a revert brought back was saved.
	RevertedFromAt time.Time
}

// RevisionRow - a revision as the history lists it.
type RevisionRow struct {
	entity.NodeRevision
	Summary RevisionSummary
	// Current - the newest revision: the node as it is.
	Current bool
}

// List - a page of the node's revisions, newest first, and how many it
// has.
func (s *NodeRevisionService) List(ctx context.Context, node entity.Node, offset, limit int) ([]RevisionRow, int, error) {
	revs, total, err := s.d.Revisions.List(ctx, node.ID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	_, fields, _, err := s.typeOf(ctx, node.Type.ToString())
	if err != nil {
		return nil, 0, err
	}
	rows := make([]RevisionRow, len(revs))
	for i, r := range revs {
		rows[i] = RevisionRow{NodeRevision: r, Summary: s.summary(ctx, r, fields), Current: offset == 0 && i == 0}
	}
	return rows, total, nil
}

// summary - what revision r changed (fields - its type's).
func (s *NodeRevisionService) summary(ctx context.Context, r entity.NodeRevision, fields entity.FieldInstanceDefs) RevisionSummary {
	sum := RevisionSummary{Kind: r.Kind, Labels: s.labels(ctx, r.Changed, fields), RevertedFromAt: r.RevertedFromAt}
	if len(r.Changed) == 1 {
		switch r.Changed[0] {
		case entity.SnapshotStatus:
			sum.Status = map[bool]string{true: "published", false: "unpublished"}[r.Snapshot.Status == 1]
		case entity.SnapshotOnModeration:
			sum.Status = map[bool]string{true: "moderation_on", false: "moderation_off"}[r.Snapshot.OnModeration == 1]
		}
	}
	return sum
}

// RecentRow - a revision as the recent changes list it.
type RecentRow struct {
	entity.RecentRevision
	Summary RevisionSummary
	// TypeLabel - the node's type as named.
	TypeLabel string
}

// Recent - a page of the revisions of every node the filter shows (and
// its viewer may see), newest first, and how many there are.
func (s *NodeRevisionService) Recent(ctx context.Context, f entity.RecentRevisionsFilter) ([]RecentRow, int, error) {
	revs, total, err := s.d.Revisions.Recent(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	type typeInfo struct {
		label  string
		fields entity.FieldInstanceDefs
	}
	types := map[string]typeInfo{}
	rows := make([]RecentRow, len(revs))
	for i, r := range revs {
		t, ok := types[r.NodeType]
		if !ok {
			t.label = r.NodeType
			def, fields, found, err := s.typeOf(ctx, r.NodeType)
			if err != nil {
				return nil, 0, err
			}
			if found {
				t = typeInfo{label: def.Label, fields: fields}
			}
			types[r.NodeType] = t
		}
		rows[i] = RecentRow{RecentRevision: r, Summary: s.summary(ctx, r.NodeRevision, t.fields), TypeLabel: t.label}
	}
	return rows, total, nil
}

// RecentScope - which nodes' revisions a viewer may see (has - their
// permissions), as a filter's AnyTypes/OwnTypes/OwnerID: every node of
// the types they may edit any of, their own published ones of those they
// may edit only their own of (as AuthService.CanEditNode).
func (s *NodeRevisionService) RecentScope(ctx context.Context, userID int, has func(permission string) bool) (entity.RecentRevisionsFilter, error) {
	f := entity.RecentRevisionsFilter{OwnerID: userID}
	defs, err := s.d.Types.ContentTypes(ctx)
	if err != nil {
		return f, fmt.Errorf("listing the content types: %w", err)
	}
	for _, d := range defs {
		switch {
		case has(PermEditAny(d.MachineName)):
			f.AnyTypes = append(f.AnyTypes, d.MachineName)
		case has(PermEditOwn(d.MachineName)):
			f.OwnTypes = append(f.OwnTypes, d.MachineName)
		}
	}
	return f, nil
}

// HistoryTypes - the admin-created content types that keep their
// history (on) and those that don't (off), as listed.
func (s *NodeRevisionService) HistoryTypes(ctx context.Context) (on, off entity.ContentTypeDefs, err error) {
	defs, err := s.d.Types.ContentTypes(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("listing the content types: %w", err)
	}
	for _, d := range defs {
		if d.Revisions {
			on = append(on, d)
		} else {
			off = append(off, d)
		}
	}
	return on, off, nil
}

// Revision - the revision id of node nodeID (entity.ErrEntityNotFound
// for none, or another node's).
func (s *NodeRevisionService) Revision(ctx context.Context, nodeID, id int) (*entity.NodeRevision, error) {
	return s.d.Revisions.ByID(ctx, nodeID, id)
}

// labels - the keys' labels: the node's parts, then the fields (a field
// removed since - its machine name).
func (s *NodeRevisionService) labels(ctx context.Context, keys []string, fields entity.FieldInstanceDefs) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, s.label(ctx, k, fields))
	}
	return out
}

func (s *NodeRevisionService) label(ctx context.Context, key string, fields entity.FieldInstanceDefs) string {
	if name, ok := entity.SnapshotFieldName(key); ok {
		if f, ok := fieldByName(fields, name); ok {
			return f.Label
		}
		return name
	}
	switch key {
	case entity.SnapshotTitle:
		return i18n.T(ctx, "Title")
	case entity.SnapshotStatus:
		return i18n.T(ctx, "Status")
	case entity.SnapshotOnModeration:
		return i18n.T(ctx, "Moderation")
	case entity.SnapshotPromote:
		return i18n.T(ctx, "On the front page")
	case entity.SnapshotSticky:
		return i18n.T(ctx, "Sticky")
	case entity.SnapshotCommentMode:
		return i18n.T(ctx, "Comments")
	case entity.SnapshotSlug:
		return i18n.T(ctx, "Address")
	case entity.SnapshotAuthor:
		return i18n.T(ctx, "Author")
	case entity.SnapshotMetaTitle:
		return i18n.T(ctx, "Page title")
	case entity.SnapshotMetaDescription:
		return i18n.T(ctx, "Description (meta description)")
	}
	return key
}

func fieldByName(fields entity.FieldInstanceDefs, name string) (entity.FieldInstanceDef, bool) {
	for _, f := range fields {
		if f.MachineName == name {
			return f, true
		}
	}
	return entity.FieldInstanceDef{}, false
}

// RevisionItem - one value of a compared part (a term, an image, "Yes"…):
// Text, or Time for a date; Changed - absent from the other side.
type RevisionItem struct {
	Text    string
	Time    time.Time
	Changed bool
}

// RevisionChange - a changed part or field, side by side: as text
// compared by words (IsText), or as items.
type RevisionChange struct {
	Label              string
	IsText             bool
	OldText, NewText   []textdiff.Paragraph
	OldItems, NewItems []RevisionItem
}

// RevisionComparison - two revisions of a node compared.
type RevisionComparison struct {
	From, To  entity.NodeRevision
	Changes   []RevisionChange
	Unchanged []string
}

// Compare - revisions fromID and toID of node compared: the changed parts
// and fields, and the labels of the title and the fields that didn't.
func (s *NodeRevisionService) Compare(ctx context.Context, node entity.Node, fromID, toID int) (*RevisionComparison, error) {
	from, err := s.Revision(ctx, node.ID, fromID)
	if err != nil {
		return nil, err
	}
	to, err := s.Revision(ctx, node.ID, toID)
	if err != nil {
		return nil, err
	}
	_, fields, _, err := s.typeOf(ctx, node.Type.ToString())
	if err != nil {
		return nil, err
	}
	out := &RevisionComparison{From: *from, To: *to}
	changed := from.Snapshot.Diff(to.Snapshot)
	for _, key := range changed {
		c, err := s.change(ctx, key, fields, from.Snapshot, to.Snapshot)
		if err != nil {
			return nil, err
		}
		out.Changes = append(out.Changes, c)
	}
	if !slices.Contains(changed, entity.SnapshotTitle) {
		out.Unchanged = append(out.Unchanged, s.label(ctx, entity.SnapshotTitle, fields))
	}
	for _, f := range fields {
		if f.FieldType != entity.FieldTypeGroup && !slices.Contains(changed, entity.SnapshotFieldKey(f.MachineName)) {
			out.Unchanged = append(out.Unchanged, f.Label)
		}
	}
	return out, nil
}

// change - key's values in before and after side by side.
func (s *NodeRevisionService) change(ctx context.Context, key string, fields entity.FieldInstanceDefs, before, after entity.NodeSnapshot) (RevisionChange, error) {
	c := RevisionChange{Label: s.label(ctx, key, fields)}
	if lines, ok := textLines(key, fields, before); ok {
		newLines, _ := textLines(key, fields, after)
		c.IsText = true
		c.OldText, c.NewText = textdiff.Compare(lines, newLines)
		return c, nil
	}
	var err error
	if c.OldItems, err = s.items(ctx, key, fields, before); err != nil {
		return c, err
	}
	if c.NewItems, err = s.items(ctx, key, fields, after); err != nil {
		return c, err
	}
	markItems(c.OldItems, c.NewItems)
	markItems(c.NewItems, c.OldItems)
	return c, nil
}

// textLines - key's value in s as paragraphs, when it is text (the title,
// the address, the SEO, a text field - its HTML as text).
func textLines(key string, fields entity.FieldInstanceDefs, s entity.NodeSnapshot) ([]string, bool) {
	single := func(v string) []string {
		if v == "" {
			return nil
		}
		return []string{v}
	}
	switch key {
	case entity.SnapshotTitle:
		return single(s.Title), true
	case entity.SnapshotSlug:
		return single(s.Slug), true
	case entity.SnapshotMetaTitle:
		return single(s.MetaTitle), true
	case entity.SnapshotMetaDescription:
		return single(s.MetaDescription), true
	}
	name, ok := entity.SnapshotFieldName(key)
	if !ok {
		return nil, false
	}
	f, ok := fieldByName(fields, name)
	if !ok || f.FieldType != entity.FieldTypeText {
		return nil, false
	}
	values, _ := s.Values(entity.FieldInstanceDefs{f})
	text, _ := values[f.ID].(string)
	if strings.Contains(text, "<") {
		return htmlsanitize.StripToLines(text), true
	}
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, true
}

// markItems marks the items of side absent from other.
func markItems(side, other []RevisionItem) {
	for i := range side {
		side[i].Changed = !slices.ContainsFunc(other, func(o RevisionItem) bool {
			return o.Text == side[i].Text && o.Time.Equal(side[i].Time)
		})
	}
}

// items - key's value in s as items: the node's flags, its author, a
// field's values by name (terms, content, files, list labels, links).
func (s *NodeRevisionService) items(ctx context.Context, key string, fields entity.FieldInstanceDefs, snap entity.NodeSnapshot) ([]RevisionItem, error) {
	yesNo := func(v bool) []RevisionItem {
		return []RevisionItem{{Text: map[bool]string{true: i18n.T(ctx, "Yes"), false: i18n.T(ctx, "No")}[v]}}
	}
	switch key {
	case entity.SnapshotStatus:
		return []RevisionItem{{Text: map[bool]string{true: i18n.T(ctx, "Published"), false: i18n.T(ctx, "Not published")}[snap.Status == 1]}}, nil
	case entity.SnapshotOnModeration:
		return []RevisionItem{{Text: map[bool]string{true: i18n.T(ctx, "In moderation"), false: i18n.T(ctx, "Not in moderation")}[snap.OnModeration == 1]}}, nil
	case entity.SnapshotPromote:
		return yesNo(snap.Promote), nil
	case entity.SnapshotSticky:
		return yesNo(snap.Sticky), nil
	case entity.SnapshotCommentMode:
		label := map[string]string{
			string(entity.CommentModeOpen):   i18n.TC(ctx, "comment mode", "Open"),
			string(entity.CommentModeClosed): i18n.TC(ctx, "comment mode", "Closed"),
			string(entity.CommentModeHidden): i18n.TC(ctx, "comment mode", "Hidden"),
		}[snap.CommentMode]
		return []RevisionItem{{Text: cmp(label, snap.CommentMode)}}, nil
	case entity.SnapshotAuthor:
		users, err := s.d.Users.ByIDs(ctx, []int{snap.AuthorID})
		if err != nil {
			return nil, fmt.Errorf("loading user %d: %w", snap.AuthorID, err)
		}
		return []RevisionItem{{Text: cmp(users[snap.AuthorID].Username, "#"+strconv.Itoa(snap.AuthorID))}}, nil
	}
	name, ok := entity.SnapshotFieldName(key)
	if !ok {
		return nil, nil
	}
	f, ok := fieldByName(fields, name)
	if !ok {
		// A field removed since: its raw value.
		if raw, ok := snap.Fields[name]; ok {
			return []RevisionItem{{Text: string(raw)}}, nil
		}
		return nil, nil
	}
	values, _ := snap.Values(entity.FieldInstanceDefs{f})
	v, ok := values[f.ID]
	if !ok {
		return nil, nil
	}
	if items := revisionFieldTypes[f.FieldType].items; items != nil {
		return items(ctx, s, f, v)
	}
	return []RevisionItem{{Text: fmt.Sprint(v)}}, nil
}

// cmp - v, or else when v is empty.
func cmp(v, orElse string) string {
	if v != "" {
		return v
	}
	return orElse
}

func listValues(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	}
	return nil
}

func idValues(v any) []int {
	switch v := v.(type) {
	case int:
		return []int{v}
	case []int:
		return v
	}
	return nil
}

// names - the names of ids of a field of type t that still exist: terms,
// content titles, file names, user names (revisionFieldType.names; none
// for a type naming nothing by ID).
func (s *NodeRevisionService) names(ctx context.Context, t entity.ContentTypeFieldType, ids []int) (map[int]string, error) {
	names := revisionFieldTypes[t].names
	if names == nil {
		return map[int]string{}, nil
	}
	return names(ctx, s, ids)
}

// Revert brings the revision revID of node nodeID back as user: its
// title, field values and SEO; the publishing, the address, the author and
// the date stay as they are. A term, a content item or a file it names
// that no longer exists is left out - dropped says how many. The result is
// a revision of its own.
func (s *NodeRevisionService) Revert(ctx context.Context, nodeID, revID int, user *entity.User) (dropped int, err error) {
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		rev, err := s.Revision(ctx, nodeID, revID)
		if err != nil {
			return err
		}
		node, err := s.d.Nodes.ByID(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("loading node %d: %w", nodeID, err)
		}
		def, fields, ok, err := s.typeOf(ctx, node.Type.ToString())
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: no history for node %d", entity.ErrEntityNotFound, nodeID)
		}
		repo := s.d.Dynamic(*def, fields)
		cur, err := repo.ByNodeID(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("loading node %d with its values: %w", nodeID, err)
		}
		values, formats := rev.Snapshot.Values(fields)
		if dropped, err = s.dropMissing(ctx, fields, values); err != nil {
			return err
		}
		if err := s.revertBlocks(ctx, fields, values, &dropped); err != nil {
			return err
		}
		if err := s.baseline(ctx, nodeID); err != nil {
			return err
		}
		updated := *cur
		updated.Node.Title = rev.Snapshot.Title
		updated.Values, updated.TextFormats = values, formats
		if _, err := repo.Update(ctx, &updated); err != nil {
			return fmt.Errorf("reverting node %d to revision %d: %w", nodeID, revID, err)
		}
		if err := s.d.Nodes.UpsertPageSEO(ctx, nodeID, rev.Snapshot.MetaTitle, rev.Snapshot.MetaDescription); err != nil {
			return fmt.Errorf("reverting the SEO of node %d: %w", nodeID, err)
		}
		// The terms index covers the taxonomy fields' terms (as a save does).
		termIDs, err := s.d.Terms.FieldTermIDsForNode(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("loading the field terms of node %d: %w", nodeID, err)
		}
		if err := s.d.Terms.UpdateNodeTerms(ctx, nodeID, termIDs); err != nil {
			return fmt.Errorf("updating the terms of node %d: %w", nodeID, err)
		}
		return s.record(ctx, nodeID, user, entity.RevisionRevert, rev.CreatedAt)
	})
	return dropped, err
}

// revertBlocks makes the blocks fields' items in values (a revision's:
// their Snapshot) new ones of their sets as the sets are now, without
// what no longer exists (dropMissing, counted in dropped); an item of a
// set that's gone is left out.
func (s *NodeRevisionService) revertBlocks(ctx context.Context, fields entity.FieldInstanceDefs, values map[int]any, dropped *int) error {
	for _, f := range fields {
		if f.FieldType != entity.FieldTypeBlocks {
			continue
		}
		var items []entity.BlockItem
		for _, item := range entity.BlockItems(values[f.ID]) {
			if item.Snapshot == nil {
				continue
			}
			_, setFields, ok, err := s.typeOf(ctx, item.Type)
			if err != nil {
				return err
			}
			if !ok {
				*dropped++
				continue
			}
			iv, formats := item.Snapshot.Values(setFields)
			n, err := s.dropMissing(ctx, setFields, iv)
			if err != nil {
				return err
			}
			*dropped += n
			items = append(items, entity.BlockItem{Type: item.Type, Fields: setFields, Values: iv, TextFormats: formats})
		}
		values[f.ID] = items
	}
	return nil
}

// dropMissing removes from values the terms, content items and files
// and users that no longer exist; how many.
func (s *NodeRevisionService) dropMissing(ctx context.Context, fields entity.FieldInstanceDefs, values map[int]any) (int, error) {
	dropped := 0
	for _, f := range fields {
		v, ok := values[f.ID]
		if !ok {
			continue
		}
		if revisionFieldTypes[f.FieldType].names == nil {
			continue
		}
		ids := idValues(v)
		names, err := s.names(ctx, f.FieldType, ids)
		if err != nil {
			return 0, err
		}
		kept := slices.DeleteFunc(slices.Clone(ids), func(id int) bool {
			_, ok := names[id]
			return !ok
		})
		dropped += len(ids) - len(kept)
		switch {
		case len(kept) == len(ids):
		case isIDList(v):
			values[f.ID] = kept
		default:
			delete(values, f.ID)
		}
	}
	return dropped, nil
}
