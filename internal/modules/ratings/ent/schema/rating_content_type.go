package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// RatingContentType - a content type whose content is rated (no row -
// not rated), by its machine name: a module's type has no content_types
// row to reference.
type RatingContentType struct {
	ent.Schema
}

// Annotations of the RatingContentType.
func (RatingContentType) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("rating_content_types")}
}

// Fields of the RatingContentType.
func (RatingContentType) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("content_type").MaxLen(64).NotEmpty(),
	}
}

// Indexes of the RatingContentType.
func (RatingContentType) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_type").Unique(),
	}
}
