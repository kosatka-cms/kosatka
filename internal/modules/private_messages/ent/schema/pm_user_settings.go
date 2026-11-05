package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// PmUserSettings - a user's settings of the module (their profile form):
// whether they get the new message e-mail. No row - the defaults (on).
type PmUserSettings struct {
	ent.Schema
}

// Annotations of the PmUserSettings.
func (PmUserSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("pm_user_settings")}
}

// Fields of the PmUserSettings.
func (PmUserSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("user_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Bool("mail_new_message").Default(true),
	}
}

// Edges of the PmUserSettings.
func (PmUserSettings) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Field("user_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the PmUserSettings.
func (PmUserSettings) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id").Unique()}
}
