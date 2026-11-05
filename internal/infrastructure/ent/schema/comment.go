package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Comment holds the schema definition for the Comment entity.
type Comment struct {
	ent.Schema
}

// Annotations of the Comment.
func (Comment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("comments"),
	}
}

// Fields of the Comment.
func (Comment) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.Int("nid").SchemaType(mysqlType("int unsigned")),
		field.Int("parent_id").SchemaType(mysqlType("int unsigned")).Optional().Nillable(),
		field.Int("created_by"),
		field.String("subject").MaxLen(255).Optional(),
		field.Text("content").SchemaType(mysqlType("mediumtext")).NotEmpty(),
		field.Time("created_at").Default(time.Now).Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		field.Int8("status").SchemaType(mysqlType("tinyint(1)")).Default(1),
		field.String("path").MaxLen(500).Optional().Nillable(),
		// hold_reason - why the comment waits for approval or is spam
		// (entity.CommentReason*), "" - none.
		field.String("hold_reason").MaxLen(255).Default(""),
	}
}

// Edges of the Comment.
func (Comment) Edges() []ent.Edge {
	return []ent.Edge{
		// Node the comment belongs to.
		edge.To("node", Node.Type).Field("nid").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),

		// Author.
		edge.To("author", User.Type).Field("created_by").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),

		// Parent comment.
		edge.To("parent", Comment.Type).Field("parent_id").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),

		// Child comments.
		edge.From("children", Comment.Type).Ref("parent"),
	}
}

// Indexes of the Comment.
func (Comment) Indexes() []ent.Index {
	return []ent.Index{
		// nid alone is covered by (nid, status, path).
		index.Fields("created_by"),
		index.Fields("parent_id"),
		index.Fields("created_at"),
		index.Fields("nid", "status", "path"),
		// The admin's tabs by status, the waiting ones' count.
		index.Fields("status", "created_at"),
	}
}
