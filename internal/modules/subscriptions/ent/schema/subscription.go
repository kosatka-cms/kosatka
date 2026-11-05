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

// Subscription - a user following a content item's comments (node_id)
// or a term's new content (term_id). Muted - they unsubscribed: the row
// stays so commenting doesn't subscribe them again. Token - the
// unsubscribe link of its e-mails.
type Subscription struct{ ent.Schema }

// Annotations of the Subscription.
func (Subscription) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("subscriptions")}
}

// Fields of the Subscription.
func (Subscription) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("user_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("node_id").SchemaType(schemakit.MySQLType("int unsigned")).Optional().Nillable(),
		field.Int("term_id").SchemaType(schemakit.MySQLType("int unsigned")).Optional().Nillable(),
		field.Bool("muted").Default(false),
		field.String("token").MaxLen(32),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")),
	}
}

// Edges of the Subscription.
func (Subscription) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Field("user_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("node", Node.Type).Field("node_id").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("term", Term.Type).Field("term_id").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the Subscription.
func (Subscription) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "node_id").Unique().StorageKey("uk_subscriptions_user_node"),
		index.Fields("user_id", "term_id").Unique().StorageKey("uk_subscriptions_user_term"),
		index.Fields("node_id", "muted").StorageKey("idx_subscriptions_node"),
		index.Fields("term_id", "muted").StorageKey("idx_subscriptions_term"),
		index.Fields("token").Unique().StorageKey("uk_subscriptions_token"),
	}
}
