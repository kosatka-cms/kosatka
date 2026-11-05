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

// Node - the core's nodes: a gallery's node, a node that can have galleries.
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

// File - the core's files: a gallery item's image.
type File struct {
	ent.Schema
}

// Annotations of the File.
func (File) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("files")}
}

// Fields of the File.
func (File) Fields() []ent.Field {
	return []ent.Field{field.Int("id").SchemaType(schemakit.MySQLType("int unsigned"))}
}

// ContentTypeDef - the core's content types: a type's galleries settings.
type ContentTypeDef struct {
	ent.Schema
}

// Annotations of the ContentTypeDef.
func (ContentTypeDef) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("content_types")}
}

// Fields of the ContentTypeDef.
func (ContentTypeDef) Fields() []ent.Field {
	return []ent.Field{field.Int("id").SchemaType(schemakit.MySQLType("int unsigned"))}
}
