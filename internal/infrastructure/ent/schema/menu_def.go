package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// MenuDef - a named menu holding a tree of MenuLinkDef rows. Placement in a
// region is a separate BlockPlacementDef (PlacementType == "menu").
type MenuDef struct {
	ent.Schema
}

// Annotations of the MenuDef.
func (MenuDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("menus"),
	}
}

// Fields of the MenuDef.
func (MenuDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.String("machine_name").MaxLen(64).NotEmpty().Unique(),
		field.String("label").MaxLen(255).NotEmpty(),
		field.Text("description").SchemaType(mysqlType("mediumtext")).Optional(),
		// in_sitemap - the menu's links go into the sitemap.
		field.Bool("in_sitemap").Default(false),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Edges of the MenuDef.
func (MenuDef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("links", MenuLinkDef.Type).Ref("menu"),
	}
}
