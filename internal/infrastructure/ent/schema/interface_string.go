package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// InterfaceString - an interface string's translation the site keeps
// itself (pkg/i18n.Entry): one changed in the admin (origin "site", over
// the catalogs) or one of a downloaded language (origin "pack",
// InterfacePack); a language, a context and a source - the English
// string, a plural one's singular - and the text or the plural forms.
type InterfaceString struct {
	ent.Schema
}

// Annotations of the InterfaceString.
func (InterfaceString) Annotations() []schema.Annotation {
	return []schema.Annotation{table("interface_strings")}
}

// Fields of the InterfaceString.
func (InterfaceString) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.String("lang").MaxLen(12),
		// origin - whose translation it is: "site" - changed in the admin,
		// "pack" - a downloaded language's.
		field.String("origin").MaxLen(8).Default("site"),
		field.String("context").MaxLen(255).Default(""),
		field.Text("source").SchemaType(mysqlType("text")),
		// key_hash - the SHA-256 of context and source (hex): the source
		// is too long to index.
		field.String("key_hash").SchemaType(fixedChar(64)),
		field.Text("text").SchemaType(mysqlType("text")).Optional(),
		// forms - a plural string's forms, in the language's order.
		field.Strings("forms").Optional(),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Indexes of the InterfaceString.
func (InterfaceString) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("lang", "origin", "key_hash").Unique().StorageKey("uniq_interface_strings_key"),
	}
}
