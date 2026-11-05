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

// FieldValueListMulti - values of a FieldTypeList field with "multiple":
// true, one row per (node, field, value). Rows are unique on (node_id,
// field_id, value).
type FieldValueListMulti struct {
	ent.Schema
}

// Annotations of the FieldValueListMulti.
func (FieldValueListMulti) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_list_multi"),
	}
}

// Fields of the FieldValueListMulti.
func (FieldValueListMulti) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.String("value").MaxLen(255).NotEmpty(),
	}
}

// Indexes of the FieldValueListMulti.
func (FieldValueListMulti) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("idx_field_value_list_multi_field"),
		index.Fields("node_id", "field_id", "value").Unique().StorageKey("uk_field_value_list_multi"),
	}
}

// Edges of the FieldValueListMulti.
func (FieldValueListMulti) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
