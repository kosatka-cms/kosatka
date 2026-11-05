package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// BlockedBot - A blocked user-agent pattern.
type BlockedBot struct {
	ent.Schema
}

// Annotations of the BlockedBot.
func (BlockedBot) Annotations() []schema.Annotation {
	return []schema.Annotation{table("blocked_bots")}
}

// Fields of the BlockedBot.
func (BlockedBot) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")),
		field.String("pattern").MaxLen(255),
		field.String("comment").MaxLen(255).Optional().Nillable(),
		field.Time("created_at").SchemaType(mysqlType("datetime")).Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Indexes of the BlockedBot.
func (BlockedBot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("pattern").Unique().StorageKey("blocked_bots_pattern"),
	}
}
