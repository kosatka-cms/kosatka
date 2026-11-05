package entity

import (
	"fmt"
	"strings"
	"time"
)

// FieldWidget - how a field is edited/displayed, independent of its
// storage type. Read via FieldInstanceDef.Widget().
type FieldWidget string

// Widgets for FieldTypeText.
const (
	// WidgetInput - single-line plain text, escaped on render. The
	// default when unset.
	WidgetInput FieldWidget = "input"
	// WidgetTextarea - multi-line plain text, escaped on render.
	WidgetTextarea FieldWidget = "textarea"
	// WidgetSafeHTML - textarea with HTML sanitized by pkg/htmlsanitize on
	// save and rendered unescaped.
	WidgetSafeHTML FieldWidget = "safe_html"
	// WidgetWYSIWYG - TinyMCE-backed textarea, its HTML sanitized on save
	// (htmlsanitize.SanitizeEditor) and rendered unescaped.
	WidgetWYSIWYG FieldWidget = "wysiwyg"
)

// Widgets for FieldTypeList (options from AllowedValues) and
// FieldTypeTaxonomy (options from the vocabulary's terms). They share
// the Config "widget" key with the text widgets but have their own
// allow-list (see admin/content_type_field_form.go's listWidgets/taxonomyWidgets).
const (
	// WidgetSelect - <select>, or <select multiple> when Multiple().
	// Taxonomy terms are indented by hierarchy.
	WidgetSelect FieldWidget = "select"
	// WidgetRadio - radio-button group; valid only when !Multiple().
	WidgetRadio FieldWidget = "radio"
	// WidgetCheckboxes - checkbox group; valid only when Multiple().
	WidgetCheckboxes FieldWidget = "checkboxes"
)

// Taxonomy-only widget. Cardinality is validated on field update
// (ContentTypeFieldUpdate).
const (
	// WidgetAutocomplete - Tagify free-tagging widget (comma-separated term
	// IDs and/or "new:<name>" tokens, see resolveTags);
	// valid only when Multiple(). The only widget that can create terms
	// on the fly; the default for multi-value taxonomy fields.
	WidgetAutocomplete FieldWidget = "autocomplete"
)

// FieldInstanceDef - a field attached to a ContentTypeDef with its
// per-type settings (label, required, order, display, widget). The
// definition itself (machine_name, field type, storage config) lives on
// the shared FieldStorageDef (field_storage.go), so one field can be
// attached to several content types sharing stored values
// (field_value_*.field_id references FieldStorageDef.ID).
//
// MachineName/FieldType/Config are denormalized from the joined
// FieldStorageDef (see dao.ToFieldInstanceDefEntity).
type FieldInstanceDef struct {
	ID             int
	ContentTypeID  int
	FieldStorageID int
	MachineName    string
	Label          string
	FieldType      ContentTypeFieldType
	Required       bool
	// HiddenInForm hides the field on the create/edit form while keeping
	// its stored value: the save path carries the existing value forward
	// instead of treating it as cleared (see
	// the hidden hooks of formFieldTypes).
	HiddenInForm bool
	// DefaultImageID - FieldTypeImage only: fallback entity.File shown when
	// the field is empty; nil - not configured. Rendered through the same
	// image styles as a real value (see
	// DynamicContentController.imageFieldURLs).
	DefaultImageID *int
	// ShowOnDetail/ShowLabel - whether the field and its label show on
	// the detail page (dynamic/page.html). Set via the detail-display tab
	// (ContentTypeService.UpdateFieldDetailDisplay), never the field form.
	ShowOnDetail bool
	ShowLabel    bool
	// RendersInBody - for an ungrouped FieldTypeText field: renders as a
	// full-width body section instead of in the fact-box (see
	// buildDisplaySections) and serves as the node body for the
	// meta-description fallback (DynamicContent.GetContent). A layout
	// choice, independent of Widget() and text formats. Set via
	// ContentTypeService.UpdateFieldDetailDisplay.
	RendersInBody bool
	// ShowLabelInList, ExcludeFromDisplay, FallbackFieldKey and TrimLength
	// are only populated on the synthetic FieldInstanceDefs Views builds
	// for dynamic/list.html (see
	// views.BuildContentFieldInstances), mirroring
	// the same-named ViewFieldDef settings. Never set on DB-backed fields.
	ShowLabelInList bool
	// ExcludeFromDisplay - the value is resolved but gets no slot of its
	// own; usable only as a {{field:<key>}} RewriteHTML token.
	ExcludeFromDisplay bool
	// FallbackFieldKey - see ApplyFieldFallbacks.
	FallbackFieldKey string
	// TrimLength - >0 strips HTML and truncates the value to that many
	// characters at render time.
	TrimLength int
	// DetailImageStyle - FieldTypeImage only: ImageStyleDef.Name used on
	// the detail page; "" - not configured. ListImageStyle - in lists:
	// the type's cards (entity.TeaserItem), a blocks field's items; "" -
	// the original. A View's field has its own (ViewFieldDef.ImageStyle).
	DetailImageStyle string
	ListImageStyle   string
	// DetailFormatter - FieldTypeLink only: the FieldFormatter showing the
	// field on the detail page ("" - links); DetailLayout - how its several
	// values show there. A Views field has its own (ViewFieldDef.Formatter,
	// on the synthetic instance's Config "formatter"/"layout").
	DetailFormatter string
	DetailLayout    FieldLayout
	// DateFormat - FieldTypeDate only: entity.DateFormat machine name;
	// "" - the default format. Views may override it
	// (ViewFieldDef.DateFormat).
	DateFormat string
	// SortOrder orders the create/edit form; DisplayOrder orders the
	// detail page. Each is set by its own admin tab's drag-and-drop.
	SortOrder    int
	DisplayOrder int
	// Config - FieldStorageDef.Config (denormalized), shared by every
	// content type the field is attached to. Holds only storage/validation
	// constraints: vocabulary_id, multiple, allowed_values,
	// target_content_type, max_len, pattern.
	Config map[string]any
	// WidgetConfig - per-content-type settings for editing the field
	// ("widget", text formats, upload settings), so the same storage can
	// be WYSIWYG on one type and a plain textarea on another.
	WidgetConfig map[string]any
	// ParentGroupID - the FieldTypeGroup field (on the same content type)
	// this field belongs to on the detail page; nil - none. Groups can't
	// nest. Set via ContentTypeService.UpdateFieldDetailDisplay.
	ParentGroupID *int
	// FormGroupID - like ParentGroupID but for the create/edit form
	// (node/edit_dynamic.html renders one collapsible section per group).
	// Independent of ParentGroupID: form and detail page may group fields
	// differently. nil - the default form section.
	FormGroupID *int
	// UsableInForm/UsableInDetail - FieldTypeGroup only: whether the group
	// can be targeted by FormGroupID and/or ParentGroupID, so a form-only
	// group doesn't appear in the detail-page group picker and vice versa.
	// Always false for other field types.
	UsableInForm   bool
	UsableInDetail bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FieldInstanceDefs - a list of fields of a content type.
type FieldInstanceDefs []FieldInstanceDef

// FieldDetailDisplay - a field's settings of the "Detail page" tab
// (ContentTypeRepository.UpdateFieldDetailDisplay): its place, whether it
// and its label show, and how - the image style (image fields), the
// formatter and the layout of several values (link fields).
type FieldDetailDisplay struct {
	DisplayOrder  int
	ShowOnDetail  bool
	ShowLabel     bool
	RendersInBody bool
	ImageStyle    string
	// ListImageStyle - FieldInstanceDef.ListImageStyle.
	ListImageStyle string
	Formatter      string
	Layout         FieldLayout
	// ParentGroupID - the group on the page; nil - none.
	ParentGroupID *int
}

// ParentGroupIDValue - ParentGroupID as a plain int (0 = no group), for
// templates: html/template can't dereference a *int in comparisons.
func (f FieldInstanceDef) ParentGroupIDValue() int {
	if f.ParentGroupID == nil {
		return 0
	}
	return *f.ParentGroupID
}

// FormGroupIDValue - FormGroupID as a plain int (0 = no group), see
// ParentGroupIDValue.
func (f FieldInstanceDef) FormGroupIDValue() int {
	if f.FormGroupID == nil {
		return 0
	}
	return *f.FormGroupID
}

// ApplyFieldFallbacks fills empty field values from another field: for
// each field with a FallbackFieldKey whose value is empty, it copies the
// value of the field whose MachineName matches the key (same namespace
// as RewriteHTML's {{field:<key>}} tokens). Missing or empty targets are
// ignored. values is mutated in place; call it after all values are
// resolved.
func ApplyFieldFallbacks(fields FieldInstanceDefs, values map[int]any) {
	byKey := make(map[string]int, len(fields))
	for _, f := range fields {
		if f.MachineName != "" {
			byKey[f.MachineName] = f.ID
		}
	}
	for _, f := range fields {
		if f.FallbackFieldKey == "" || !isEmptyFieldValue(values[f.ID]) {
			continue
		}
		targetID, ok := byKey[f.FallbackFieldKey]
		if !ok || targetID == f.ID {
			continue
		}
		if v, ok := values[targetID]; ok && !isEmptyFieldValue(v) {
			values[f.ID] = v
		}
	}
}

// isEmptyFieldValue reports whether v is a zero/blank value, matching the
// templates' {{if $value}} convention.
func isEmptyFieldValue(v any) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(val) == ""
	case int:
		return val == 0
	case float64:
		return val == 0
	case bool:
		return !val
	case time.Time:
		return val.IsZero()
	case []Link:
		return len(val) == 0
	default:
		return false
	}
}

// VocabularyID - FieldTypeTaxonomy: the vocabulary terms are chosen
// from. Required for this field type.
func (f FieldInstanceDef) VocabularyID() int {
	if v, ok := f.Config["vocabulary_id"]; ok {
		if n, ok := v.(float64); ok { // JSON numbers decode as float64
			return int(n)
		}
	}
	return 0
}

// Multiple - for the types that allow it (AllowsMultiple): several
// values per (node, field), stored in the *_multi value table (a link
// field's in its one table). Set at creation
// (ContentTypeService.CreateField); changed only by
// ContentTypeService.SetFieldMultiple, which moves the values between the
// tables.
func (f FieldInstanceDef) Multiple() bool {
	if v, ok := f.Config["multiple"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// Widget returns the configured widget from WidgetConfig (per content
// type, unlike Config), or WidgetInput when unset. Valid values depend on
// the field type - see the FieldWidget constants. Changing it doesn't
// affect stored values.
func (f FieldInstanceDef) Widget() FieldWidget {
	if v, ok := f.WidgetConfig["widget"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return FieldWidget(s)
		}
	}
	return WidgetInput
}

// Symmetric - for a Multiple() FieldTypeReference: the relation is mutual
// (A references B ⇔ B references A through the same field). On save,
// back-references on added/removed targets are synced (the database's
// syncSymmetricReferences), so the relation can be edited from either
// side.
func (f FieldInstanceDef) Symmetric() bool {
	b, _ := f.Config["symmetric"].(bool)
	return b
}

// TargetContentType - FieldTypeReference: optional comma-separated
// machine names of allowed target types; "" - no filter.
func (f FieldInstanceDef) TargetContentType() string {
	if v, ok := f.Config["target_content_type"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// HasExplicitMaxLen reports whether config sets max_len explicitly. The
// 255 default of MaxLen applies only to WidgetInput; long-form widgets
// get a length check only when max_len is set.
func (f FieldInstanceDef) HasExplicitMaxLen() bool {
	if v, ok := f.Config["max_len"]; ok {
		if n, ok := v.(float64); ok && n > 0 {
			return true
		}
	}
	return false
}

// MaxLen - FieldTypeText max length, 255 by default. For widgets other
// than WidgetInput it applies only if HasExplicitMaxLen().
func (f FieldInstanceDef) MaxLen() int {
	if v, ok := f.Config["max_len"]; ok {
		if n, ok := v.(float64); ok && n > 0 {
			return int(n)
		}
	}
	return 255
}

// Pattern - FieldTypeText: optional RE2 regexp a non-empty value must
// match (e.g. a URL or code format); "" - no format check.
func (f FieldInstanceDef) Pattern() string {
	if v, ok := f.Config["pattern"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// PatternMessage - error message when Pattern() doesn't match; "" - a
// generic default is used.
func (f FieldInstanceDef) PatternMessage() string {
	if v, ok := f.Config["pattern_message"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// AllowedTextFormatIDs - FieldTypeText: TextFormatDef IDs the editor may
// pick per value. A non-empty list overrides Widget() for choosing the
// editor (see node/edit_dynamic.html's textFormatPicker). Empty - the
// field doesn't use text formats and Widget() decides as usual
// (WidgetSafeHTML and WidgetWYSIWYG sanitized with fixed policies). Per
// content type (WidgetConfig).
func (f FieldInstanceDef) AllowedTextFormatIDs() []int {
	raw, ok := f.WidgetConfig["allowed_text_formats"].([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(raw))
	for _, item := range raw {
		if n, ok := item.(float64); ok && n > 0 {
			out = append(out, int(n))
		}
	}
	return out
}

// RichText - a FieldTypeText field whose values are HTML: it uses text
// formats (sanitized per format on save) or the legacy WidgetSafeHTML /
// WidgetWYSIWYG widget. Templates output only such values unescaped;
// other text values are plain text and are escaped.
func (f FieldInstanceDef) RichText() bool {
	if f.FieldType != FieldTypeText {
		return false
	}
	if len(f.AllowedTextFormatIDs()) > 0 {
		return true
	}
	w := f.Widget()
	return w == WidgetSafeHTML || w == WidgetWYSIWYG
}

// AllowedFileExtensions - FieldTypeImage/FieldTypeFile: extensions
// without the dot (e.g. "jpg") accepted for this field. Per content type
// (WidgetConfig). Empty - FileUploadService.FieldUploadPolicy falls back
// to the site-wide UploadSettings for this setting.
func (f FieldInstanceDef) AllowedFileExtensions() []string {
	raw, ok := f.WidgetConfig["allowed_extensions"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// FileDirectory - Image/File field's upload directory pattern
// ("File directory"): storage-relative, may contain tokens such as
// [date:custom:Y]; resolved by files.ResolveFileDirectory. Empty -
// the default content directory. Field INSTANCE setting (WidgetConfig),
// like the other upload settings above.
func (f FieldInstanceDef) FileDirectory() string {
	s, _ := f.WidgetConfig["file_directory"].(string)
	return s
}

// PrivateFiles - FieldTypeFile, FieldTypeImage: new uploads go to the
// private files (entity.PrivateScheme), served - an image's style
// derivatives too - only to those who may see a content item they are
// attached to ("Private files"). Field INSTANCE setting
// (WidgetConfig); files already uploaded keep where they are.
func (f FieldInstanceDef) PrivateFiles() bool {
	v, _ := f.WidgetConfig["private_files"].(bool)
	return v
}

// MediaSource - where an Image/File field's files come from
// (FieldInstanceDef.MediaSource): uploads, the media library, or both.
type MediaSource string

// The media sources: both (the default), uploads only, the library only.
const (
	MediaSourceBoth    MediaSource = ""
	MediaSourceUpload  MediaSource = "upload"
	MediaSourceLibrary MediaSource = "library"
)

// MediaSource - FieldTypeFile, FieldTypeImage: whether the form uploads
// a file, picks one from the media library, or both (the default). Field
// INSTANCE setting (WidgetConfig "media_source").
func (f FieldInstanceDef) MediaSource() MediaSource {
	switch s := MediaSource(fmt.Sprint(f.WidgetConfig["media_source"])); s {
	case MediaSourceUpload, MediaSourceLibrary:
		return s
	}
	return MediaSourceBoth
}

// AllowsUpload and AllowsLibrary - MediaSource: the form uploads, picks
// from the media library.
func (f FieldInstanceDef) AllowsUpload() bool { return f.MediaSource() != MediaSourceLibrary }

// AllowsLibrary - the form picks a file from the media library
// (MediaSource).
func (f FieldInstanceDef) AllowsLibrary() bool { return f.MediaSource() != MediaSourceUpload }

// HasMaxUploadSize reports whether this field sets its own max upload
// size (otherwise the site-wide limit applies).
func (f FieldInstanceDef) HasMaxUploadSize() bool {
	v, ok := f.WidgetConfig["max_filesize_bytes"]
	if !ok {
		return false
	}
	n, ok := v.(float64)
	return ok && n > 0
}

// MaxUploadSizeBytes - FieldTypeImage/FieldTypeFile max upload size;
// meaningful only if HasMaxUploadSize().
func (f FieldInstanceDef) MaxUploadSizeBytes() int {
	if v, ok := f.WidgetConfig["max_filesize_bytes"]; ok {
		if n, ok := v.(float64); ok && n > 0 {
			return int(n)
		}
	}
	return 0
}

// MaxValues - how many images or links a FieldTypeImage/FieldTypeLink
// field with several values holds at most (widget config "max_values"); 0 -
// no limit.
func (f FieldInstanceDef) MaxValues() int {
	return intFromWidgetConfig(f.WidgetConfig, "max_values")
}

// MaxResolution and MinResolution - FieldTypeImage resolution limits. (0, 0)
// means unset: max falls back to the site-wide
// UploadSettings.MaxImageDimension (as a square bound), min - no minimum.
func (f FieldInstanceDef) MaxResolution() (width, height int) {
	return intFromWidgetConfig(f.WidgetConfig, "max_resolution_width"),
		intFromWidgetConfig(f.WidgetConfig, "max_resolution_height")
}

// MinResolution - the FieldTypeImage minimum resolution; (0, 0) - none.
func (f FieldInstanceDef) MinResolution() (width, height int) {
	return intFromWidgetConfig(f.WidgetConfig, "min_resolution_width"),
		intFromWidgetConfig(f.WidgetConfig, "min_resolution_height")
}

func intFromWidgetConfig(config map[string]any, key string) int {
	if v, ok := config[key]; ok {
		if n, ok := v.(float64); ok && n > 0 {
			return int(n)
		}
	}
	return 0
}

// AllowedFileExtensionsString - space-separated, the format of the
// "Allowed file extensions" input (see fieldWidgetConfigFromForm).
func (f FieldInstanceDef) AllowedFileExtensionsString() string {
	return strings.Join(f.AllowedFileExtensions(), " ")
}

// MaxUploadSizeMB - MaxUploadSizeBytes() in MB for the admin form; 0 when
// unset.
func (f FieldInstanceDef) MaxUploadSizeMB() float64 {
	return float64(f.MaxUploadSizeBytes()) / 1024 / 1024
}

// MaxResolutionString and MinResolutionString - "WIDTHxHEIGHT" (the
// resolution input format), or "" when unset.
func (f FieldInstanceDef) MaxResolutionString() string {
	w, h := f.MaxResolution()
	if w == 0 && h == 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", w, h)
}

// MinResolutionString - MinResolution as "WIDTHxHEIGHT", or "" when
// unset.
func (f FieldInstanceDef) MinResolutionString() string {
	w, h := f.MinResolution()
	if w == 0 && h == 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", w, h)
}

// FieldAllowedValue - a stored value and its display label for
// FieldTypeList.
type FieldAllowedValue struct {
	Value string
	Label string
}

// AllowedValues - FieldTypeList options in the order entered. Stored as a
// JSON array (not a map) so the order survives encoding.
func (f FieldInstanceDef) AllowedValues() []FieldAllowedValue {
	raw, ok := f.Config["allowed_values"].([]any)
	if !ok {
		return nil
	}
	out := make([]FieldAllowedValue, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		v, _ := m["value"].(string)
		l, _ := m["label"].(string)
		if v == "" {
			continue
		}
		out = append(out, FieldAllowedValue{Value: v, Label: l})
	}
	return out
}

// AllowedValuesText serializes AllowedValues() as "value|Label" lines to
// prefill the field edit form's textarea.
func (f FieldInstanceDef) AllowedValuesText() string {
	values := f.AllowedValues()
	lines := make([]string, 0, len(values))
	for _, av := range values {
		lines = append(lines, av.Value+"|"+av.Label)
	}
	return strings.Join(lines, "\n")
}

// AllowedValueLabel returns the label for a stored value, or the value
// itself if it is no longer among AllowedValues() (option removed after
// being saved).
func (f FieldInstanceDef) AllowedValueLabel(value string) string {
	for _, av := range f.AllowedValues() {
		if av.Value == value {
			return av.Label
		}
	}
	return value
}

// ListItem - an option chosen in a FieldTypeList field: its value, its
// label and its place among the field's options (1 - the first; 0 - an
// option removed since), so a theme can style the options in their order.
type ListItem struct {
	Value string
	Label string
	Index int
}

// ListItems - the options chosen in a list field; printed, their labels
// joined with ", ".
type ListItems []ListItem

func (l ListItems) String() string {
	labels := make([]string, len(l))
	for i, it := range l {
		labels[i] = it.Label
	}
	return strings.Join(labels, ", ")
}

// ListItems - the chosen options of a FieldTypeList value v (one option's
// value, or several).
func (f FieldInstanceDef) ListItems(v any) ListItems {
	var values []string
	switch vals := v.(type) {
	case string:
		if vals != "" {
			values = []string{vals}
		}
	case []string:
		values = vals
	}
	options := f.AllowedValues()
	out := make(ListItems, 0, len(values))
	for _, val := range values {
		it := ListItem{Value: val, Label: val}
		for i, o := range options {
			if o.Value == val {
				it.Label, it.Index = o.Label, i+1
				break
			}
		}
		out = append(out, it)
	}
	return out
}

// SortBySortOrder returns a copy sorted by SortOrder, then ID (stable).
func (fs FieldInstanceDefs) SortBySortOrder() FieldInstanceDefs {
	sorted := make(FieldInstanceDefs, len(fs))
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

// SortByDisplayOrder returns a copy sorted by DisplayOrder, then ID.
func (fs FieldInstanceDefs) SortByDisplayOrder() FieldInstanceDefs {
	sorted := make(FieldInstanceDefs, len(fs))
	copy(sorted, fs)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			a, b := sorted[j-1], sorted[j]
			if a.DisplayOrder < b.DisplayOrder || (a.DisplayOrder == b.DisplayOrder && a.ID <= b.ID) {
				break
			}
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	return sorted
}
