package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// StatisticsSettings - Statistics settings (one row).
type StatisticsSettings struct {
	ent.Schema
}

// Annotations of the StatisticsSettings.
func (StatisticsSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("statistics_settings")}
}

// Fields of the StatisticsSettings.
func (StatisticsSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")).Annotations(notIncremental(), entsql.Default("1")),
		field.Bool("view_counting_enabled").Default(true),
		// daily_views_keep_days - how many days node_view_days keeps (the
		// popular content of a period); 0 - for ever.
		field.Int("daily_views_keep_days").SchemaType(mysqlType("int unsigned")).Default(90),
	}
}
