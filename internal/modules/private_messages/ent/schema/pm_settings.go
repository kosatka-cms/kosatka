package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// PmSettings - the module's settings, one row (id 1): the new message
// e-mail's template; empty - the default text in the site's language. No
// row until saved on the "Account settings" page.
type PmSettings struct {
	ent.Schema
}

// Annotations of the PmSettings.
func (PmSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("pm_settings")}
}

// Fields of the PmSettings.
func (PmSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Text("new_message_subject").SchemaType(schemakit.MySQLType("text")),
		field.Text("new_message_body").SchemaType(schemakit.MySQLType("text")),
	}
}
