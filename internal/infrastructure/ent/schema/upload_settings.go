package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// UploadSettings - The sitewide upload policy (one row).
type UploadSettings struct {
	ent.Schema
}

// Annotations of the UploadSettings.
func (UploadSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("upload_settings")}
}

// Fields of the UploadSettings.
func (UploadSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")).Annotations(notIncremental()), // the single settings row
		field.Int("max_file_size_bytes").SchemaType(mysqlType("int")),
		field.Int("max_image_dimension").SchemaType(mysqlType("int")),
		field.String("allowed_mime_types").MaxLen(255),
		field.String("user_file_extensions").MaxLen(500).Default("pdf doc docx xls xlsx ppt pptx odt ods odp txt rtf csv zip"),
	}
}
