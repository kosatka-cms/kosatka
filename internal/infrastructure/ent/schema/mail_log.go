package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MailLog - an email the site sent or tried to (the Mail page's log):
// when, which of the site's emails (its key; "" - a message of no
// template, a form's notice), to whom, its subject and what came of it.
// Not its text. Kept as long as mail_settings.log_keep_days says.
type MailLog struct {
	ent.Schema
}

// Annotations of the MailLog.
func (MailLog) Annotations() []schema.Annotation {
	return []schema.Annotation{table("mail_log")}
}

// Fields of the MailLog.
func (MailLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("bigint unsigned")),
		field.Time("sent_at").SchemaType(mysqlType("datetime")),
		field.String("mail_key").MaxLen(128).Default(""),
		field.String("recipient").MaxLen(255),
		field.String("subject").MaxLen(512).Default(""),
		// status - entity.MailStatus*: sent, failed, unsent (email off).
		field.String("status").MaxLen(16),
		field.String("error").MaxLen(1024).Default(""),
	}
}

// Indexes of the MailLog.
func (MailLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sent_at").StorageKey("idx_mail_log_sent_at"),
		index.Fields("mail_key", "sent_at").StorageKey("idx_mail_log_mail_key_sent_at"),
	}
}
