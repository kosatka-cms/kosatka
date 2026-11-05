package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// Module - A module or feature switch state; a name with no row is enabled.
type Module struct {
	ent.Schema
}

// Annotations of the Module.
func (Module) Annotations() []schema.Annotation {
	return []schema.Annotation{table("modules")}
}

// Fields of the Module.
func (Module) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").StorageKey("name").MaxLen(64).NotEmpty(),
		field.Bool("enabled").Default(true),
		field.Time("updated_at").SchemaType(mysqlType("datetime")).Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")).UpdateDefault(time.Now),
	}
}
