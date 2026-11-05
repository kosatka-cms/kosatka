// Package schema - the galleries module's own ent schema: its tables,
// apart from the core's. A gallery's node, an item's file and a type's
// settings' content type are the core's: Node, File and ContentTypeDef
// here are the module's stand-ins for their tables (migrations.Sets).
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

// The image styles a gallery's previews use until it chooses others (the
// module's install step creates them).
const (
	DefaultPreviewSmallStyle  = "gallery_thumbnail"
	DefaultPreviewMediumStyle = "gallery_medium"
)

// Gallery holds the schema definition for the Gallery entity.
type Gallery struct {
	ent.Schema
}

// Annotations of the Gallery.
func (Gallery) Annotations() []schema.Annotation {
	return []schema.Annotation{
		schemakit.Table("galleries"),
	}
}

// Fields of the Gallery.
func (Gallery) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("nid").SchemaType(schemakit.MySQLType("int unsigned")).Comment("The node (nodes.id) the gallery belongs to"),
		// preview_small_style / preview_medium_style - the image styles of
		// the gallery's grid previews and of its larger previews
		// (image_styles.name).
		field.String("preview_small_style").MaxLen(64).Default(DefaultPreviewSmallStyle),
		field.String("preview_medium_style").MaxLen(64).Default(DefaultPreviewMediumStyle),
		field.String("title").
			MaxLen(255).
			Optional().
			Comment("The gallery's title"),
		field.Text("description").SchemaType(schemakit.MySQLType("text")).
			Optional().
			Comment("The gallery's description"),
		field.Int("weight").SchemaType(schemakit.MySQLType("int")).
			Default(0).
			Optional().
			Comment("The gallery's position among the node's galleries"),
		field.Bool("status").
			Default(true).
			Optional().
			Comment("1 - shown, 0 - hidden"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the Gallery.
func (Gallery) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).
			Field("nid").
			Required().
			Unique().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.From("items", GalleryItem.Type).
			Ref("gallery"),
	}
}

// Indexes of the Gallery.
func (Gallery) Indexes() []ent.Index {
	return []ent.Index{
		// nid alone is covered by (nid, status, weight).
		index.Fields("status"),
		index.Fields("nid", "status", "weight"),
	}
}
