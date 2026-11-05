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

// PmThreadMember holds the schema definition for the PmThreadMember entity.
type PmThreadMember struct {
	ent.Schema
}

// Annotations of the PmThreadMember.
func (PmThreadMember) Annotations() []schema.Annotation {
	return []schema.Annotation{
		schemakit.Table("pm_thread_members"),
	}
}

// Fields of the PmThreadMember.
func (PmThreadMember) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("thread_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("user_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Bool("is_new").Default(true),
		field.Bool("deleted").Default(false),
		field.Time("joined_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the PmThreadMember.
func (PmThreadMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("thread", PmThread.Type).Field("thread_id").Ref("members").Unique().Required(),
		edge.To("user", User.Type).Field("user_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the PmThreadMember.
func (PmThreadMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "deleted", "is_new"),
		index.Fields("user_id", "deleted", "is_new", "thread_id"),
		index.Fields("thread_id", "deleted"),
		// A user is a thread's member once: its flags (read, deleted) are
		// one state, and the inbox and the member lists count each once.
		index.Fields("thread_id", "user_id").Unique().StorageKey("pm_thread_members_thread_user"),
	}
}
