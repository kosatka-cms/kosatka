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

// FieldValueBlock - values of a "blocks" field: one row per item (a node of
// a field set, its parent the node holding the field) in the editor's order
// (delta); a deleted item's row goes with it.
type FieldValueBlock struct {
	ent.Schema
}

// Annotations of the FieldValueBlock.
func (FieldValueBlock) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_block"),
	}
}

// Fields of the FieldValueBlock.
func (FieldValueBlock) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("item_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("delta").SchemaType(mysqlType("int")).Default(0),
	}
}

// Indexes of the FieldValueBlock.
func (FieldValueBlock) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("idx_field_value_block_field"),
		index.Fields("node_id", "field_id", "delta").StorageKey("idx_field_value_block_node"),
		// An item is in one place.
		index.Fields("item_id").Unique().StorageKey("uk_field_value_block_item"),
	}
}

// Edges of the FieldValueBlock.
func (FieldValueBlock) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("item", Node.Type).Field("item_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
