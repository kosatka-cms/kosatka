package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Role holds the schema definition for the Role entity.
type Role struct {
	ent.Schema
}

// Annotations of the Role.
func (Role) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("roles"),
	}
}

// Fields of the Role.
func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")),
		field.String("name").MaxLen(50).Unique().NotEmpty(),
		field.String("description").MaxLen(255).Optional(),
		field.JSON("permissions", map[string]any{}).Optional(),
		field.Time("created_at").SchemaType(mysqlType("datetime")).Default(time.Now).Immutable(),
		field.Time("updated_at").SchemaType(mysqlType("datetime")).Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Role.
func (Role) Edges() []ent.Edge {
	return []ent.Edge{
		// Users with this role.
		edge.From("users", User.Type).Ref("role"),
	}
}

// Indexes of the Role.
func (Role) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Unique(),
	}
}
