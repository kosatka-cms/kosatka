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

// FieldValueDate - a "date" field value of a node of a dynamic content
// type.
type FieldValueDate struct {
	ent.Schema
}

// Annotations of the FieldValueDate.
func (FieldValueDate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_value_date"),
	}
}

// Fields of the FieldValueDate.
func (FieldValueDate) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("field_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		// value - the start: a date at UTC midnight, or (with_time) an
		// instant in UTC.
		field.Time("value").Optional().SchemaType(map[string]string{dialect.MySQL: "datetime"}),
		// end_value - the end of a range (entity.DateValue.End), the same
		// kind as value; NULL - none.
		field.Time("end_value").Optional().Nillable().SchemaType(map[string]string{dialect.MySQL: "datetime"}),
		// with_time - value and end_value have a time (entity.DateValue.WithTime).
		field.Bool("with_time").Default(false),
		// delta - the value's place among a field's several (the editor's
		// order); 0 for one.
		field.Int("delta").SchemaType(mysqlType("int")).Default(0),
	}
}

// Indexes of the FieldValueDate.
func (FieldValueDate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("field_id").StorageKey("idx_field_value_date_field"),
		index.Fields("node_id", "field_id", "delta").Unique().StorageKey("uk_field_value_date_delta"),
	}
}

// Edges of the FieldValueDate.
func (FieldValueDate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("field_storage", FieldStorageDef.Type).Field("field_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
