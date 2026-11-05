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

// FieldValueReference - a "reference" field value: a link to another node
// (target_node_id).
type FieldValueReference struct {
	ent.Schema
}

// Annotations of the FieldValueReference.
func (FieldValueReference) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_reference"),
	}
}

// Fields of the FieldValueReference.
func (FieldValueReference) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("target_node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
	}
}

// Indexes of the FieldValueReference.
func (FieldValueReference) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("fk_field_value_reference_field"),
		index.Fields("node_id", "field_id").Unique().StorageKey("uk_field_value_reference"),
		index.Fields("target_node_id").StorageKey("idx_field_value_reference_target"),
	}
}

// Edges of the FieldValueReference.
func (FieldValueReference) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("target_node", Node.Type).Field("target_node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
