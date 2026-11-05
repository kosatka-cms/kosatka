package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// The core's tables the module's reference, as its stand-ins: their id
// only (of the core's column type), for the module's edges - the foreign
// keys (ON DELETE CASCADE). A table named as a core one isn't the
// module's: its set leaves it out (migrations.Sets) and the module reads
// them through the core, never through these.

// Node - the core's nodes: a vote's node.
type Node struct {
	ent.Schema
}

// Annotations of the Node.
func (Node) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("nodes")}
}

// Fields of the Node.
func (Node) Fields() []ent.Field {
	return []ent.Field{field.Int("id").SchemaType(schemakit.MySQLType("int unsigned"))}
}

// User - the core's users: a vote's voter (a deleted user's votes go).
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
