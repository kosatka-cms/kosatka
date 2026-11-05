// Package schema - the private messages module's own ent schema: its
// tables (pm_threads, pm_thread_members, pm_messages), apart from the
// core's. A message's author and a member are the core's users: User here
// is the module's stand-in for their table (migrations.Sets).
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// The core's table the module's reference, as its stand-in: its id only
// (of the core's column type), for the module's edges - the foreign keys.
// A table named as a core one isn't the module's: its set leaves it out
// (migrations.Sets) and the module reads users through the core, never
// through this.

// User - the core's users: a message's author (ON DELETE SET NULL), a
// thread's member and a user's settings (ON DELETE CASCADE).
type User struct {
	ent.Schema
}

// Annotations of the User.
func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("users")}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{field.Int("id").SchemaType(schemakit.MySQLType("int unsigned"))}
}
