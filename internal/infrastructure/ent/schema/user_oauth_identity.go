package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserOAuthIdentity holds OAuth provider links for users.
type UserOAuthIdentity struct {
	ent.Schema
}

// Annotations of the UserOAuthIdentity.
func (UserOAuthIdentity) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("user_oauth_identities"),
	}
}

// Fields of the UserOAuthIdentity.
func (UserOAuthIdentity) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(mysqlType("bigint unsigned")),
		field.Int("user_id"),
		field.String("provider").MaxLen(16),
		field.String("provider_user_id").MaxLen(128),
		field.String("email").MaxLen(255).Optional().Nillable(),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
	}
}

// Edges of the UserOAuthIdentity.
func (UserOAuthIdentity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("oauth_identities").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes of the UserOAuthIdentity.
func (UserOAuthIdentity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider", "provider_user_id").Unique(),
		index.Fields("user_id", "provider"),
	}
}
