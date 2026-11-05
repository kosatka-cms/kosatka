// Package schema - the languages module's own ent schema: which content
// items translate which, and the module's settings.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// NodeTranslation - a content item of a set of translations: nid - the
// item, tnid - the set (its original's nid; the original's own row has
// tnid = nid, and the set outlives the original's deletion); outdated -
// the original changed after the translation was last saved.
type NodeTranslation struct{ ent.Schema }

// Annotations of the NodeTranslation.
func (NodeTranslation) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("node_translations")}
}

// Fields of the NodeTranslation.
func (NodeTranslation) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("nid").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("tnid").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Bool("outdated").Default(false),
	}
}

// Edges of the NodeTranslation.
func (NodeTranslation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("nid").Required().Unique().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the NodeTranslation.
func (NodeTranslation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("nid").Unique().StorageKey("node_translations_nid"),
		index.Fields("tnid").StorageKey("node_translations_tnid"),
	}
}

// LanguageSettings - the module's settings (one row, id 1; none - the
// defaults): translatable_types - the content types whose items are
// translated (their "Translations" tab); untranslated - what a list in a
// language shows of the items without a version in it ("original" -
// their original, "hide" - nothing, "all" - lists aren't narrowed).
type LanguageSettings struct{ ent.Schema }

// Annotations of the LanguageSettings.
func (LanguageSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("language_settings")}
}

// Fields of the LanguageSettings.
func (LanguageSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.JSON("translatable_types", []string{}).Optional(),
		field.String("untranslated").MaxLen(16).Default("original"),
	}
}

// Node - the core's nodes (their id only): a translation's item, deleted
// with it (ON DELETE CASCADE). The module reads them through the core.
type Node struct{ ent.Schema }

// Annotations of the Node.
func (Node) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("nodes")}
}

// Fields of the Node.
func (Node) Fields() []ent.Field {
	return []ent.Field{field.Int("id").SchemaType(schemakit.MySQLType("int unsigned"))}
}
