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

// FieldValueText - a "text" field value of a node of a dynamic content
// type; the widget (entity.FieldWidget) doesn't affect storage. No edges:
// repositories read/write through *generated.Client directly.
type FieldValueText struct {
	ent.Schema
}

// Annotations of the FieldValueText.
func (FieldValueText) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_text"),
	}
}

// Fields of the FieldValueText.
func (FieldValueText) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Text("value").SchemaType(mysqlType("mediumtext")).Optional(),
		// text_format_id - the TextFormatDef the value was saved with (see
		// entity.TextFormatDef); set only for fields with allowed text formats.
		// NULL renders/edits the value without a text format.
		field.Int("text_format_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
	}
}

// Indexes of the FieldValueText.
func (FieldValueText) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("idx_field_value_text_field"),
		index.Fields("node_id", "field_id").Unique().StorageKey("uk_field_value_text"),
	}
}

// Edges of the FieldValueText.
func (FieldValueText) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
