package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// BlockedIP - A blocked IP address (manual or automatic, optionally expiring).
type BlockedIP struct {
	ent.Schema
}

// Annotations of the BlockedIP.
func (BlockedIP) Annotations() []schema.Annotation {
	return []schema.Annotation{table("blocked_ips")}
}

// Fields of the BlockedIP.
func (BlockedIP) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")),
		field.String("ip").MaxLen(45),
		field.String("comment").MaxLen(255).Optional().Nillable(),
		field.Time("created_at").SchemaType(mysqlType("datetime")).Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Indexes of the BlockedIP.
func (BlockedIP) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ip").Unique().StorageKey("blocked_ips_ip"),
	}
}
