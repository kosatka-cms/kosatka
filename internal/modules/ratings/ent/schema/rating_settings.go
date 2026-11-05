package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// RatingSettings - the ratings module's settings (one row).
type RatingSettings struct {
	ent.Schema
}

// Annotations of the RatingSettings.
func (RatingSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("rating_settings")}
}

// Fields of the RatingSettings.
func (RatingSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")).Annotations(schemakit.NotIncremental()), // the single settings row
		// scale - the stars of a vote, 3 to 10.
		field.Int("scale").SchemaType(schemakit.MySQLType("smallint")).Default(5),
		// allow_guests - whether guests may rate (by their session), besides
		// the users with the "rate content" permission.
		field.Bool("allow_guests").Default(false),
		// allow_change, allow_withdraw - whether a voter may change their
		// vote, take it back.
		field.Bool("allow_change").Default(true),
		field.Bool("allow_withdraw").Default(true),
	}
}
