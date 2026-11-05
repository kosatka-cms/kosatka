package entity

import "time"

// FieldStorageDef - a reusable field definition (machine name, type,
// config) shared by every FieldInstanceDef that attaches it to a content
// type. field_value_* rows reference this ID, so a shared field's values
// live in one place across content types.
type FieldStorageDef struct {
	ID          int
	MachineName string
	FieldType   ContentTypeFieldType
	Config      map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// FieldStorageDefs - a list of field storages.
type FieldStorageDefs []FieldStorageDef

// VocabularyID - for FieldTypeTaxonomy: the vocabulary the select is built
// from (required). Shared by every content type using the field.
func (f FieldStorageDef) VocabularyID() int {
	if v, ok := f.Config["vocabulary_id"]; ok {
		if n, ok := v.(float64); ok { // JSON numbers decode as float64
			return int(n)
		}
	}
	return 0
}

// TargetContentType - for FieldTypeReference: optional target type machine
// name filter; empty means no filter.
func (f FieldStorageDef) TargetContentType() string {
	if v, ok := f.Config["target_content_type"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// HasExplicitMaxLen reports whether config sets max_len. Textarea/WYSIWYG
// fields get a length check only when it is set explicitly.
func (f FieldStorageDef) HasExplicitMaxLen() bool {
	if v, ok := f.Config["max_len"]; ok {
		if n, ok := v.(float64); ok && n > 0 {
			return true
		}
	}
	return false
}

// MaxLen - for FieldTypeText; defaults to 255 (the WidgetInput limit). For
// other widgets it applies only when HasExplicitMaxLen is true.
func (f FieldStorageDef) MaxLen() int {
	if v, ok := f.Config["max_len"]; ok {
		if n, ok := v.(float64); ok && n > 0 {
			return int(n)
		}
	}
	return 255
}

// Multiple - the "multiple" config flag, same as FieldInstanceDef.Multiple,
// for callers that only have the storage definition.
func (f FieldStorageDef) Multiple() bool {
	if v, ok := f.Config["multiple"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// ContentTypeFieldType - the field storage type; each maps to its own
// typed field_value_* table.
type ContentTypeFieldType string

// The field storage types.
const (
	// FieldTypeText covers both short strings and rich long-form content
	// (stored in field_value_text). Editing/rendering is chosen by
	// Widget(), not by the storage type.
	FieldTypeText      ContentTypeFieldType = "text"
	FieldTypeNumber    ContentTypeFieldType = "number"
	FieldTypeDate      ContentTypeFieldType = "date"
	FieldTypeBool      ContentTypeFieldType = "bool"
	FieldTypeTaxonomy  ContentTypeFieldType = "taxonomy"
	FieldTypeReference ContentTypeFieldType = "reference"
	FieldTypeImage     ContentTypeFieldType = "image"
	// FieldTypeFile - a generic (non-image) upload such as PDF/DOCX/ZIP.
	// Stored like FieldTypeImage (value = entity.File ID); differs only in
	// upload validation (see ResolveFieldUploadPolicy) and display (a
	// download link instead of a thumbnail).
	FieldTypeFile ContentTypeFieldType = "file"
	// FieldTypeList - a fixed, admin-defined set of value/label pairs
	// (see AllowedValues). Stored in field_value_list, or
	// field_value_list_multi when Multiple(). Widget() picks the control:
	// WidgetSelect (any cardinality), WidgetRadio (single only) or
	// WidgetCheckboxes (multiple only).
	FieldTypeList ContentTypeFieldType = "list"
	// FieldTypeLink - web addresses with optional titles (Link), stored in
	// field_value_link in the editor's order whatever the cardinality.
	// Shown as links, or by a module's FieldFormatter (a video player,
	// say): the field instance's DetailFormatter, a View field's own.
	FieldTypeLink ContentTypeFieldType = "link"
	// FieldTypeUser - site users (their IDs, []int in the editor's order
	// whatever the cardinality, UserIDs), stored in field_value_user:
	// authors, members, the people in charge. Shown as links to their
	// profiles; a View's profile tab lists the content a user is in
	// (ViewFilterOperatorIsProfileUser).
	FieldTypeUser ContentTypeFieldType = "user"
	// FieldTypeEmail, FieldTypePhone - an e-mail address and a telephone
	// number, stored as text (field_value_text), checked on saving and
	// shown as a "write"/"call" link (an address hidden from harvesters
	// unless the field says otherwise: EmailPlain).
	FieldTypeEmail ContentTypeFieldType = "email"
	FieldTypePhone ContentTypeFieldType = "phone"
	// FieldTypeBlocks - composite blocks: items of a field set (a content
	// type with Embedded, BlockTypes), each a node of its own with its
	// parent (Node.ParentID), stored in field_value_block in the editor's
	// order. Its value - []BlockItem as DynamicContentRepository reads
	// it, item IDs in a list's batch (BlockItems).
	FieldTypeBlocks ContentTypeFieldType = "blocks"
	// FieldTypeGroup has no stored value. It is a container other fields
	// point at via FieldInstanceDef.ParentGroupID/FormGroupID so they
	// render as one section with a shared heading.
	FieldTypeGroup ContentTypeFieldType = "group"
)

// FileIDs - the files of an image or file field's value: one (an int) or,
// for a field with several values, a slice in the editor's order; none for
// anything else.
func FileIDs(v any) []int {
	switch id := v.(type) {
	case int:
		if id > 0 {
			return []int{id}
		}
	case []int:
		return id
	}
	return nil
}

// FirstFileID - the first file of an image or file field's value
// (FileIDs), 0 - none: where one picture stands for the content (a
// teaser, a feed item, a search result).
func FirstFileID(v any) int {
	if ids := FileIDs(v); len(ids) > 0 {
		return ids[0]
	}
	return 0
}

// FieldStorageUsage - what still holds a field storage: its instances and
// the configuration that references it. The storage leaves the catalog
// only when nothing holds it.
type FieldStorageUsage struct {
	Instances int
	// Views - fields, filters and sorts of Views plus displays that use it
	// for carousel links or a glossary.
	Views           int
	BreadcrumbRules int
	SEORules        int
}

// References - references other than instances.
func (u FieldStorageUsage) References() int {
	return u.Views + u.BreadcrumbRules + u.SEORules
}

// Removable - nothing holds the storage.
func (u FieldStorageUsage) Removable() bool {
	return u.Instances == 0 && u.References() == 0
}
