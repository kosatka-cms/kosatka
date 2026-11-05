// Package schema - the error reports module's own ent schema: its
// tables, apart from the core's (internal/infrastructure/ent/schema).
// A reporter is a core user by user_id, without an edge: the module reads
// users through the core (modulekit.UserRepository).
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// ErrorReport - A visitor's report of an error in the text (error reports module).
type ErrorReport struct {
	ent.Schema
}

// Annotations of the ErrorReport.
func (ErrorReport) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("error_reports")}
}

// Fields of the ErrorReport.
func (ErrorReport) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int")),
		field.Int("user_id").SchemaType(schemakit.MySQLType("int unsigned")).Optional().Nillable(),
		field.String("page_url").MaxLen(2048),
		field.Text("selected_text").SchemaType(schemakit.MySQLType("mediumtext")),
		field.Text("comment").SchemaType(schemakit.MySQLType("text")).Optional().Nillable(),
		field.String("client_ip").MaxLen(45).Optional().Nillable(),
		field.String("user_agent").MaxLen(512).Optional().Nillable(),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")).Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
		// resolved_at - when an editor marked the report done; NULL - new.
		field.Time("resolved_at").SchemaType(schemakit.MySQLType("datetime")).Optional().Nillable(),
	}
}

// Indexes of the ErrorReport.
func (ErrorReport) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at").Annotations(entsql.Desc()).StorageKey("error_reports_created_at"),
		index.Fields("user_id").StorageKey("error_reports_user_id"),
		index.Fields("resolved_at").StorageKey("error_reports_resolved_at"),
	}
}
