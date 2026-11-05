package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// NodeViewDay - a node's views of one day of the site (the popular
// content of a period: the report, the Views sorts by a week's or a
// month's views). Kept as long as statistics_settings.daily_views_keep_days
// says.
type NodeViewDay struct {
	ent.Schema
}

// Annotations of the NodeViewDay.
func (NodeViewDay) Annotations() []schema.Annotation {
	return []schema.Annotation{table("node_view_days")}
}

// Fields of the NodeViewDay.
func (NodeViewDay) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")),
		field.Int("nid").SchemaType(mysqlType("int unsigned")),
		// day - the site's day as YYYYMMDD (the site's time zone): a
		// number compares and ranges the same in both databases.
		field.Int("day").SchemaType(mysqlType("int unsigned")),
		field.Int("views").SchemaType(mysqlType("int unsigned")).Annotations(entsql.Default("0")),
	}
}

// Edges of the NodeViewDay.
func (NodeViewDay) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("node", Node.Type).
			Ref("view_days").
			Unique().
			Required().
			Field("nid"),
	}
}

// Indexes of the NodeViewDay.
func (NodeViewDay) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("nid", "day").Unique().StorageKey("uniq_node_view_days_nid_day"),
		index.Fields("day").StorageKey("idx_node_view_days_day"),
	}
}
