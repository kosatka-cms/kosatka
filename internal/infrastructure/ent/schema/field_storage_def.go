package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// FieldStorageDef - a reusable field definition shared by every content
// type that attaches it via FieldInstanceDef. field_value_* rows reference
// this table's id, so a field's values live in one place.
type FieldStorageDef struct {
	ent.Schema
}

// Annotations of the FieldStorageDef.
func (FieldStorageDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("field_storages"),
	}
}

// Fields of the FieldStorageDef.
func (FieldStorageDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.String("machine_name").MaxLen(64).NotEmpty().Unique(),
		// text | number | date | bool | taxonomy | reference | image | file | list | group
		field.String("field_type").MaxLen(20).NotEmpty(),
		// Per-field-type settings shared by every content type using this field,
		// e.g. {"max_len":255} or {"vocabulary_id":12}.
		field.JSON("config", map[string]any{}).Optional(),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}
