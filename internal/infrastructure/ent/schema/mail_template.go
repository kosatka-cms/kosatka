package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MailTemplate - what the Mail page changed of one of the site's emails
// (entity.MailDef, by its key): whether it is sent, and the text of an
// email that keeps it nowhere else (a blank subject or body - the
// email's default). A row only for an email changed.
type MailTemplate struct {
	ent.Schema
}

// Annotations of the MailTemplate.
func (MailTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{table("mail_templates")}
}

// Fields of the MailTemplate.
func (MailTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.String("mail_key").MaxLen(128),
		field.Bool("enabled").Default(true),
		field.String("subject").MaxLen(512).Default(""),
		field.Text("body").SchemaType(mysqlType("mediumtext")),
	}
}

// Indexes of the MailTemplate.
func (MailTemplate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("mail_key").Unique().StorageKey("uniq_mail_templates_mail_key"),
	}
}
