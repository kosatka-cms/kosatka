package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// AccountSettings - Account settings (one row): registration, verification, notification emails, avatars.
type AccountSettings struct {
	ent.Schema
}

// Annotations of the AccountSettings.
func (AccountSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("account_settings")}
}

// Fields of the AccountSettings.
func (AccountSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")).Annotations(notIncremental(), entsql.Default("1")),
		field.String("registration_mode").MaxLen(32).Default("visitors"),
		field.Bool("require_email_verification").Default(false),
		field.Bool("user_timezones").Default(false),
		field.String("subj_welcome_admin").MaxLen(512).Default(""),
		field.Text("body_welcome_admin").SchemaType(mysqlType("mediumtext")),
		field.String("subj_welcome_pending").MaxLen(512).Default(""),
		field.Text("body_welcome_pending").SchemaType(mysqlType("mediumtext")),
		field.String("subj_welcome_open").MaxLen(512).Default(""),
		field.Text("body_welcome_open").SchemaType(mysqlType("mediumtext")),
		field.String("subj_activation").MaxLen(512).Default(""),
		field.Text("body_activation").SchemaType(mysqlType("mediumtext")),
		field.String("subj_password_reset").MaxLen(512).Default(""),
		field.Text("body_password_reset").SchemaType(mysqlType("mediumtext")),
		field.String("subj_email_confirm").MaxLen(512).Default(""),
		field.Text("body_email_confirm").SchemaType(mysqlType("mediumtext")),
		field.String("subj_admin_notify").MaxLen(512).Default(""),
		field.Text("body_admin_notify").SchemaType(mysqlType("mediumtext")),
		field.Text("registration_blocked_domains").SchemaType(mysqlType("text")),
		field.String("avatar_allowed_extensions").MaxLen(255).Default(""),
		field.Int("avatar_max_file_size_bytes").SchemaType(mysqlType("int")).Default(0),
		// two_factor_roles - the roles (ids) whose users must use two-factor
		// sign-in; two_factor_issuer - the site's name in the authenticator
		// ("" - the site's name).
		field.JSON("two_factor_roles", []int{}).Optional(),
		field.String("two_factor_issuer").MaxLen(100).Default(""),
		// cancel_content - what a user deleting their own account leaves
		// (entity.AccountSettings.CancelContent): keep - their content
		// without an author, delete - their content too.
		field.String("cancel_content").MaxLen(16).Default("keep"),
	}
}
