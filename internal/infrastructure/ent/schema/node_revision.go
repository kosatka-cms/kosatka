package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// NodeRevision - one saved state of a node of a content type keeping its
// history (content_types.revisions): the node's own fields, its field
// values and SEO as JSON (entity.NodeSnapshot), who saved it and when.
// Deleting the node deletes its history.
type NodeRevision struct {
	ent.Schema
}

// Annotations of the NodeRevision.
func (NodeRevision) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("node_revisions"),
	}
}

// Fields of the NodeRevision.
func (NodeRevision) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Int("node_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		// user_id - who saved it; NULL for the state before the history
		// was on (entity.RevisionBaseline) or a deleted account.
		field.Int("user_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		// kind - entity.RevisionKind: create, edit, revert, baseline.
		field.String("kind").MaxLen(16),
		// reverted_from_at - when the revision a revert brought back was
		// saved (kept here: that one may be pruned since).
		field.Time("reverted_from_at").SchemaType(mysqlType("datetime")).Optional().Nillable(),
		// changed - the snapshot's keys that differ from the revision
		// before (entity.NodeSnapshot.Diff), as JSON.
		field.JSON("changed", []string{}).Optional(),
		field.Text("snapshot").SchemaType(mysqlType("mediumtext")),
	}
}

// Indexes of the NodeRevision.
func (NodeRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("node_id", "id").StorageKey("idx_node_revisions_node"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("user_id"),
	}
}

// Edges of the NodeRevision.
func (NodeRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("node_id").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("user", User.Type).Field("user_id").Unique().Annotations(entsql.OnDelete(entsql.SetNull)),
	}
}
