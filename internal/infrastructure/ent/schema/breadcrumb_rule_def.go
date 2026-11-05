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

// BreadcrumbRuleDef - storage for entity.BreadcrumbRuleDef: content type +
// taxonomy field + terms mapped to a menu item.
type BreadcrumbRuleDef struct {
	ent.Schema
}

// Annotations of the BreadcrumbRuleDef.
func (BreadcrumbRuleDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("breadcrumb_rules"),
	}
}

// Fields of the BreadcrumbRuleDef.
func (BreadcrumbRuleDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.String("content_type").MaxLen(64).NotEmpty(),
		field.Int("field_storage_id").SchemaType(mysqlType("int unsigned")).Optional().Nillable(),
		field.String("term_ids").MaxLen(1024).Default(""),
		field.Int("menu_link_id").SchemaType(mysqlType("int unsigned")),
		field.Int("weight").SchemaType(mysqlType("int")).Default(0),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Indexes of the BreadcrumbRuleDef.
func (BreadcrumbRuleDef) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_storage_id").StorageKey("fk_breadcrumb_rules_field"),
		index.Fields("menu_link_id").StorageKey("fk_breadcrumb_rules_link"),
		index.Fields("weight").StorageKey("idx_breadcrumb_rules_weight"),
	}
}

// Edges of the BreadcrumbRuleDef.
func (BreadcrumbRuleDef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("menu_link", MenuLinkDef.Type).Field("menu_link_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_storage_id").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
