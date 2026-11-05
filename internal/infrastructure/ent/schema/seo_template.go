package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// SeoTemplate - SEO templates of a page scope.
type SeoTemplate struct {
	ent.Schema
}

// Annotations of the SeoTemplate.
func (SeoTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{table("seo_templates")}
}

// Fields of the SeoTemplate.
func (SeoTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").StorageKey("scope").MaxLen(80).NotEmpty(),
		field.String("title_template").MaxLen(512).Default(""),
		field.String("description_template").MaxLen(512).Default(""),
		field.String("image_template").MaxLen(512).Default(""),
		// schema_type, schema_props - node scopes: the pages' schema.org
		// type and its properties' templates (entity.SEOTemplate).
		field.String("schema_type").MaxLen(64).Default(""),
		field.JSON("schema_props", map[string]string{}).Optional(),
	}
}
