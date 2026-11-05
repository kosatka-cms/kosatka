package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TaxonomyVocabulary holds the schema definition for the TaxonomyVocabulary entity.
type TaxonomyVocabulary struct {
	ent.Schema
}

// Annotations of the TaxonomyVocabulary.
func (TaxonomyVocabulary) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("taxonomy_vocabulary"),
	}
}

// Fields of the TaxonomyVocabulary.
func (TaxonomyVocabulary) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.String("name").SchemaType(mysqlType("varchar(255)")).MaxLen(100),
		field.Text("description").Optional(),
		field.Int("weight").SchemaType(mysqlType("int")).Optional().Annotations(entsql.Default("0")),
		field.String("slug").MaxLen(255).Unique(),
		// term_path_prefix - entity.TaxonomyVocabulary.TermPathPrefix.
		field.String("term_path_prefix").MaxLen(255).Default(""),
		// public_term_pages - entity.TaxonomyVocabulary.PublicTermPages.
		field.Bool("public_term_pages").Default(false),
		field.Int("created_by").SchemaType(mysqlType("int unsigned")).Optional(),
		field.Time("created_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		field.Time("updated_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")).UpdateDefault(time.Now),
	}
}

// Indexes of the TaxonomyVocabulary.
func (TaxonomyVocabulary) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_by").StorageKey("idx_vocab_created_by"),
	}
}

// Edges of the TaxonomyVocabulary.
func (TaxonomyVocabulary) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("creator", User.Type).Field("created_by").Unique().Annotations(entsql.OnDelete(entsql.SetNull)),
	}
}
