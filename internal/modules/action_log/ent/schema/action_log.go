// Package schema - the action log module's own ent schema: the recorded
// actions and the module's settings.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// ActionLogEntry - an action done on the site (audit.Event): when, its
// type and group, who (actor_id - the user, NULL - nobody signed in;
// actor_name - their name at the time), from where (ip, "" - not kept),
// what it was done to (object_kind, object_id, object_label - its name
// at the time, object_type - its kind, a node's content type;
// object_url - where it is, "" - nowhere) and what it changed (changes -
// a JSON list of {field, from, to}).
type ActionLogEntry struct{ ent.Schema }

// Annotations of the ActionLogEntry.
func (ActionLogEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("action_log")}
}

// Fields of the ActionLogEntry.
func (ActionLogEntry) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")),
		field.String("type").MaxLen(64),
		field.String("action_group").MaxLen(64),
		field.Int("actor_id").SchemaType(schemakit.MySQLType("int unsigned")).Optional().Nillable(),
		field.String("actor_name").MaxLen(255).Default(""),
		field.String("ip").MaxLen(64).Default(""),
		field.String("object_kind").MaxLen(32).Default(""),
		field.String("object_id").MaxLen(64).Default(""),
		field.String("object_label").MaxLen(512).Default(""),
		field.String("object_type").MaxLen(255).Default(""),
		field.String("object_url").MaxLen(512).Default(""),
		field.Text("changes").SchemaType(schemakit.MySQLType("text")).Default(""),
	}
}

// Indexes of the ActionLogEntry.
func (ActionLogEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at").StorageKey("action_log_created_at"),
		index.Fields("action_group", "created_at").StorageKey("action_log_group_created_at"),
		index.Fields("actor_id", "created_at").StorageKey("action_log_actor_created_at"),
		index.Fields("object_kind", "object_id").StorageKey("action_log_object"),
	}
}

// ActionLogSettings - the module's settings (one row, id 1; none - the
// defaults): keep_days - how long entries are kept (0 - forever); log_ip -
// whether the client's address is kept; groups_off - the groups not
// recorded, by name, space-separated (a group added later is recorded).
type ActionLogSettings struct{ ent.Schema }

// Annotations of the ActionLogSettings.
func (ActionLogSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("action_log_settings")}
}

// Fields of the ActionLogSettings.
func (ActionLogSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("keep_days").SchemaType(schemakit.MySQLType("int")).Default(90),
		field.Bool("log_ip").Default(true),
		field.String("groups_off").MaxLen(1024).Default(""),
	}
}
