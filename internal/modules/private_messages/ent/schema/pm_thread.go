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

// PmThread holds the schema definition for the PmThread entity.
type PmThread struct {
	ent.Schema
}

// Annotations of the PmThread.
func (PmThread) Annotations() []schema.Annotation {
	return []schema.Annotation{
		schemakit.Table("pm_threads"),
	}
}

// Fields of the PmThread.
func (PmThread) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("subject").MaxLen(255).Default(""),
		field.Time("created_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		field.Time("updated_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")).UpdateDefault(time.Now),
	}
}

// Edges of the PmThread.
func (PmThread) Edges() []ent.Edge {
	return []ent.Edge{
		// Thread messages.
		edge.To("messages", PmMessage.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		// Thread members.
		edge.To("members", PmThreadMember.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the PmThread.
func (PmThread) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("updated_at"),
		index.Fields("created_at"),
	}
}
