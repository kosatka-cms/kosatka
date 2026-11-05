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

// ViewFieldDef - one field displayed by a ViewDisplayDef. Independent of
// ViewFilterDef - a field can be shown without filtering by it, and vice
// versa.
type ViewFieldDef struct {
	ent.Schema
}

// Annotations of the ViewFieldDef.
func (ViewFieldDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("view_fields"),
	}
}

// Fields of the ViewFieldDef.
func (ViewFieldDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("view_display_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		// field_storage_id nil means this field reads builtin_field instead of
		// a generic field (same either/or as ViewFilterDef).
		field.Int("field_storage_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// builtin_field: "title" | "main_image", used only when field_storage_id
		// is nil. These have no field_storages row, so they are synthesized
		// separately (see views.BuildContentFieldInstances).
		field.String("builtin_field").MaxLen(32).Optional(),
		// label - display label in this View. Empty falls back to the field's
		// machine name at render time; never defaulted here, so the admin UI can
		// distinguish "left blank" from "set to the same text".
		field.String("label").MaxLen(255),
		field.Bool("show_label").Default(true),
		// image_style - FieldTypeImage only.
		field.String("image_style").MaxLen(64).Optional(),
		// date_format - date fields (and the created_at builtin) only: date_formats
		// machine name; empty uses the field instance's format, else the default.
		field.String("date_format").MaxLen(64).Optional(),
		// tag_format - FieldTypeTaxonomy only: "plain" (default, comma-joined
		// names), "linked" (comma-joined links) or "badges" (linked pill list).
		field.String("tag_format").MaxLen(20).Optional(),
		// image_display - FieldTypeImage only: "" (the first image) or
		// "all" (every image of a field with several).
		field.String("image_display").MaxLen(10).Optional(),
		// formatter/layout - FieldTypeLink only: the formatter showing the
		// field ("" - links) and how its several values show
		// (entity.FieldLayout), like the instance's detail_formatter/detail_layout.
		field.String("formatter").MaxLen(64).Optional(),
		field.String("layout").MaxLen(16).Optional(),
		// link_to: "" | "content" | "file" (see entity.ViewFieldLinkTarget);
		// "file" applies only to FieldTypeImage/main_image.
		field.String("link_to").MaxLen(20).Optional(),
		// css_class - extra class on the field's wrapper (grid card / table cell),
		// added to the default item-field-{FieldType} class.
		field.String("css_class").MaxLen(255).Optional(),
		// rewrite_html - admin HTML wrapping the rendered value via a literal
		// "{{value}}" token, substituted with strings.Replace (never parsed as a
		// Go template; see rewriteFieldValue in service.DefaultTemplateService).
		// Sanitized with htmlsanitize.SanitizeAdmin on save.
		field.Text("rewrite_html").SchemaType(mysqlType("mediumtext")).Optional(),
		// trim_length - 0 shows the value as-is; >0 strips HTML and truncates to
		// this many characters.
		field.Int("trim_length").SchemaType(mysqlType("int")).Default(0),
		field.Int("sort_order").SchemaType(mysqlType("int")).Default(0),
		// exclude_from_display - the value is still resolved (usable as a
		// {{field:<key>}} token in another field's rewrite_html) but not rendered.
		field.Bool("exclude_from_display").Default(false),
		// hide_in_overflow - applies when the display's overflow_after > 0: the
		// field is rendered in the first overflow_after items but skipped in the
		// compact overflow list below them.
		field.Bool("hide_in_overflow").Default(false),
		// fallback_field_key - when this field is empty for a node, show the value
		// of another field in the same row instead (same key namespace as
		// rewrite_html's {{field:<key>}} tokens).
		field.String("fallback_field_key").MaxLen(64).Optional(),
		// params - a module's field's settings in this View (how it is
		// shown); empty for the others.
		field.JSON("params", map[string]string{}).Optional(),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Edges of the ViewFieldDef.
func (ViewFieldDef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("view_display", ViewDisplayDef.Type).Field("view_display_id").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_storage_id").Unique().Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

// Indexes of the ViewFieldDef.
func (ViewFieldDef) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("view_display_id", "sort_order").StorageKey("idx_view_fields_view_sort"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("field_storage_id"),
	}
}
