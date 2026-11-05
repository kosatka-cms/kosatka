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

// ViewSortDef - one sort criterion of a ViewDisplayDef. Same
// field_storage_id/builtin_field either/or as ViewFilterDef.
type ViewSortDef struct {
	ent.Schema
}

// Annotations of the ViewSortDef.
func (ViewSortDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("view_sorts"),
	}
}

// Fields of the ViewSortDef.
func (ViewSortDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("view_display_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		// field_storage_id nil means this criterion sorts by builtin_field
		// instead of a generic field.
		field.Int("field_storage_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// builtin_field: "created_at" | "views" | "title" - meaningful
		// only when field_storage_id is nil.
		field.String("builtin_field").MaxLen(32).Optional(),
		field.String("direction").MaxLen(4).Default("desc"),
		// sort_order - position in the list; 0 is the primary sort key.
		field.Int("sort_order").SchemaType(mysqlType("int")).Default(0),
		// params - a module's criterion's settings in this View (its
		// threshold); empty for the others.
		field.JSON("params", map[string]string{}).Optional(),
		// exposed_key - the criterion offered to visitors: its value of the
		// page's "sort" parameter (?sort=<key>); empty - not offered.
		field.String("exposed_key").MaxLen(64).Default(""),
		// exposed_label - the option's label in the visitors' choice.
		field.String("exposed_label").MaxLen(255).Default(""),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Edges of the ViewSortDef.
func (ViewSortDef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("view_display", ViewDisplayDef.Type).Field("view_display_id").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_storage_id").Unique().Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

// Indexes of the ViewSortDef.
func (ViewSortDef) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("view_display_id", "sort_order").StorageKey("idx_view_sorts_view_sort"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("field_storage_id"),
	}
}
