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

// FieldValueFileMulti - values of an "image" field with "multiple": true,
// one row per (node, field, file), in the editor's order (delta).
type FieldValueFileMulti struct {
	ent.Schema
}

// Annotations of the FieldValueFileMulti.
func (FieldValueFileMulti) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_file_multi"),
	}
}

// Fields of the FieldValueFileMulti.
func (FieldValueFileMulti) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("file_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("delta").SchemaType(mysqlType("int")).Default(0),
	}
}

// Indexes of the FieldValueFileMulti.
func (FieldValueFileMulti) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("idx_field_value_file_multi_field"),
		index.Fields("file_id").StorageKey("idx_field_value_file_multi_file"),
		index.Fields("node_id", "field_id", "file_id").Unique().StorageKey("uk_field_value_file_multi"),
	}
}

// Edges of the FieldValueFileMulti.
func (FieldValueFileMulti) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("file", File.Type).Field("file_id").Unique().Required().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
