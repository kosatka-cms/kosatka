package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SearchWord - a word of a node's title or of its value of a text field,
// as the search looks for it (folded and stemmed: internal/lib/searchtext); one
// row per word, field and node.
type SearchWord struct {
	ent.Schema
}

// Annotations of the SearchWord: the words compared byte by byte in
// MySQL (utf8mb4_bin), as in PostgreSQL - two words a case-insensitive
// collation would take for one stay two.
func (SearchWord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "search_words", Charset: "utf8mb4", Collation: "utf8mb4_bin"}}
}

// Fields of the SearchWord.
func (SearchWord) Fields() []ent.Field {
	return []ent.Field{
		// id - bigint: every save of a node writes its words again.
		field.Int64("id"),
		field.Int("nid").SchemaType(mysqlType("int unsigned")),
		// field_id - the field storage of the word's value; 0 - the title.
		field.Int("field_id").SchemaType(mysqlType("int unsigned")).Default(0),
		field.String("word").MaxLen(64),
	}
}

// Edges of the SearchWord.
func (SearchWord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("node", Node.Type).
			Ref("search_words").
			Unique().
			Required().
			Field("nid"),
	}
}

// Indexes of the SearchWord: by word (the prefix of the last word of a
// query too: varchar_pattern_ops lets PostgreSQL's LIKE 'x%' use it), by
// node (a node's words rewritten).
func (SearchWord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("word", "field_id", "nid").StorageKey("idx_search_words_word").
			Annotations(entsql.OpClassColumn("word", "varchar_pattern_ops")),
		index.Fields("nid", "field_id").StorageKey("idx_search_words_nid"),
	}
}
