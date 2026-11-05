package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// ThemeSettings - a theme's setting values (entity.ThemeSetting), a row
// per theme that has any set.
type ThemeSettings struct {
	ent.Schema
}

// Annotations of the ThemeSettings.
func (ThemeSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("theme_settings")}
}

// Fields of the ThemeSettings.
func (ThemeSettings) Fields() []ent.Field {
	return []ent.Field{
		// theme - the theme's placement key (entity.ThemeKey).
		field.String("theme").MaxLen(64).Unique().NotEmpty(),
		// data - setting ID → value.
		field.JSON("data", map[string]string{}).Optional(),
		field.Time("updated_at").SchemaType(mysqlType("datetime")).Default(time.Now).UpdateDefault(time.Now),
	}
}
