package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// BlockDef - a custom block of admin-authored rich text, placed into a
// region via BlockPlacementDef.
type BlockDef struct {
	ent.Schema
}

// Annotations of the BlockDef.
func (BlockDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("blocks"),
	}
}

// Fields of the BlockDef.
func (BlockDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.String("title").MaxLen(255).NotEmpty(),
		// machine_name - entity.BlockDef.MachineName: what a text or a
		// template embeds the block by ([[block:<name>]]). NULL only on the
		// blocks from before it, till their data step names them.
		field.String("machine_name").MaxLen(64).Optional().Nillable().Unique(),
		// show_title - whether Title renders as an <h2>; Title itself is always
		// non-empty. See entity.BlockDef.ShowTitle.
		field.Bool("show_title").Default(true),
		// body - sanitized with htmlsanitize.SanitizeAdmin on save (admin-authored),
		// unless text_format_id opts the block into the Text Formats system.
		field.Text("body").Optional(),
		// text_format_id - see entity.BlockDef.TextFormatID; NULL keeps the
		// admin sanitizer behavior.
		field.Int("text_format_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		field.Bool("status").Default(true),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Edges of the BlockDef.
func (BlockDef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("placements", BlockPlacementDef.Type).Ref("block"),
	}
}
