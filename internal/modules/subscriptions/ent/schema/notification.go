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

// SubscriptionNotification - something told to a user: kind "reply" (a
// reply to their comment), "comment" (a comment on content they
// follow), "content" (new content with a term they follow, term_id).
// read_at - when they opened it; mail_state - 0 waiting for the e-mail,
// 1 mailed, 2 not to be mailed.
type SubscriptionNotification struct{ ent.Schema }

// Annotations of the SubscriptionNotification.
func (SubscriptionNotification) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("subscription_notifications")}
}

// Fields of the SubscriptionNotification.
func (SubscriptionNotification) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("user_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("kind").MaxLen(16),
		field.Int("node_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("comment_id").SchemaType(schemakit.MySQLType("int unsigned")).Optional().Nillable(),
		field.Int("actor_id").SchemaType(schemakit.MySQLType("int unsigned")).Optional().Nillable(),
		field.Int("term_id").SchemaType(schemakit.MySQLType("int unsigned")).Optional().Nillable(),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")),
		field.Time("read_at").SchemaType(schemakit.MySQLType("datetime")).Optional().Nillable(),
		field.Int8("mail_state").Default(0),
	}
}

// Edges of the SubscriptionNotification.
func (SubscriptionNotification) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Field("user_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("comment", Comment.Type).Field("comment_id").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("actor", User.Type).Field("actor_id").Unique().Annotations(entsql.OnDelete(entsql.SetNull)),
		edge.To("term", Term.Type).Field("term_id").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the SubscriptionNotification.
func (SubscriptionNotification) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "read_at").StorageKey("idx_subscription_notifications_user"),
		index.Fields("mail_state", "user_id").StorageKey("idx_subscription_notifications_mail"),
		index.Fields("created_at").StorageKey("idx_subscription_notifications_created"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("actor_id"),
		index.Fields("comment_id"),
		index.Fields("node_id"),
		index.Fields("term_id"),
	}
}
