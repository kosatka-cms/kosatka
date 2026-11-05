package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// TextTranslation - a text of the site (kind: a menu link's title…) of a
// thing (ref - its id) in a language besides the default one.
type TextTranslation struct{ ent.Schema }

// Annotations of the TextTranslation.
func (TextTranslation) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("text_translations")}
}

// Fields of the TextTranslation.
func (TextTranslation) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("kind").MaxLen(64),
		field.String("ref").MaxLen(64),
		field.String("lang").MaxLen(12),
		field.Text("value").SchemaType(schemakit.MySQLType("text")),
	}
}

// Indexes of the TextTranslation.
func (TextTranslation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("kind", "ref", "lang").Unique().StorageKey("text_translations_kind_ref_lang"),
	}
}

// LanguageVisibility - the languages whose pages show a thing (kind: a
// menu link…; ref - its id); none - every language's.
type LanguageVisibility struct{ ent.Schema }

// Annotations of the LanguageVisibility.
func (LanguageVisibility) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("language_visibility")}
}

// Fields of the LanguageVisibility.
func (LanguageVisibility) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("kind").MaxLen(64),
		field.String("ref").MaxLen(64),
		field.JSON("langs", []string{}),
	}
}

// Indexes of the LanguageVisibility.
func (LanguageVisibility) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("kind", "ref").Unique().StorageKey("language_visibility_kind_ref"),
	}
}
