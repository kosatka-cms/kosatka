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

// GalleryContentType - the galleries module's settings of a content type
// (no row - the defaults).
type GalleryContentType struct {
	ent.Schema
}

// Annotations of the GalleryContentType.
func (GalleryContentType) Annotations() []schema.Annotation {
	return []schema.Annotation{
		schemakit.Table("gallery_content_types"),
	}
}

// Fields of the GalleryContentType.
func (GalleryContentType) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("content_type_id").SchemaType(schemakit.MySQLType("int unsigned")),
		// default_enabled - whether a new node of the type can have
		// galleries (the node's own switch starts from it).
		field.Bool("default_enabled").Default(false),
		// picture_field - the type's image field a gallery image can be set
		// as ("" - none).
		field.String("picture_field").MaxLen(64).Default(""),
	}
}

// Edges of the GalleryContentType.
func (GalleryContentType) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("content_type", ContentTypeDef.Type).
			Field("content_type_id").
			Required().
			Unique().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the GalleryContentType.
func (GalleryContentType) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_type_id").Unique(),
	}
}
