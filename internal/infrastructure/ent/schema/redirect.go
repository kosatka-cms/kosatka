package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Redirect - A redirect from an old path.
type Redirect struct {
	ent.Schema
}

// Annotations of the Redirect.
func (Redirect) Annotations() []schema.Annotation {
	return []schema.Annotation{table("redirects")}
}

// Fields of the Redirect.
func (Redirect) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")),
		field.String("from_path").MaxLen(500),
		field.String("to_path").MaxLen(500),
	}
}

// Indexes of the Redirect.
func (Redirect) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("from_path").Unique().StorageKey("redirects_from_path"),
	}
}
