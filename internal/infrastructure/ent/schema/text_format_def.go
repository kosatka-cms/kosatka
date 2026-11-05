package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// TextFormatDef - an admin-editable text format: an HTML tag allowlist plus
// a TinyMCE toolbar config, chosen per value for FieldTypeText safe
// HTML/WYSIWYG content (see entity.TextFormatDef).
type TextFormatDef struct {
	ent.Schema
}

// Annotations of the TextFormatDef.
func (TextFormatDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("text_formats"),
	}
}

// Fields of the TextFormatDef.
func (TextFormatDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.String("machine_name").MaxLen(64).NotEmpty().Unique(),
		field.String("label").MaxLen(255).NotEmpty(),
		field.Int("weight").SchemaType(mysqlType("int")).Default(0),
		// allowed_tags - space-separated HTML tag names allowed by this format (see
		// pkg/htmlsanitize.SanitizeWithFormat). Empty means plain text: all tags
		// stripped.
		field.Text("allowed_tags").SchemaType(mysqlType("mediumtext")).Optional(),
		field.Bool("allow_class_attribute").Default(false),
		// has_rich_editor - whether TinyMCE is attached for this format (see
		// entity.TextFormatDef.HasRichEditor).
		field.Bool("has_rich_editor").Default(true),
		field.Text("tinymce_plugins").SchemaType(mysqlType("mediumtext")).Optional(),
		field.Text("tinymce_toolbar").SchemaType(mysqlType("mediumtext")).Optional(),
		// allowed_extensions/max_file_size_bytes - upload policy for the TinyMCE
		// file manager, overriding the sitewide UploadSettings. Empty/0 means not
		// set - see entity.ResolveTextFormatUploadPolicy.
		field.String("allowed_extensions").MaxLen(255).Default(""),
		field.Int("max_file_size_bytes").SchemaType(mysqlType("int")).Default(0),
		field.Int("status").SchemaType(mysqlType("int")).Default(1),
		// filters - the modules' text filters on for the format, by name,
		// space-separated (see entity.TextFormatDef.Filters).
		field.String("filters").MaxLen(255).Default(""),
		field.Int("created_by").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional(),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}
