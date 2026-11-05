package entity

import (
	"context"
	"slices"
	"time"
)

// ContentTypeDef - an admin-defined content type. Its MachineName becomes
// node.Type at runtime without a Go constant. Not to be confused with
// ContentType (node.go): a node's type name, an admin-defined type's or
// one a module declares (Module.NodeTypes).
type ContentTypeDef struct {
	ID          int
	MachineName string
	Label       string
	// PathPattern - the URL alias pattern of this type's nodes, tokens in
	// brackets like the SEO templates' ([node:type]/[node:title]); "" -
	// DefaultNodePathPattern. A node follows it while its PathAuto is
	// on (service.NodePathService).
	PathPattern string
	Description string
	// Status - 1 enabled, 0 disabled: a disabled type takes no new
	// content (Creatable); its existing content is shown and edited as
	// before.
	Status int
	// DefaultPromote/DefaultSticky - a new node's Node.Promote ("on the
	// front page") and Node.Sticky; an editor without "administer
	// content" gets them as they are.
	DefaultPromote bool
	DefaultSticky  bool
	// DefaultPublishing - how a new node comes out: published, on
	// moderation or unpublished; an editor without "administer content"
	// gets it as it is (and keeps an edited node's own), who may sets it
	// in the form.
	DefaultPublishing NodePublishing
	// DefaultCommentMode is copied onto Node.CommentMode when a node of
	// this type is created; later changes never affect existing nodes.
	DefaultCommentMode CommentMode
	// HideEmptyComments hides the comments section (including the form)
	// while the node has no comments.
	HideEmptyComments bool
	// TitleDisplay - how the page heading combines the title and the
	// alternative title (AltTitleField).
	TitleDisplay TitleDisplay
	// RedirectField - machine_name of a text field holding a URL; the node
	// page redirects there instead of rendering. "" or an empty value
	// renders the page normally.
	RedirectField string
	// AltTitleField - machine_name of a text field holding the alternative
	// (e.g. translated) title, "" - none. Used by TitleDisplay, the default
	// page title, the Views title field and TeaserItem.AltTitle.
	AltTitleField string
	// ShowSubmitted shows author and dates on the node page.
	ShowSubmitted bool
	// ShowViewCount shows the view counter next to author and dates.
	ShowViewCount bool
	// Revisions - every save of the type's nodes is kept as a revision
	// (NodeRevision): compared, brought back.
	Revisions bool
	// RevisionsKeep - how many of a node's revisions are kept, the oldest
	// pruned beyond; 0 - all.
	RevisionsKeep int
	// Embedded - a field set: its nodes are items of the blocks fields
	// (FieldTypeBlocks) of other content, not content of their own -
	// no page, address, lists, search or permissions; ContentTypes leaves
	// it out (FieldSets lists them).
	Embedded bool
	// ViewRoles - the roles who may view the type's content (role names;
	// RoleAnonymous for guests, RoleUser for everyone signed in); empty -
	// everyone. Its pages, lists, search, feeds and files are kept from
	// the others (service.ContentAccess).
	ViewRoles []string
	CreatedBy int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ViewableBy reports whether a visitor with roles (UserRoleNames) may
// view the type's content by its ViewRoles.
func (d ContentTypeDef) ViewableBy(roles []string) bool {
	if len(d.ViewRoles) == 0 {
		return true
	}
	for _, r := range roles {
		if slices.Contains(d.ViewRoles, r) {
			return true
		}
	}
	return false
}

// Creatable - new content of the type may be created: the type is
// enabled (Status).
func (d ContentTypeDef) Creatable() bool {
	return d.Status == 1
}

// ContentTypeDefs - a list of content types.
type ContentTypeDefs []ContentTypeDef

// TitleDisplay - see ContentTypeDef.TitleDisplay.
type TitleDisplay string

const (
	// TitleDisplayOriginalWithTranslation - "Title (Translated)".
	TitleDisplayOriginalWithTranslation TitleDisplay = "original_with_translation"
	// TitleDisplayTranslatedFirst - the heading is the translated title,
	// the original goes into a subtitle (the breadcrumb shows the
	// translated one).
	TitleDisplayTranslatedFirst TitleDisplay = "translated_first"
	// TitleDisplayOriginalOnly - just the title.
	TitleDisplayOriginalOnly TitleDisplay = "original_only"
)

// IsValidTitleDisplay reports whether v is one of the TitleDisplay values.
func IsValidTitleDisplay(v string) bool {
	switch TitleDisplay(v) {
	case TitleDisplayOriginalWithTranslation, TitleDisplayTranslatedFirst, TitleDisplayOriginalOnly:
		return true
	}
	return false
}

// Template-friendly string accessors for the admin field form
// (content_type_field_edit.html): html/template can't call methods
// returning (int, int) or render a []string as an input value.

// ContentTypeRepository - persistence port for content types, fields and
// generic field value storage.
type ContentTypeRepository interface {
	// Content type CRUD
	// ContentTypes - the content types; the field sets (Embedded) are
	// FieldSets'.
	ContentTypes(ctx context.Context) (ContentTypeDefs, error)
	// FieldSets - the field sets (ContentTypeDef.Embedded), by label.
	FieldSets(ctx context.Context) (ContentTypeDefs, error)
	ContentTypeByID(ctx context.Context, id int) (*ContentTypeDef, error)
	ContentTypeByMachineName(ctx context.Context, machineName string) (*ContentTypeDef, error)
	CreateContentType(ctx context.Context, ct ContentTypeDef) (*ContentTypeDef, error)
	UpdateContentType(ctx context.Context, ct ContentTypeDef) (*ContentTypeDef, error)
	// DeleteContentType deletes the type with its fields (groups among
	// them), each as DeleteField does; the caller checks the type has no
	// nodes and runs it in a transaction.
	DeleteContentType(ctx context.Context, id int) error

	// Field CRUD (flat list ordered by SortOrder)
	FieldsByContentTypeID(ctx context.Context, contentTypeID int) (FieldInstanceDefs, error)
	FieldByID(ctx context.Context, id int) (*FieldInstanceDef, error)
	// CreateField atomically creates a new FieldStorageDef and an instance
	// referencing it. The machine name must be globally unused, otherwise
	// ErrFieldStorageMachineNameTaken.
	CreateField(ctx context.Context, f FieldInstanceDef) (*FieldInstanceDef, error)
	// UpdateField writes FieldStorageDef.Config (shared by all types using
	// the field) and the instance's Label/Required/HiddenInForm/FormGroupID.
	UpdateField(ctx context.Context, f FieldInstanceDef) (*FieldInstanceDef, error)
	// DeleteField deletes the instance and its values on that type's nodes;
	// when it was the storage's last instance and nothing else references
	// the storage (FieldStorageUsage), the storage leaves the catalog too
	// (a storage goes with its last field).
	DeleteField(ctx context.Context, id int) error
	// FieldStorageUsage - what still holds a catalog field.
	FieldStorageUsage(ctx context.Context, storageID int) (FieldStorageUsage, error)
	// NodesWithSeveralFieldValues - the nodes holding more than one value
	// of a multi-value field, sorted by ID.
	NodesWithSeveralFieldValues(ctx context.Context, storage FieldStorageDef) ([]int, error)
	// SetFieldMultiple switches a field between one and several values per
	// node: moves its values to the other value table, sets the storage's
	// "multiple" (and drops "symmetric" when single) and replaces the
	// instances' widgets the new cardinality doesn't allow.
	SetFieldMultiple(ctx context.Context, storageID int, multiple bool) error

	// FieldStorages - the whole catalog of reusable fields, for the
	// "add existing field" picker.
	FieldStorages(ctx context.Context) (FieldStorageDefs, error)
	// FieldStorageByID - a single catalog entry (used by the Views query
	// layer).
	FieldStorageByID(ctx context.Context, id int) (*FieldStorageDef, error)
	// FieldInstanceLabelsByFieldStorageIDs returns a human-readable label
	// per field_storage_id, taken from its first (by id) instance, since
	// FieldStorageDef has no label. Takes explicit IDs so public View
	// rendering loads only the fields it shows.
	FieldInstanceLabelsByFieldStorageIDs(ctx context.Context, fieldStorageIDs []int) (map[int]string, error)
	// RichTextFieldStorageIDs - which of fieldStorageIDs hold HTML: every
	// instance of the storage is RichText (a View shows values of all of
	// them, and one plain instance stores unsanitized text).
	RichTextFieldStorageIDs(ctx context.Context, fieldStorageIDs []int) (map[int]bool, error)
	// AttachExistingField attaches an existing FieldStorageDef to a content
	// type. Returns ErrFieldAlreadyAttached if it is already attached.
	AttachExistingField(ctx context.Context, contentTypeID, fieldStorageID int, label string, required bool, sortOrder int) (*FieldInstanceDef, error)

	// UpdateFieldSortOrder/UpdateFieldDetailDisplay update only the
	// settings of their own admin tab, so one tab never overwrites
	// another's values. formGroupID/parentGroupID move the field into a
	// group; nil clears membership.
	UpdateFieldSortOrder(ctx context.Context, fieldID int, sortOrder int, formGroupID *int) error
	UpdateFieldDetailDisplay(ctx context.Context, fieldID int, d FieldDetailDisplay) error

	// CountFieldValues counts values stored for this field on this content
	// type (for confirming detachment). Scoped by contentTypeID because
	// value rows are shared by every type using the field.
	CountFieldValues(ctx context.Context, contentTypeID, fieldID int, fieldType ContentTypeFieldType, multiple bool) (int, error)

	// Generic value storage - fans out over the typed value tables inside
	// the adapter.
	FieldValues(ctx context.Context, nodeID int, fields FieldInstanceDefs) (map[int]any, error)
	// FieldValuesForNodes - batch FieldValues (nodeID→fieldID→value): one
	// query per field for all nodes instead of one per (node × field).
	// Supports text/number/date/bool and image (the file ID) fields.
	FieldValuesForNodes(ctx context.Context, nodeIDs []int, fields FieldInstanceDefs) (map[int]map[int]any, error)
	// SetFieldValues's textFormats is keyed like values (FieldInstanceDef.ID),
	// see DynamicContent.TextFormats. nil is fine when text formats aren't
	// used.
	SetFieldValues(ctx context.Context, nodeID int, fields FieldInstanceDefs, values map[int]any, textFormats map[int]int) error
	// TextFormatIDsByNode - for the edit form: the TextFormatDef.ID each
	// existing value was saved with, keyed by FieldInstanceDef.ID. Callers
	// should pass only fields with non-empty AllowedTextFormatIDs() to
	// skip the query when text formats aren't used.
	TextFormatIDsByNode(ctx context.Context, nodeID int, fields FieldInstanceDefs) (map[int]int, error)

	// FieldTextValuesForNodes/FieldNumberValuesForNodes/FieldDateValuesForNodes
	// batch-resolve one field (fieldStorageID is FieldStorageDef.ID) for
	// many nodes in one query: node_id → value, only for nodes that have
	// one. For listings that read one field across many nodes.
	FieldTextValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]string, error)
	FieldNumberValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]float64, error)
	FieldDateValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]time.Time, error)
	// FieldTaxonomyValuesForNodes - same batch shape, for a
	// FieldTypeTaxonomy field: node_id -> term_id. Used by catalog
	// listings to filter by a term (e.g. ?category=<term id>).
	FieldTaxonomyValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]int, error)
	// FieldTaxonomyMultiValuesForNodes is FieldTaxonomyValuesForNodes for
	// a Multiple() taxonomy field: node_id -> all its term IDs (from
	// field_value_taxonomy_multi). Used for OR-match filtering and
	// displaying multi-value fields outside the generic field loop.
	FieldTaxonomyMultiValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int][]int, error)
}

// NodePublishing - a content type's publishing of new nodes
// (ContentTypeDef.DefaultPublishing).
type NodePublishing string

const (
	// NodePublished - published at once, everyone sees it.
	NodePublished NodePublishing = "published"
	// NodeToModeration - published and on moderation: its author sees
	// it, the moderation queue lists it, the others don't until a
	// moderator approves it.
	NodeToModeration NodePublishing = "moderation"
	// NodeUnpublished - unpublished: only who may view unpublished
	// content sees it.
	NodeUnpublished NodePublishing = "unpublished"
)

// Valid - one of the publishings.
func (p NodePublishing) Valid() bool {
	return p == NodePublished || p == NodeToModeration || p == NodeUnpublished
}

// Flags - a new node's Node.Status and Node.OnModeration; an unknown
// publishing goes to moderation.
func (p NodePublishing) Flags() (status, onModeration int) {
	switch p {
	case NodePublished:
		return 1, 0
	case NodeUnpublished:
		return 0, 0
	}
	return 1, 1
}
