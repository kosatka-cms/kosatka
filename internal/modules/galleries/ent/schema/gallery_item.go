package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// GalleryItem holds the schema definition for the GalleryItem entity.
type GalleryItem struct {
	ent.Schema
}

// Annotations of the GalleryItem.
func (GalleryItem) Annotations() []schema.Annotation {
	return []schema.Annotation{
		schemakit.Table("gallery_items"),
	}
}

// Fields of the GalleryItem.
func (GalleryItem) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("gallery_id").SchemaType(schemakit.MySQLType("int unsigned")).Comment("The gallery (galleries.id)"),
		field.Int("fid").SchemaType(schemakit.MySQLType("int unsigned")).Comment("The image file (files.id)"),
		field.String("title").
			MaxLen(255).
			Optional().
			Comment("The image's title"),
		field.Text("description").SchemaType(schemakit.MySQLType("text")).
			Optional().
			Comment("The image's description"),
		field.Int("weight").SchemaType(schemakit.MySQLType("int")).
			Default(0).
			Optional().
			Comment("The image's position in the gallery"),
		field.Bool("is_cover").
			Default(false).
			Comment("Whether the image is the gallery's cover"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the GalleryItem.
func (GalleryItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("gallery", Gallery.Type).
			Field("gallery_id").
			Required().
			Unique().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("file", File.Type).
			Field("fid").
			Required().
			Unique().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the GalleryItem.
func (GalleryItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("gallery_id", "fid").Unique(),
		index.Fields("gallery_id", "weight"),
		index.Fields("gallery_id", "is_cover"),
		index.Fields("fid"),
	}
}
