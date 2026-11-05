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

// FieldValueUser - values of a "user" field, one row per user in the
// editor's order (delta), whether the field holds one or several; a deleted
// user's rows go with it.
type FieldValueUser struct {
	ent.Schema
}

// Annotations of the FieldValueUser.
func (FieldValueUser) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_user"),
	}
}

// Fields of the FieldValueUser.
func (FieldValueUser) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("user_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("delta").SchemaType(mysqlType("int")).Default(0),
	}
}

// Indexes of the FieldValueUser.
func (FieldValueUser) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("idx_field_value_user_field"),
		index.Fields("user_id", "field_id").StorageKey("idx_field_value_user_user"),
		index.Fields("node_id", "field_id", "user_id").Unique().StorageKey("uk_field_value_user"),
	}
}

// Edges of the FieldValueUser.
func (FieldValueUser) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("user", User.Type).Field("user_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
