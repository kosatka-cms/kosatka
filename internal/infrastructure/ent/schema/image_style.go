package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// ImageStyle - admin-editable image resize presets. pkg/imagestyle resolves
// styles by Name at request time through an adapter (see
// internal/infrastructure/image_style_adapter.go).
type ImageStyle struct {
	ent.Schema
}

// Annotations of the ImageStyle.
func (ImageStyle) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("image_styles"),
	}
}

// Fields of the ImageStyle.
func (ImageStyle) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		// name is immutable once created - derivative URLs
		// (/uploads/styles/{name}/...) and Go call sites reference
		// styles by it.
		field.String("name").MaxLen(64).NotEmpty().Unique(),
		field.String("label").MaxLen(255).NotEmpty(),
		field.String("description").MaxLen(255).Optional(),
		field.Int("width").SchemaType(mysqlType("int")),
		// height=0 means "preserve aspect ratio" (see imagestyle.ImageStyle).
		field.Int("height").SchemaType(mysqlType("int")).Default(0),
		// fill | fit | resize - imagestyle.ResizeMode.
		field.String("mode").MaxLen(20).NotEmpty(),
		// One of imaging.Anchor's 9 values, lowercase/hyphenated (center,
		// top, top-left, top-right, left, right, bottom-left, bottom,
		// bottom-right) - see entity.ImageStyleDef's anchor mapping.
		field.String("anchor").MaxLen(20).Default("center"),
		field.Int("quality").SchemaType(mysqlType("int")).Default(85),
		// webp | jpeg - imagestyle.Format, the derivatives' encoding.
		field.String("format").MaxLen(8).Default("webp"),
		// hidpi - its derivatives are made at double size too, for
		// high-density screens (srcset 2x, imagestyle.ImageStyle.HiDPI).
		field.Bool("hidpi").Default(false),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}
