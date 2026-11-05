package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// SearchSettings - Search settings (one row).
type SearchSettings struct {
	ent.Schema
}

// Annotations of the SearchSettings.
func (SearchSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("search_settings")}
}

// Fields of the SearchSettings.
func (SearchSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")).Annotations(notIncremental()), // the single settings row
		field.String("placeholder_image").MaxLen(255).Default(""),
		field.String("categories_menu").MaxLen(64).Default(""),
	}
}
