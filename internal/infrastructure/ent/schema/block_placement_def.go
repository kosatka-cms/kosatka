package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// BlockPlacementDef - where a block is shown: theme, region, order within
// the region and page visibility rules.
type BlockPlacementDef struct {
	ent.Schema
}

// Annotations of the BlockPlacementDef.
func (BlockPlacementDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("block_placements"),
	}
}

// Fields of the BlockPlacementDef.
func (BlockPlacementDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		// block_id - set only for placement_type="custom"; NULL for other types.
		field.Int("block_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// placement_type - selects which of block_id/plugin_id/view_display_id/
		// menu_id is used ("custom" | "plugin" | "view" | "menu").
		field.String("placement_type").MaxLen(16).Default("custom"),
		// plugin_id - one of entity.PluginBlocks()' machine IDs, set only for
		// placement_type="plugin". No FK: plugins are a fixed Go registry.
		field.String("plugin_id").MaxLen(32).Optional().Nillable(),
		// plugin_config - a plugin block's settings (entity.BlockPlacementDef.
		// PluginConfig), e.g. which form the forms module's block shows.
		field.JSON("plugin_config", map[string]string{}).Optional(),
		// view_display_id - set only for placement_type="view": a View's Block
		// display exposed as a placeable block.
		field.Int("view_display_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// menu_id - set only for placement_type="menu".
		field.Int("menu_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// menu_start_level - placement_type="menu" only: initial visibility level
		// (1-10, clamped by entity.ClampMenuLevelSettings before every write).
		field.Int("menu_start_level").SchemaType(mysqlType("int")).Default(1),
		// menu_levels_to_show - placement_type="menu" only: number of levels to
		// display (0-9; 0 = unlimited).
		field.Int("menu_levels_to_show").SchemaType(mysqlType("int")).Default(0),
		// title/show_title - the placement's own title, used by placement types
		// that have no title of their own (currently "plugin"; views and menus
		// render their own label).
		field.String("title").MaxLen(255).Optional(),
		field.Bool("show_title").Default(false),
		// theme - the machine name of the theme the placement is in: each
		// theme keeps its own blocks layout.
		field.String("theme").MaxLen(64).Default("default"),
		// region - one of the theme's regions (its theme.yaml, the engine's
		// by default); regions are tied to template locations.
		field.String("region").MaxLen(64).NotEmpty(),
		// weight - sort order within the region, set via the admin drag-and-drop
		// list.
		field.Int("weight").SchemaType(mysqlType("int")).Default(0),
		// visibility_scope/visibility_content_type - visibility rule. No FK on
		// visibility_content_type: built-in content types have no rows to reference.
		field.String("visibility_scope").MaxLen(16).Default("all"),
		field.String("visibility_content_type").MaxLen(255).Optional().Nillable(),
		// visibility_pages/visibility_pages_mode - path list (one pattern per line,
		// "*" wildcard, "<front>" = homepage); mode "show" (only on listed pages)
		// or "hide" (everywhere except listed pages). Used only when
		// visibility_scope == "pages".
		field.Text("visibility_pages").SchemaType(mysqlType("mediumtext")).Optional(),
		field.String("visibility_pages_mode").MaxLen(8).Default("show"),
		// visibility_roles - role machine names (comma-separated, "anonymous"
		// for guests) the placement is shown to, combined with the scope;
		// empty - everyone.
		field.String("visibility_roles").MaxLen(512).Default(""),
		// lazy - render a placeholder and load the content via
		// /api/block-placement/{id} when it scrolls near the viewport.
		field.Bool("lazy").Default(false),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Edges of the BlockPlacementDef.
func (BlockPlacementDef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("block", BlockDef.Type).Field("block_id").Unique().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("view_display", ViewDisplayDef.Type).Field("view_display_id").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("menu", MenuDef.Type).Field("menu_id").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the BlockPlacementDef.
func (BlockPlacementDef) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("theme", "region").StorageKey("idx_block_placements_theme_region"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("block_id"),
		index.Fields("menu_id"),
		index.Fields("view_display_id"),
	}
}
