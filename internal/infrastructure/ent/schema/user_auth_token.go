package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserAuthToken - A one-time token of a user (email confirmation, password reset, login link).
type UserAuthToken struct {
	ent.Schema
}

// Annotations of the UserAuthToken.
func (UserAuthToken) Annotations() []schema.Annotation {
	return []schema.Annotation{table("user_auth_tokens")}
}

// Fields of the UserAuthToken.
func (UserAuthToken) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("bigint unsigned")),
		field.Int("user_id").SchemaType(mysqlType("int unsigned")),
		field.String("token_hash").SchemaType(fixedChar(64)),
		field.String("kind").MaxLen(32),
		field.Time("expires_at").SchemaType(mysqlType("datetime")),
		field.Time("used_at").SchemaType(mysqlType("datetime")).Optional().Nillable(),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
	}
}

// Indexes of the UserAuthToken.
func (UserAuthToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token_hash", "kind").StorageKey("idx_hash_kind"),
		index.Fields("user_id", "kind").StorageKey("idx_user_kind"),
	}
}

// Edges of the UserAuthToken.
func (UserAuthToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Field("user_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
