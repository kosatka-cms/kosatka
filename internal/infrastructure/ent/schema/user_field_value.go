package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserFieldValue - A user's value of a user field.
type UserFieldValue struct {
	ent.Schema
}

// Annotations of the UserFieldValue.
func (UserFieldValue) Annotations() []schema.Annotation {
	return []schema.Annotation{table("user_field_values")}
}

// Fields of the UserFieldValue.
func (UserFieldValue) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.Int("user_id").SchemaType(mysqlType("int unsigned")),
		field.Int("field_id").SchemaType(mysqlType("int unsigned")),
		field.Text("value").SchemaType(mysqlType("text")),
	}
}

// Indexes of the UserFieldValue.
func (UserFieldValue) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "field_id").Unique().StorageKey("user_field_values_user_field"),
		index.Fields("field_id").StorageKey("user_field_values_field"),
	}
}
