package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MailTransportSetting - a setting of a way of sending email that a
// module adds (service.MailTransport): the transport's name, the
// setting's name and its value, sealed with the site's key
// (internal/lib/secretbox) when it has one - secret or not, a value is
// read only through the repository.
type MailTransportSetting struct {
	ent.Schema
}

// Annotations of the MailTransportSetting.
func (MailTransportSetting) Annotations() []schema.Annotation {
	return []schema.Annotation{table("mail_transport_settings")}
}

// Fields of the MailTransportSetting.
func (MailTransportSetting) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.String("transport").MaxLen(16),
		field.String("name").MaxLen(64),
		field.Text("value").SchemaType(mysqlType("text")),
	}
}

// Indexes of the MailTransportSetting.
func (MailTransportSetting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("transport", "name").Unique().StorageKey("uniq_mail_transport_settings_name"),
	}
}
