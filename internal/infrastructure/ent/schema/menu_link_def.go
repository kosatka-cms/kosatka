package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MenuLinkDef - one item in a MenuDef's tree (arbitrary depth via
// parent_id).
type MenuLinkDef struct {
	ent.Schema
}

// Annotations of the MenuLinkDef.
func (MenuLinkDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("menu_links"),
	}
}

// Fields of the MenuLinkDef.
func (MenuLinkDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("menu_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		// parent_id - self-reference; nil means a top-level item.
		field.Int("parent_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		field.String("title").MaxLen(255).NotEmpty(),
		// link_type - selects which of link_url/link_node_id/link_term_id/
		// link_view_id is used (see entity.MenuLinkType).
		field.String("link_type").MaxLen(16).Default("url"),
		// link_url - link_type == "url" only: an internal path (starting with "/")
		// or a full external URL, stored as typed.
		field.String("link_url").MaxLen(2048).Optional(),
		// link_node_id/link_term_id/link_view_id - link_type "node"/"term"/"view"
		// only: the target's ID, resolved to its current URL at render time (see
		// service.DefaultMenuService.ResolveMenuLinkURL). No FK: a deleted target
		// renders as a broken link, not a DB error.
		field.Int("link_node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		field.Int("link_term_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		field.Int("link_view_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// weight - sort order among siblings, set via the admin drag-and-drop tree.
		field.Int("weight").SchemaType(mysqlType("int")).Default(0),
		field.Bool("enabled").Default(true),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Indexes of the MenuLinkDef.
func (MenuLinkDef) Indexes() []ent.Index {
	return []ent.Index{
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("menu_id"),
		index.Fields("parent_id"),
	}
}

// Edges of the MenuLinkDef.
func (MenuLinkDef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("menu", MenuDef.Type).Field("menu_id").Required().Unique(),
		edge.To("parent", MenuLinkDef.Type).Field("parent_id").Unique(),
		edge.From("children", MenuLinkDef.Type).Ref("parent"),
	}
}
