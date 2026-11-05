package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// FieldValueTaxonomy - a "taxonomy" field value: a reference to
// taxonomy_term_data.term_id.
type FieldValueTaxonomy struct {
	ent.Schema
}

// Annotations of the FieldValueTaxonomy.
func (FieldValueTaxonomy) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_taxonomy"),
	}
}

// Fields of the FieldValueTaxonomy.
func (FieldValueTaxonomy) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("term_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
	}
}

// Indexes of the FieldValueTaxonomy.
func (FieldValueTaxonomy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("fk_field_value_taxonomy_field"),
		index.Fields("node_id", "field_id").Unique().StorageKey("uk_field_value_taxonomy"),
		index.Fields("term_id").StorageKey("idx_field_value_taxonomy_term"),
	}
}

// Edges of the FieldValueTaxonomy.
func (FieldValueTaxonomy) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("term", TaxonomyTermData.Type).Field("term_id").Unique().Required().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
