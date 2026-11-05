package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// CommentSettings - what is done with suspicious comments (one row):
// new_author_checks - how many of a new author's first comments wait
// for approval (0 - none, -1 - all); links_limit - more links than that
// wait (-1 - no limit); stop_words - one per line, stop_words_action -
// "hold" or "reject"; duplicate_hours - the same text again by its author
// within that is refused (0 - allowed); spam_keep_days - spam deleted
// after that (0 - kept); bot_trap - forms carry a field bots fill and
// the time they took, bot_min_seconds - a form sent sooner is a bot's.
type CommentSettings struct {
	ent.Schema
}

// Annotations of the CommentSettings.
func (CommentSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("comment_settings")}
}

// Fields of the CommentSettings.
func (CommentSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")).Annotations(notIncremental(), entsql.Default("1")),
		field.Int("new_author_checks").Default(1),
		field.Int("links_limit").Default(2),
		field.Text("stop_words").SchemaType(mysqlType("text")).Optional(),
		field.String("stop_words_action").MaxLen(16).Default("hold"),
		field.Int("duplicate_hours").Default(24),
		field.Int("spam_keep_days").Default(30),
		field.Bool("bot_trap").Default(true),
		field.Int("bot_min_seconds").Default(3),
		// load_with_page - a content page has its comments in its HTML;
		// false - they load once the visitor scrolls to them (ajax).
		field.Bool("load_with_page").Default(true),
	}
}
