// Package schema - the hello module's own ent schema.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// Greeting - a greeting on the hello page.
type Greeting struct {
	ent.Schema
}

// Annotations of the Greeting.
func (Greeting) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("hello_greetings")}
}

// Fields of the Greeting.
func (Greeting) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int")),
		field.String("text").MaxLen(255),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")).Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}
