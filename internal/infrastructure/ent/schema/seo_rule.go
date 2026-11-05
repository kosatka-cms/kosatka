package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SeoRule - An SEO title/description rule for a content type (optionally by terms of a field).
type SeoRule struct {
	ent.Schema
}

// Annotations of the SeoRule.
func (SeoRule) Annotations() []schema.Annotation {
	return []schema.Annotation{table("seo_rules")}
}

// Fields of the SeoRule.
func (SeoRule) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.String("content_type").MaxLen(64),
		field.Int("field_storage_id").SchemaType(mysqlType("int unsigned")).Optional().Nillable(),
		field.String("term_ids").MaxLen(1024).Default(""),
		field.String("title_template").MaxLen(512).Default(""),
		field.String("description_template").MaxLen(512).Default(""),
		field.Int("weight").SchemaType(mysqlType("int")).Default(0),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Indexes of the SeoRule.
func (SeoRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("weight").StorageKey("idx_seo_rules_weight"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("field_storage_id"),
	}
}

// Edges of the SeoRule.
func (SeoRule) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("field_storage", FieldStorageDef.Type).Field("field_storage_id").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
