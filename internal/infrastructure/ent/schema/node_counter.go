package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// NodeCounter holds the schema definition for the NodeCounter entity.
type NodeCounter struct {
	ent.Schema
}

// Annotations of the NodeCounter.
func (NodeCounter) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("node_counter"),
	}
}

// Fields of the NodeCounter.
func (NodeCounter) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")),
		field.Int("nid").SchemaType(mysqlType("int unsigned")),
		field.Int("totalcount").SchemaType(mysqlType("bigint unsigned")).Annotations(entsql.Default("0")),
		field.Int("daycount").SchemaType(mysqlType("mediumint unsigned")).Annotations(entsql.Default("0")),
		field.Time("last_viewed").Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the NodeCounter.
func (NodeCounter) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("node", Node.Type).
			Ref("node_counter").
			Unique().
			Required().
			Field("nid"),
	}
}

// Indexes of the NodeCounter.
func (NodeCounter) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("totalcount").StorageKey("idx_node_counter_totalcount"),
	}
}
