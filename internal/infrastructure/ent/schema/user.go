package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Annotations of the User.
func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("users"),
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.String("username").MaxLen(255),
		field.String("slug").MaxLen(255),
		field.String("avatar").MaxLen(255).Optional(),
		field.String("email").NotEmpty().MaxLen(255).Optional(),
		field.String("password_hash").NotEmpty().MaxLen(255),
		// timezone - the user's IANA time zone, used when per-user time zones are
		// enabled; empty means the site's time zone.
		field.String("timezone").MaxLen(64).Optional(),
		field.JSON("metadata", map[string]any{}).Optional(),
		field.Text("signature").SchemaType(mysqlType("mediumtext")).Optional(),
		field.Int("role_id").SchemaType(mysqlType("int")).Optional(),
		field.Int("status").SchemaType(mysqlType("int")).Default(1).Optional(), // 0=blocked, 1=active
		// Email verification.
		field.Bool("email_verified").Default(false).Optional(),
		// session_version - the account's sessions carry it; raising it
		// ends every one of them (a new password, two-factor sign-in
		// turned on or off).
		field.Int("session_version").SchemaType(mysqlType("int")).Default(0),
		// last_activity - last activity on the site (column accessed_at).
		field.Time("last_activity").Optional().StorageKey("accessed_at"),
		field.Time("created_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		field.Time("updated_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")).UpdateDefault(time.Now),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		// User's role.
		edge.To("role", Role.Type).Field("role_id").Unique(),
		edge.To("oauth_identities", UserOAuthIdentity.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("last_activity").StorageKey("idx_users_accessed_at"),
		// One account per address; stored in lower case
		// (entity.NormalizeEmail), no address is NULL.
		index.Fields("email").Unique().StorageKey("idx_users_email"),
		index.Fields("slug").Unique().StorageKey("alias"),
		index.Fields("username").Unique().StorageKey("username"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("role_id"),
	}
}
