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

// GalleryNode - a node that can have galleries (the galleries module's
// switch of a node: no row - off).
type GalleryNode struct {
	ent.Schema
}

// Annotations of the GalleryNode.
func (GalleryNode) Annotations() []schema.Annotation {
	return []schema.Annotation{
		schemakit.Table("gallery_nodes"),
	}
}

// Fields of the GalleryNode.
func (GalleryNode) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("nid").SchemaType(schemakit.MySQLType("int unsigned")),
	}
}

// Edges of the GalleryNode.
func (GalleryNode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).
			Field("nid").
			Required().
			Unique().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the GalleryNode.
func (GalleryNode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("nid").Unique(),
	}
}
