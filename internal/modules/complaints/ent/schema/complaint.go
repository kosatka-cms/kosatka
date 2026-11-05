// Package schema - the complaints module's own ent schema: its tables,
// apart from the core's (internal/infrastructure/ent/schema). Users,
// content and comments are the core's, by id, without edges: the module
// reads them through the core (modulekit).
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// ComplaintTarget - a content item or a comment complained about: one
// row however many complain, the moderators' queue.
type ComplaintTarget struct {
	ent.Schema
}

// Annotations of the ComplaintTarget.
func (ComplaintTarget) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("complaint_targets")}
}

// Fields of the ComplaintTarget.
func (ComplaintTarget) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		// kind - "node" or "comment"; object_id - its id.
		field.String("kind").MaxLen(16),
		field.Int("object_id").SchemaType(schemakit.MySQLType("int unsigned")),
		// node_id - the content item (a comment's).
		field.Int("node_id").SchemaType(schemakit.MySQLType("int unsigned")),
		// author_id - who wrote it (0 - nobody known).
		field.Int("author_id").SchemaType(schemakit.MySQLType("int unsigned")).Default(0),
		// status - "new" (waits for a moderator) or "resolved".
		field.String("status").MaxLen(16).Default("new"),
		// hidden - the complaints hid it until a moderator decides.
		field.Bool("hidden").Default(false),
		// resolution - what the moderator did: kept, hidden, spam, deleted.
		field.String("resolution").MaxLen(16).Default(""),
		field.Int("resolved_by").SchemaType(schemakit.MySQLType("int unsigned")).Default(0),
		field.Time("resolved_at").SchemaType(schemakit.MySQLType("datetime")).Optional().Nillable(),
		field.Time("last_at").SchemaType(schemakit.MySQLType("datetime")),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")),
	}
}

// Edges of the ComplaintTarget.
func (ComplaintTarget) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("complaints", Complaint.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the ComplaintTarget.
func (ComplaintTarget) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("kind", "object_id").Unique().StorageKey("uniq_complaint_targets_kind_object"),
		index.Fields("status", "last_at").StorageKey("idx_complaint_targets_status_last"),
		index.Fields("node_id").StorageKey("idx_complaint_targets_node"),
	}
}

// Complaint - one visitor's complaint about a target: once per visitor.
type Complaint struct {
	ent.Schema
}

// Annotations of the Complaint.
func (Complaint) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("complaints")}
}

// Fields of the Complaint.
func (Complaint) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("target_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("user_id").SchemaType(schemakit.MySQLType("int unsigned")),
		// reason - the one chosen, as the settings named it then.
		field.String("reason").MaxLen(255),
		field.String("comment").MaxLen(1000).Default(""),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")),
	}
}

// Edges of the Complaint.
func (Complaint) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("target", ComplaintTarget.Type).Ref("complaints").Unique().Required().Field("target_id"),
	}
}

// Indexes of the Complaint.
func (Complaint) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("target_id", "user_id").Unique().StorageKey("uniq_complaints_target_user"),
		index.Fields("user_id").StorageKey("idx_complaints_user"),
	}
}

// ComplaintSettings - the module's settings (one row, id 1).
type ComplaintSettings struct {
	ent.Schema
}

// Annotations of the ComplaintSettings.
func (ComplaintSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("complaint_settings")}
}

// Fields of the ComplaintSettings.
func (ComplaintSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		// comments - whether comments may be reported.
		field.Bool("comments").Default(true),
		// node_types - the content types whose items may be reported.
		field.JSON("node_types", []string{}).Optional(),
		// reasons - the reasons offered, one per line ("" - the default
		// ones; "Other" with a comment is always there).
		field.Text("reasons").SchemaType(schemakit.MySQLType("text")).Optional(),
		// hide_after - complaints of different visitors that hide an item
		// until a moderator decides (0 - never).
		field.Int("hide_after").SchemaType(schemakit.MySQLType("int unsigned")).Default(3),
	}
}
