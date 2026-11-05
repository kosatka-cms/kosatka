package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TaxonomyTermData holds the schema definition for the TaxonomyTermData entity.
type TaxonomyTermData struct {
	ent.Schema
}

// Annotations of the TaxonomyTermData.
func (TaxonomyTermData) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("taxonomy_term_data"),
	}
}

// Fields of the TaxonomyTermData.
func (TaxonomyTermData) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.Int("vid").Annotations(entsql.Default("0")),
		field.String("name").SchemaType(mysqlType("varchar(255)")).MaxLen(100),
		field.Text("description").Optional(),
		// page_title - entity.TaxonomyTerm.PageTitle.
		field.String("page_title").MaxLen(255).Default(""),
		field.Int("weight").SchemaType(mysqlType("int")).Optional().Annotations(entsql.Default("0")),
		field.String("slug").MaxLen(255),
		field.Int("parent_id").SchemaType(mysqlType("int unsigned")).Optional().Nillable(), // NULL for top-level terms
		field.Int("created_by").SchemaType(mysqlType("int unsigned")).Optional(),
		field.Time("created_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		field.Time("updated_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")).UpdateDefault(time.Now),
	}
}

// Edges of the TaxonomyTermData.
func (TaxonomyTermData) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", TaxonomyTermData.Type).From("parent").Unique().Field("parent_id").Annotations(entsql.OnDelete(entsql.SetNull)),
		edge.To("creator", User.Type).Field("created_by").Unique().Annotations(entsql.OnDelete(entsql.SetNull)),
		edge.To("vocabulary", TaxonomyVocabulary.Type).Field("vid").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the TaxonomyTermData.
func (TaxonomyTermData) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("vid").StorageKey("idx_term_vid"),
		index.Fields("created_by").StorageKey("idx_term_created_by"),
		index.Fields("parent_id").StorageKey("idx_term_parent"),
		index.Fields("vid", "id").StorageKey("idx_term_vid_id"),
		index.Fields("vid", "name").StorageKey("idx_term_name_vid"),
		index.Fields("vid", "slug").Unique().StorageKey("ux_vid_slug"),
	}
}
