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

// SubscriptionUserSettings - a user's settings of the module (their
// profile form): mail - "now", "daily" or "never"; replies - told of
// replies to their comments; auto_own / auto_commented - subscribed to
// their content, to content they comment on. No row - the defaults.
type SubscriptionUserSettings struct{ ent.Schema }

// Annotations of the SubscriptionUserSettings.
func (SubscriptionUserSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("subscription_user_settings")}
}

// Fields of the SubscriptionUserSettings.
func (SubscriptionUserSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("user_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("mail").MaxLen(8).Default("now"),
		field.Bool("replies").Default(true),
		field.Bool("auto_own").Default(true),
		field.Bool("auto_commented").Default(true),
	}
}

// Edges of the SubscriptionUserSettings.
func (SubscriptionUserSettings) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Field("user_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the SubscriptionUserSettings.
func (SubscriptionUserSettings) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id").Unique().StorageKey("uk_subscription_user_settings_user")}
}

// SubscriptionSettings - the module's settings (one row, id 1): the
// e-mail's template (blank - the default) and the day the last digest
// went out.
type SubscriptionSettings struct{ ent.Schema }

// Annotations of the SubscriptionSettings.
func (SubscriptionSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("subscription_settings")}
}

// Fields of the SubscriptionSettings.
func (SubscriptionSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("mail_subject").MaxLen(255).Default(""),
		field.Text("mail_body").SchemaType(schemakit.MySQLType("text")).Optional(),
		field.String("digest_day").MaxLen(10).Default(""),
	}
}
