// Package schema - the feeds module's own ent schema: its tables, apart
// from the core's.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// Feed - A configurable XML feed at its own path (feeds module).
type Feed struct {
	ent.Schema
}

// Annotations of the Feed.
func (Feed) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("feeds")}
}

// Fields of the Feed.
func (Feed) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("label").MaxLen(255),
		field.String("path").MaxLen(255),
		field.String("format").MaxLen(16).Default("rss"),
		field.Int("status").SchemaType(schemakit.MySQLType("tinyint")).Default(1),
		field.String("content_types").MaxLen(1024).Default(""),
		field.String("term_ids").MaxLen(1024).Default(""),
		field.Int("item_limit").SchemaType(schemakit.MySQLType("int")).Default(50),
		field.Int("days").SchemaType(schemakit.MySQLType("int")).Default(0),
		field.String("title").MaxLen(255).Default(""),
		field.String("description").MaxLen(255).Default(""),
		field.String("language").MaxLen(8).Default(""),
		field.String("image_style").MaxLen(64).Default(""),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")),
		field.Time("updated_at").SchemaType(schemakit.MySQLType("datetime")),
	}
}

// Indexes of the Feed.
func (Feed) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("path").Unique().StorageKey("feeds_path"),
	}
}
