package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PageSeo holds the schema definition for the PageSeo entity.
type PageSeo struct {
	ent.Schema
}

// Annotations of the PageSeo.
func (PageSeo) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("page_seo"),
	}
}

// Fields of the PageSeo.
func (PageSeo) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.Int("nid").SchemaType(mysqlType("int unsigned")),
		field.String("title").MaxLen(200).Optional(),
		field.Text("description").SchemaType(mysqlType("text")).Optional(),
	}
}

// Edges of the PageSeo.
func (PageSeo) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("node", Node.Type).
			Ref("page_seo").
			Unique().
			Required().
			Field("nid"),
	}
}

// Indexes of the PageSeo.
func (PageSeo) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("nid").StorageKey("idx_page_seo_nid"),
	}
}
