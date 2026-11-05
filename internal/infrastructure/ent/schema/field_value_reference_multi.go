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

// FieldValueReferenceMulti - values of a reference field with "multiple":
// true, one row per (node, field, target); delta keeps the editor's order.
type FieldValueReferenceMulti struct {
	ent.Schema
}

// Annotations of the FieldValueReferenceMulti.
func (FieldValueReferenceMulti) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_reference_multi"),
	}
}

// Fields of the FieldValueReferenceMulti.
func (FieldValueReferenceMulti) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("target_node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("delta").SchemaType(mysqlType("int")).Default(0),
	}
}

// Indexes of the FieldValueReferenceMulti.
func (FieldValueReferenceMulti) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("idx_field_value_reference_multi_field"),
		index.Fields("node_id", "field_id", "target_node_id").Unique().StorageKey("uk_field_value_reference_multi"),
		index.Fields("target_node_id", "field_id").StorageKey("idx_field_value_reference_multi_target"),
	}
}

// Edges of the FieldValueReferenceMulti.
func (FieldValueReferenceMulti) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("target_node", Node.Type).Field("target_node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
