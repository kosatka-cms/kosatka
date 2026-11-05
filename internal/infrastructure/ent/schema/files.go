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

// File holds the schema definition for the File entity.
type File struct {
	ent.Schema
}

// Annotations of the File.
func (File) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("files"),
	}
}

// Fields of the File.
func (File) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.String("uri").MaxLen(255),
		field.String("mime_type").MaxLen(50).Optional(),
		field.Int("width").SchemaType(mysqlType("int")).Optional(),
		field.Int("height").SchemaType(mysqlType("int")).Optional(),
		field.Int("size_bytes").Optional(),
		field.String("alt").MaxLen(255).Optional(),
		// original_filename - set only by FieldTypeFile uploads (see
		// entity.File.OriginalFilename); empty otherwise.
		field.String("original_filename").MaxLen(255).Optional(),
		// focal_x/focal_y - the image's focal point, in percent of its
		// width and height from the top left (none - no point): a style
		// cropping at it keeps it in the picture (entity.FocalPoint).
		field.Int("focal_x").SchemaType(mysqlType("smallint")).Optional().Nillable(),
		field.Int("focal_y").SchemaType(mysqlType("smallint")).Optional().Nillable(),
		field.Int("created_by").Optional(),
		field.Time("created_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		field.Time("updated_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")).UpdateDefault(time.Now),
	}
}

// Edges of the File.
func (File) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("users", User.Type).Field("created_by").Unique(),
	}
}

// Indexes of the File.
func (File) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("uri").StorageKey("idx_images_uri"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("created_by"),
	}
}
