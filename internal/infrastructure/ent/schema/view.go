package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// View - a container of ViewDisplayDef rows: one Default display (never
// rendered; the shared baseline, see ViewDisplayDef.IsDefault) plus
// Page/Block displays that inherit or override each section from it. Label
// is admin-facing only. Created by views.DefaultService.CloneDisplay.
type View struct {
	ent.Schema
}

// Annotations of the View.
func (View) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("views"),
	}
}

// Fields of the View.
func (View) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.String("machine_name").MaxLen(64).NotEmpty().Unique(),
		field.String("label").MaxLen(255).NotEmpty(),
		field.Int("created_by").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Edges of the View.
func (View) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("displays", ViewDisplayDef.Type).Ref("view"),
	}
}
