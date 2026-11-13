package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// ContentTypeDef - an admin-created content type.
type ContentTypeDef struct {
	ent.Schema
}

// Annotations of the ContentTypeDef.
func (ContentTypeDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("content_types"),
	}
}

// Fields of the ContentTypeDef.
func (ContentTypeDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.String("machine_name").MaxLen(64).NotEmpty().Unique(),
		field.String("label").MaxLen(255).NotEmpty(),
		// path_pattern - entity.ContentTypeDef.PathPattern ("" - the default).
		field.String("path_pattern").MaxLen(255).Default(""),
		field.String("description").MaxLen(255).Optional(),
		field.Int("status").SchemaType(mysqlType("tinyint")).Default(1), // 1=enabled (creatable), 0=disabled
		// default_comment_mode: "open" | "closed" | "hidden" (entity.CommentMode),
		// copied onto new nodes of this type; changing it doesn't affect existing nodes.
		field.String("default_comment_mode").MaxLen(10).Default("open"),
		// hide_empty_comments - entity.ContentTypeDef.HideEmptyComments.
		// default_promote / default_sticky - a new node's Promote and Sticky
		// (entity.ContentTypeDef.DefaultPromote/DefaultSticky).
		field.Bool("default_promote").Default(false),
		field.Bool("default_sticky").Default(false),
		// default_publishing - a new node's publishing: "published" |
		// "moderation" | "unpublished"
		// (entity.ContentTypeDef.DefaultPublishing).
		field.String("default_publishing").MaxLen(16).Default("moderation"),
		field.Bool("hide_empty_comments").Default(false),
		// show_submitted / show_view_count - node detail page display
		// settings, see entity.ContentTypeDef.
		// redirect_field - entity.ContentTypeDef.RedirectField.
		field.String("redirect_field").MaxLen(64).Default(""),
		field.Bool("show_submitted").Default(true),
		field.Bool("show_view_count").Default(false),
		// revisions / revisions_keep - entity.ContentTypeDef.Revisions,
		// RevisionsKeep.
		field.Bool("revisions").Default(false),
		field.Int("revisions_keep").SchemaType(mysqlType("int")).Default(50),
		// no_preview - entity.ContentTypeDef.NoPreview: the type's form
		// without Preview (on unless turned off).
		field.Bool("no_preview").Default(false),
		// embedded - entity.ContentTypeDef.Embedded: a field set, its
		// nodes items of blocks fields. Set on creating, never changed.
		field.Bool("embedded").Default(false),
		// view_roles - entity.ContentTypeDef.ViewRoles: the role names who
		// may view the type's content, comma-separated; "" - everyone.
		field.String("view_roles").MaxLen(1024).Default(""),
		field.Int("created_by").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}
