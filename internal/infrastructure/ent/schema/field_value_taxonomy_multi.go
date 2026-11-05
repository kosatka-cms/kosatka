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

// FieldValueTaxonomyMulti - values of a taxonomy field with "multiple":
// true, one row per (node, field, term). Rows are unique on (node_id,
// field_id, term_id).
type FieldValueTaxonomyMulti struct {
	ent.Schema
}

// Annotations of the FieldValueTaxonomyMulti.
func (FieldValueTaxonomyMulti) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_taxonomy_multi"),
	}
}

// Fields of the FieldValueTaxonomyMulti.
func (FieldValueTaxonomyMulti) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("term_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
	}
}

// Indexes of the FieldValueTaxonomyMulti.
func (FieldValueTaxonomyMulti) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("idx_field_value_taxonomy_multi_field"),
		index.Fields("node_id", "field_id", "term_id").Unique().StorageKey("uk_field_value_taxonomy_multi"),
		index.Fields("term_id").StorageKey("idx_field_value_taxonomy_multi_term"),
	}
}

// Edges of the FieldValueTaxonomyMulti.
func (FieldValueTaxonomyMulti) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("term", TaxonomyTermData.Type).Field("term_id").Unique().Required().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
