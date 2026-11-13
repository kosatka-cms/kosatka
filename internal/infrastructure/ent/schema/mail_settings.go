package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// MailSettings - Mail settings (one row): transport, sender, secrets.
type MailSettings struct {
	ent.Schema
}

// Annotations of the MailSettings.
func (MailSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("mail_settings")}
}

// Fields of the MailSettings.
func (MailSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("tinyint unsigned")).Annotations(notIncremental()), // the single settings row
		field.Bool("enabled").Default(true),
		field.String("transport").MaxLen(16).Default(""),
		field.String("from_address").MaxLen(255).Default(""),
		// smtp_password - sealed with the site's key
		// (internal/lib/secretbox) when it has one.
		field.String("smtp_host").MaxLen(255).Default(""),
		field.Int("smtp_port").SchemaType(mysqlType("int")).Default(0),
		field.String("smtp_user").MaxLen(255).Default(""),
		field.String("smtp_password").MaxLen(512).Default(""),
		// log_keep_days - how many days the mail log keeps an email (0 -
		// no log).
		field.Int("log_keep_days").SchemaType(mysqlType("int unsigned")).Default(30),
	}
}
