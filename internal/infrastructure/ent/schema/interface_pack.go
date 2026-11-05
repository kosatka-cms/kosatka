package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// InterfacePack - a language's interface translation downloaded from the
// translations catalog: its language, the catalog's version and checksum
// of the file, how many of its strings were kept, whether the catalog
// marks it a draft (machine translated), when it was installed. Its
// strings are rows of interface_strings (origin "pack").
type InterfacePack struct {
	ent.Schema
}

// Annotations of the InterfacePack.
func (InterfacePack) Annotations() []schema.Annotation {
	return []schema.Annotation{table("interface_packs")}
}

// Fields of the InterfacePack.
func (InterfacePack) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.String("lang").MaxLen(12).Unique(),
		field.String("version").MaxLen(32),
		field.String("sha256").SchemaType(fixedChar(64)),
		field.Int("strings").Default(0),
		field.Bool("draft").Default(false),
		field.Time("installed_at").SchemaType(mysqlType("datetime")),
	}
}
