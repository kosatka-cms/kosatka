package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// FeedFieldMap - Which fields of a content type fill a feed's items (feeds module).
type FeedFieldMap struct {
	ent.Schema
}

// Annotations of the FeedFieldMap.
func (FeedFieldMap) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("feed_field_maps")}
}

// Fields of the FeedFieldMap.
func (FeedFieldMap) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("feed_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("content_type").MaxLen(64),
		field.String("body_field").MaxLen(64).Default(""),
		field.String("image_field").MaxLen(64).Default(""),
		field.String("category_field").MaxLen(64).Default(""),
	}
}

// Indexes of the FeedFieldMap.
func (FeedFieldMap) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("feed_id", "content_type").Unique().StorageKey("feed_field_maps_feed_type"),
	}
}
