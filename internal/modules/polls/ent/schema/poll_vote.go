package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// PollVote holds the schema definition for the PollVote entity.
type PollVote struct {
	ent.Schema
}

// Annotations of the PollVote.
func (PollVote) Annotations() []schema.Annotation {
	return []schema.Annotation{
		schemakit.Table("poll_votes"),
	}
}

// Fields of the PollVote.
func (PollVote) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("poll_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("user_id"),
		field.Int("option_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")).Default(time.Now),
	}
}

// Edges of the PollVote.
func (PollVote) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("poll", Poll.Type).
			Ref("votes").
			Field("poll_id").
			Unique().
			Required(),
		edge.From("users", User.Type).
			Ref("poll_votes").
			Field("user_id").
			Unique().
			Required(),
		edge.From("poll_option", PollOption.Type).
			Ref("option_votes").
			Field("option_id").
			Unique().
			Required(),
	}
}

// Indexes of the PollVote.
func (PollVote) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("poll_id", "user_id", "option_id").Unique(),
		index.Fields("poll_id", "user_id"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("option_id"),
		index.Fields("user_id"),
	}
}
