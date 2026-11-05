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

// RatingVote - one voter's vote for a node: a user's, or a guest's (by
// their browser's cookie).
type RatingVote struct {
	ent.Schema
}

// Annotations of the RatingVote.
func (RatingVote) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("rating_votes")}
}

// Fields of the RatingVote.
func (RatingVote) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("nid").SchemaType(schemakit.MySQLType("int unsigned")),
		// uid - the voter, nil for a guest.
		field.Int("uid").SchemaType(schemakit.MySQLType("int unsigned")).Optional().Nillable(),
		// guest - a guest's key: the SHA-256 (hex) of their cookie's token;
		// nil for a user.
		field.String("guest").SchemaType(schemakit.FixedChar(64)).Optional().Nillable(),
		// score of scale - the stars given and the scale they were given
		// on: a change of the site's scale keeps the vote's share.
		field.Int("score").SchemaType(schemakit.MySQLType("smallint")),
		field.Int("scale").SchemaType(schemakit.MySQLType("smallint")),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")),
		field.Time("updated_at").SchemaType(schemakit.MySQLType("datetime")),
	}
}

// Edges of the RatingVote.
func (RatingVote) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("nid").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("user", User.Type).Field("uid").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the RatingVote.
func (RatingVote) Indexes() []ent.Index {
	return []ent.Index{
		// One vote per voter and node (NULLs differ: a guest's row, a user's).
		index.Fields("nid", "uid").Unique().StorageKey("idx_rating_votes_node_user"),
		index.Fields("nid", "guest").Unique().StorageKey("idx_rating_votes_node_guest"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("uid"),
	}
}
