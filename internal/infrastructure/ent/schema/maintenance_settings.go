package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// MaintenanceSettings - Maintenance mode (one row).
type MaintenanceSettings struct {
	ent.Schema
}

// Annotations of the MaintenanceSettings.
func (MaintenanceSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("maintenance_settings")}
}

// Fields of the MaintenanceSettings.
func (MaintenanceSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")).Annotations(notIncremental(), entsql.Default("1")),
		field.Bool("enabled").Default(false),
		field.Text("message").SchemaType(mysqlType("text")),
	}
}
