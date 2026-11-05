package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TaxonomyIndex holds the schema definition for the TaxonomyIndex entity.
type TaxonomyIndex struct {
	ent.Schema
}

// Annotations of the TaxonomyIndex.
func (TaxonomyIndex) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("taxonomy_index"),
	}
}

// Fields of the TaxonomyIndex.
func (TaxonomyIndex) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")),
		field.Int("nid").SchemaType(mysqlType("int unsigned")),
		field.Int("tid").SchemaType(mysqlType("int unsigned")),
		field.Time("created_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the TaxonomyIndex.
func (TaxonomyIndex) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("nid").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("taxonomy_term_data", TaxonomyTermData.Type).Field("tid").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the TaxonomyIndex.
func (TaxonomyIndex) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("nid", "tid").Unique().StorageKey("ux_node_term"),
		index.Fields("tid", "nid").StorageKey("idx_taxonomy_index_tid_nid"),
	}
}
