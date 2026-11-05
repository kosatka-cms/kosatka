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

// PollOption holds the schema definition for the PollOption entity.
type PollOption struct {
	ent.Schema
}

// Annotations of the PollOption.
func (PollOption) Annotations() []schema.Annotation {
	return []schema.Annotation{
		schemakit.Table("poll_options"),
	}
}

// Fields of the PollOption.
func (PollOption) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("poll_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("label").MaxLen(500).NotEmpty(),
		field.Int("weight").SchemaType(schemakit.MySQLType("int")).Default(0),
	}
}

// Indexes of the PollOption.
func (PollOption) Indexes() []ent.Index {
	return []ent.Index{
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("poll_id"),
	}
}

// Edges of the PollOption.
func (PollOption) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("poll", Poll.Type).
			Ref("options").
			Field("poll_id").
			Unique().
			Required(),
		edge.To("option_votes", PollVote.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
