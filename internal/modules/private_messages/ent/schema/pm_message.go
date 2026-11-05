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

// PmMessage holds the schema definition for the PmMessage entity.
type PmMessage struct {
	ent.Schema
}

// Annotations of the PmMessage.
func (PmMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		schemakit.Table("pm_messages"),
	}
}

// Fields of the PmMessage.
func (PmMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("thread_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("author_id").SchemaType(schemakit.MySQLType("int unsigned")).Optional(), // NULL: the author was deleted
		field.Text("body"),
		field.Time("created_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the PmMessage.
func (PmMessage) Edges() []ent.Edge {
	return []ent.Edge{
		// Thread the message belongs to.
		edge.From("thread", PmThread.Type).Field("thread_id").Ref("messages").Unique().Required(),
		// Author.
		edge.To("author", User.Type).Field("author_id").Unique().Annotations(entsql.OnDelete(entsql.SetNull)),
	}
}

// Indexes of the PmMessage.
func (PmMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("thread_id", "created_at"),
		index.Fields("thread_id", "id"),
		index.Fields("author_id", "created_at"),
	}
}
