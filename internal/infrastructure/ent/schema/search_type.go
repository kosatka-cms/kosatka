package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SearchType - how a node type takes part in the live search, or that
// it doesn't (enabled false).
type SearchType struct {
	ent.Schema
}

// Annotations of the SearchType.
func (SearchType) Annotations() []schema.Annotation {
	return []schema.Annotation{table("search_types")}
}

// Fields of the SearchType.
func (SearchType) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").StorageKey("node_type").MaxLen(64).NotEmpty(),
		field.Int("weight").SchemaType(mysqlType("int")).Default(0),
		field.Int("result_limit").SchemaType(mysqlType("int")).Default(5),
		field.String("label").MaxLen(64).Default(""),
		field.String("match_fields").MaxLen(255).Default(""),
		field.String("title_field").MaxLen(64).Default(""),
		field.String("subtitle_field").MaxLen(64).Default(""),
		field.String("image_field").MaxLen(64).Default(""),
		field.String("image_style").MaxLen(64).Default(""),
		field.Int("badge_vocabulary_id").SchemaType(mysqlType("int unsigned")).Optional().Nillable(),
		// enabled - false: the type is left out of the search (a type with
		// no row is searched by default).
		field.Bool("enabled").Default(true),
		// all_fields - all the type's text fields are searched, new ones
		// too (match_fields then only remembered for the form).
		field.Bool("all_fields").Default(false),
	}
}

// Indexes of the SearchType.
func (SearchType) Indexes() []ent.Index {
	return []ent.Index{
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("badge_vocabulary_id"),
	}
}

// Edges of the SearchType.
func (SearchType) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("badge_vocabulary", TaxonomyVocabulary.Type).Field("badge_vocabulary_id").Unique().Annotations(entsql.OnDelete(entsql.SetNull)),
	}
}
