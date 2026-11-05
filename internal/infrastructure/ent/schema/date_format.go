package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DateFormat - A date display format of the date formats catalog.
type DateFormat struct {
	ent.Schema
}

// Annotations of the DateFormat.
func (DateFormat) Annotations() []schema.Annotation {
	return []schema.Annotation{table("date_formats")}
}

// Fields of the DateFormat.
func (DateFormat) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.String("machine_name").MaxLen(64),
		field.String("label").MaxLen(255),
		field.String("pattern").MaxLen(255),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Indexes of the DateFormat.
func (DateFormat) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("machine_name").Unique().StorageKey("date_formats_machine_name"),
	}
}
