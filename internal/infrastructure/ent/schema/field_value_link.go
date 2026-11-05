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

// FieldValueLink - values of a "link" field (entity.Link), one row per link
// in the editor's order (delta), whether the field holds one or several.
type FieldValueLink struct {
	ent.Schema
}

// Annotations of the FieldValueLink.
func (FieldValueLink) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_link"),
	}
}

// Fields of the FieldValueLink.
func (FieldValueLink) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("delta").SchemaType(mysqlType("int")).Default(0),
		field.String("url").MaxLen(2048),
		// title - "" when the field has none (entity.LinkTitleNone) or it
		// wasn't given.
		field.String("title").MaxLen(255).Default(""),
	}
}

// Indexes of the FieldValueLink.
func (FieldValueLink) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("idx_field_value_link_field"),
		index.Fields("node_id", "field_id", "delta").Unique().StorageKey("uk_field_value_link"),
	}
}

// Edges of the FieldValueLink.
func (FieldValueLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
