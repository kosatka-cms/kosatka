// Package schema - the polls module's own ent schema: its tables, apart
// from the core's. A poll's node and a vote's user are the core's: Node
// and User here are the module's stand-ins for their tables
// (migrations.Sets).
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

// Poll holds poll-specific fields; question text lives in nodes.title.
type Poll struct {
	ent.Schema
}

// Annotations of the Poll.
func (Poll) Annotations() []schema.Annotation {
	return []schema.Annotation{
		schemakit.Table("polls"),
	}
}

// Fields of the Poll.
func (Poll) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("nid").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Text("description").SchemaType(schemakit.MySQLType("text")).Optional(),
		field.Int("max_answers").SchemaType(schemakit.MySQLType("int")).Default(1),
		field.Time("available_from").SchemaType(schemakit.MySQLType("datetime")).Optional().Nillable(),
		field.Time("available_to").SchemaType(schemakit.MySQLType("datetime")).Optional().Nillable(),
		field.Bool("results_only").Default(false),
		field.Bool("allow_revote").Default(true),
	}
}

// Edges of the Poll.
func (Poll) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("nodes", Node.Type).Field("nid").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("options", PollOption.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("votes", PollVote.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the Poll.
func (Poll) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("nid").Unique().StorageKey("polls_nid"),
	}
}
