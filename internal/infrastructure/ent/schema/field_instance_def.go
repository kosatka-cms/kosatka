package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// FieldInstanceDef - a field attached to a content type, with per-type
// settings. It points at a reusable FieldStorageDef
// (machine_name/field_type/config shared across content types). Ordered by
// sort_order; no hierarchy.
type FieldInstanceDef struct {
	ent.Schema
}

// Annotations of the FieldInstanceDef.
func (FieldInstanceDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_instances"),
	}
}

// Fields of the FieldInstanceDef.
func (FieldInstanceDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("content_type_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_storage_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		// config - per-instance settings (currently just "widget"). Shared settings
		// such as cardinality, allowed values and max length live in
		// FieldStorageDef.config. See entity.FieldInstanceDef.WidgetConfig.
		field.JSON("config", map[string]any{}).Optional(),
		field.String("label").MaxLen(255).NotEmpty(),
		field.Bool("required").Default(false),
		// hidden_in_form - hides the input on the create/edit form; the stored value
		// is carried forward on save instead of being cleared (see
		// the hidden hooks of formFieldTypes).
		field.Bool("hidden_in_form").Default(false),
		// default_image_id - FieldTypeImage only: fallback file shown when the
		// field is empty, regardless of image style. NULL = not configured. See
		// DynamicContentController.imageFieldURLs.
		field.Int("default_image_id").SchemaType(mysqlType("int unsigned")).Optional().Nillable(),
		// show_on_detail/show_label - detail page display: show_on_detail=false
		// hides the field entirely; show_label=false hides only its heading.
		field.Bool("show_on_detail").Default(true),
		field.Bool("show_label").Default(true),
		// renders_in_body - ungrouped FieldTypeText only: render as a full-width
		// body section on the detail page instead of in the fact box; also used as
		// the node's body text for the meta description fallback (see
		// entity.DynamicContent.GetContent). Independent of the widget and text format.
		field.Bool("renders_in_body").Default(false),
		// detail_image_style/list_image_style - FieldTypeImage only: the
		// imagestyle.ImageStyleDef.Name used on the detail/list page. NULL on an
		// image field means not configured - nothing is rendered. Per instance, so
		// the same field can use different styles on different content types.
		field.String("detail_image_style").MaxLen(64).Optional(),
		field.String("list_image_style").MaxLen(64).Optional(),
		// detail_formatter/detail_layout - FieldTypeLink only: the formatter
		// showing the field on the detail page ("" - links) and how its several
		// values show (entity.FieldLayout). See entity.FieldInstanceDef.DetailFormatter.
		field.String("detail_formatter").MaxLen(64).Optional(),
		field.String("detail_layout").MaxLen(16).Optional(),
		// date_format - FieldTypeDate only: date_formats machine name used to
		// display the value; empty means the default format.
		field.String("date_format").MaxLen(64).Optional(),
		// sort_order - order in the create/edit form (node/edit_dynamic.html).
		field.Int("sort_order").SchemaType(mysqlType("int")).Default(0),
		// display_order - order on the detail page, independent of sort_order.
		field.Int("display_order").SchemaType(mysqlType("int")).Default(0),
		// parent_group_id - the FieldTypeGroup field on the same content type this
		// field belongs to on the detail page; NULL = ungrouped. Groups can't nest.
		// See FieldTypeGroup.
		field.Int("parent_group_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// form_group_id - like parent_group_id, but for the create/edit form
		// (one accordion section per group); independent of parent_group_id. See
		// entity.FieldInstanceDef.FormGroupID.
		field.Int("form_group_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// usable_in_form/usable_in_detail - group fields only: whether the group can
		// be targeted by form_group_id and/or parent_group_id. See
		// entity.FieldInstanceDef.UsableInForm.
		field.Bool("usable_in_form").Default(false),
		field.Bool("usable_in_detail").Default(false),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Edges of the FieldInstanceDef.
func (FieldInstanceDef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("content_type", ContentTypeDef.Type).Field("content_type_id").Required().Unique(),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_storage_id").Required().Unique(),
		edge.To("default_image", File.Type).Field("default_image_id").Unique().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("parent_group", FieldInstanceDef.Type).Field("parent_group_id").Unique().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("group_members", FieldInstanceDef.Type).Ref("parent_group"),
		edge.To("form_group", FieldInstanceDef.Type).Field("form_group_id").Unique().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("form_group_members", FieldInstanceDef.Type).Ref("form_group"),
	}
}

// Indexes of the FieldInstanceDef.
func (FieldInstanceDef) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_type_id", "field_storage_id").Unique().StorageKey("uk_content_type_fields_storage"),
		index.Fields("content_type_id", "sort_order").StorageKey("idx_content_type_fields_sort"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("default_image_id"),
		index.Fields("field_storage_id"),
		index.Fields("form_group_id"),
		index.Fields("parent_group_id"),
	}
}
