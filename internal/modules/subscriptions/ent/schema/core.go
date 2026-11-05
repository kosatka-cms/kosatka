// Package schema - the subscriptions module's own ent schema: its tables,
// apart from the core's. Their users, content, comments and terms are the
// core's: the stand-ins here are their tables (migrations.Sets), for the
// foreign keys only.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// The core's tables the module's reference, as stand-ins: their id only
// (of the core's column type). A table named as a core one isn't the
// module's: its set leaves it out (migrations.Sets) and the module reads
// them through the core, never through these.

// User - the core's users: a subscriber, a notification's recipient (ON
// DELETE CASCADE) and its actor (SET NULL).
type User struct{ ent.Schema }

// Annotations of the User.
func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("users")}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{field.Int("id").SchemaType(schemakit.MySQLType("int unsigned"))}
}

// Node - the core's content: what is subscribed to and told of (ON
// DELETE CASCADE).
type Node struct{ ent.Schema }

// Annotations of the Node.
func (Node) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("nodes")}
}

// Fields of the Node.
func (Node) Fields() []ent.Field {
	return []ent.Field{field.Int("id").SchemaType(schemakit.MySQLType("int unsigned"))}
}

// Comment - the core's comments: the one a notification tells of (ON
// DELETE CASCADE).
type Comment struct{ ent.Schema }

// Annotations of the Comment.
func (Comment) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("comments")}
}

// Fields of the Comment.
func (Comment) Fields() []ent.Field {
	return []ent.Field{field.Int("id").SchemaType(schemakit.MySQLType("int unsigned"))}
}

// Term - the core's taxonomy terms: one followed (ON DELETE CASCADE).
type Term struct{ ent.Schema }

// Annotations of the Term.
func (Term) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("taxonomy_term_data")}
}

// Fields of the Term.
func (Term) Fields() []ent.Field {
	return []ent.Field{field.Int("id").SchemaType(schemakit.MySQLType("int unsigned"))}
}
