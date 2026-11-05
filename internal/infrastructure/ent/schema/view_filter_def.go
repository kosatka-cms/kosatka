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

// ViewFilterDef - one filter criterion of a ViewDisplayDef. Either fixed
// (value, set by the admin) or exposed (exposed_param, read from the query
// string) - never both.
type ViewFilterDef struct {
	ent.Schema
}

// Annotations of the ViewFilterDef.
func (ViewFilterDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("view_filters"),
	}
}

// Fields of the ViewFilterDef.
func (ViewFilterDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("view_display_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		// field_storage_id nil means this filter reads builtin_field instead of
		// a generic field.
		field.Int("field_storage_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// via_field_storage_id - entity.ViewFilterDef.ViaFieldStorageID: a
		// blocks field whose blocks the field is filtered in; NULL - the
		// node's own field.
		field.Int("via_field_storage_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// builtin_field - entity.ViewFilterBuiltinField, meaningful only when
		// field_storage_id is nil.
		field.String("builtin_field").MaxLen(32).Optional(),
		// operator - entity.ViewFilterOperator; the allowed ones depend on
		// the target field's type (the Views catalog).
		field.String("operator").MaxLen(32).NotEmpty(),
		// value - the fixed filter value, used only when exposed=false.
		field.Text("value").SchemaType(mysqlType("mediumtext")).Optional(),
		// value_is_relative - FieldTypeDate only: value is a relative expression
		// ("now", "+7 days", "-1 month") resolved against today's date on every
		// query (see entity.ParseRelativeDate) instead of a literal date. Ignored when
		// exposed=true.
		field.Bool("value_is_relative").Default(false),
		field.Bool("exposed").Default(false),
		// exposed_param - query parameter read when exposed=true (e.g. "category"
		// for ?category=news). Required only in that case.
		field.String("exposed_param").MaxLen(64).Optional(),
		// exposed_label - label on the public exposed-filters form; empty falls
		// back to the field's label at render time (never defaulted here).
		field.String("exposed_label").MaxLen(255).Optional(),
		// contextual - the value comes from the page (the node, term or user
		// it is for) instead of value/exposed_param: entity.ViewFilterDef.
		// Contextual.
		field.Bool("contextual").Default(false),
		// sort_order - position in the admin filter list and the exposed-filters
		// form.
		field.Int("sort_order").SchemaType(mysqlType("int")).Default(0),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Edges of the ViewFilterDef.
func (ViewFilterDef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("view_display", ViewDisplayDef.Type).Field("view_display_id").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_storage_id").Unique().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("via_field_storage", FieldStorageDef.Type).Field("via_field_storage_id").Unique().Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

// Indexes of the ViewFilterDef.
func (ViewFilterDef) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("view_display_id", "sort_order").StorageKey("idx_view_filters_view_sort"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("field_storage_id"),
		index.Fields("via_field_storage_id"),
	}
}
