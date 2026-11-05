package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PreviewLink - a link showing a content item (published or not) to
// whoever has it, until it expires or is revoked (entity.PreviewLink).
type PreviewLink struct {
	ent.Schema
}

// Annotations of the PreviewLink.
func (PreviewLink) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("preview_links"),
	}
}

// Fields of the PreviewLink.
func (PreviewLink) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id"),
		field.Int("nid").SchemaType(mysqlType("int unsigned")),
		// token_hash - the link's token's SHA-256, hex: the token itself
		// is shown once, when the link is made.
		field.String("token_hash").MaxLen(64).Unique(),
		field.Time("expires_at").SchemaType(mysqlType("datetime")),
		field.Int("created_by").Optional(),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
	}
}

// Edges of the PreviewLink.
func (PreviewLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("node", Node.Type).
			Ref("preview_links").
			Unique().
			Required().
			Field("nid"),
	}
}

// Indexes of the PreviewLink.
func (PreviewLink) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("nid").StorageKey("idx_preview_links_nid"),
	}
}
