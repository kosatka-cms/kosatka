package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserTwoFactor - a user's two-factor sign-in: the authenticator's secret
// (base32), when it was turned on, the last time step a code was accepted
// for (a code isn't accepted twice). A row - on; none - off.
type UserTwoFactor struct {
	ent.Schema
}

// Annotations of the UserTwoFactor.
func (UserTwoFactor) Annotations() []schema.Annotation {
	return []schema.Annotation{table("user_two_factor")}
}

// Fields of the UserTwoFactor.
func (UserTwoFactor) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.Int("user_id").SchemaType(mysqlType("int unsigned")),
		// secret - sealed with the site's key (internal/lib/secretbox) when it has one.
		field.String("secret").MaxLen(255),
		field.Int64("last_step").Default(0),
		field.Time("enabled_at").SchemaType(mysqlType("datetime")),
	}
}

// Indexes of the UserTwoFactor.
func (UserTwoFactor) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique().StorageKey("idx_user_two_factor_user"),
	}
}

// Edges of the UserTwoFactor.
func (UserTwoFactor) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Field("user_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// UserRecoveryCode - a one-time recovery code of a user's two-factor
// sign-in: its SHA-256 (hex), when it was used (nil - not yet).
type UserRecoveryCode struct {
	ent.Schema
}

// Annotations of the UserRecoveryCode.
func (UserRecoveryCode) Annotations() []schema.Annotation {
	return []schema.Annotation{table("user_recovery_codes")}
}

// Fields of the UserRecoveryCode.
func (UserRecoveryCode) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.Int("user_id").SchemaType(mysqlType("int unsigned")),
		field.String("code_hash").SchemaType(fixedChar(64)),
		field.Time("used_at").SchemaType(mysqlType("datetime")).Optional().Nillable(),
	}
}

// Indexes of the UserRecoveryCode.
func (UserRecoveryCode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "code_hash").StorageKey("idx_user_recovery_codes_user"),
	}
}

// Edges of the UserRecoveryCode.
func (UserRecoveryCode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Field("user_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
