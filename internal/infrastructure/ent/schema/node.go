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

// Node holds the schema definition for the Node entity.
type Node struct {
	ent.Schema
}

// Annotations of the Node.
func (Node) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("nodes"),
	}
}

// Fields of the Node.
func (Node) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.String("type").MaxLen(100),
		field.String("title").MaxLen(255),
		field.String("slug").MaxLen(255).Unique().Optional(),
		field.Int("status").SchemaType(mysqlType("tinyint(1)")).Default(1).Annotations(entsql.Default("0")),
		field.Int("on_moderation").SchemaType(mysqlType("tinyint(1)")).Default(0),
		// comment_mode: "open" | "closed" | "hidden" (entity.CommentMode) - copied
		// from the content type's default_comment_mode at creation, independent
		// afterwards. Defaults to "open" for types without a ContentTypeDef.
		field.String("comment_mode").MaxLen(10).Default("open"),
		// path_auto - entity.Node.PathAuto: the slug follows the type's path
		// pattern on every save. Off for the nodes there before patterns.
		field.Bool("path_auto").Default(false),
		// promote / sticky - entity.Node.Promote ("on the front page") and
		// Sticky ("at the top of lists"): Views filter and
		// sort by them; a new node takes its type's defaults.
		field.Bool("promote").Default(false),
		field.Bool("sticky").Default(false),
		// view_roles - entity.Node.ViewRoles: the role names who may view
		// the item, of those its type is for, comma-separated; "" - whoever
		// may view its type.
		field.String("view_roles").MaxLen(1024).Default(""),
		// publish_on / unpublish_on - entity.Node.PublishOn, UnpublishOn:
		// the scheduled publishing; NULL - none.
		field.Time("publish_on").SchemaType(mysqlType("datetime")).Optional().Nillable(),
		field.Time("unpublish_on").SchemaType(mysqlType("datetime")).Optional().Nillable(),
		// parent_id - entity.Node.ParentID: an item of a blocks field, the
		// node holding it (deleted with it); NULL - content of its own.
		field.Int("parent_id").SchemaType(mysqlType("int unsigned")).Optional().Nillable(),
		// deleted_at / deleted_by - entity.Node.DeletedAt, DeletedBy: the
		// node is in the trash since then, put there by that user; NULL -
		// not in the trash.
		field.Time("deleted_at").SchemaType(mysqlType("datetime")).Optional().Nillable(),
		field.Int("deleted_by").Optional().Nillable(),
		// langcode - the language of the content (entity.Node.Langcode): a
		// new one gets the request's (Hooks), the ones from before the site's.
		field.String("langcode").MaxLen(12).Default(""),
		field.Int("created_by").Optional(),
		field.Time("created_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		field.Time("updated_at").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")).UpdateDefault(time.Now),
	}
}

// Edges of the Node.
func (Node) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("users", User.Type).Field("created_by").Unique(),
		edge.From("taxonomy_indices", TaxonomyIndex.Type).Ref("node"),
		edge.To("node_counter", NodeCounter.Type).Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("view_days", NodeViewDay.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		// search_words - the node's words for the search, gone with it.
		edge.To("search_words", SearchWord.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("page_seo", PageSeo.Type).Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		// preview_links - the node's links for viewing it unpublished
		// (entity.PreviewLink), gone with it.
		edge.To("preview_links", PreviewLink.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		// A blocks field's items (parent_id): deleted with their node.
		edge.To("block_items", Node.Type).Annotations(entsql.OnDelete(entsql.Cascade)).From("parent").Field("parent_id").Unique(),
	}
}

// Indexes of the Node.
func (Node) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "on_moderation", "created_at", "id").StorageKey("idx_nodes_status_moderation_created"),
		index.Fields("status", "on_moderation", "type").StorageKey("idx_nodes_status_mod_type"),
		index.Fields("type", "status", "created_at").StorageKey("idx_nodes_type_status_created"),
		// The front page: the promoted ones, the sticky first, the newest.
		index.Fields("promote", "status", "sticky", "created_at").StorageKey("idx_nodes_promote_status_sticky_created"),
		index.Fields("publish_on").StorageKey("idx_nodes_publish_on"),
		index.Fields("unpublish_on").StorageKey("idx_nodes_unpublish_on"),
		index.Fields("parent_id").StorageKey("idx_nodes_parent"),
		index.Fields("deleted_at").StorageKey("idx_nodes_deleted_at"),
		index.Fields("langcode").StorageKey("idx_nodes_langcode"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("created_by"),
	}
}
