package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserField - An admin-defined profile field.
type UserField struct {
	ent.Schema
}

// Annotations of the UserField.
func (UserField) Annotations() []schema.Annotation {
	return []schema.Annotation{table("user_fields")}
}

// Fields of the UserField.
func (UserField) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.String("machine_name").MaxLen(64),
		field.String("label").MaxLen(255),
		field.Int("max_len").SchemaType(mysqlType("int")).Default(255),
		// required - asked at sign-up and never left empty.
		field.Bool("required").Default(false),
		field.Int("weight").SchemaType(mysqlType("int")).Default(0),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Indexes of the UserField.
func (UserField) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("machine_name").Unique().StorageKey("user_fields_machine_name"),
	}
}
