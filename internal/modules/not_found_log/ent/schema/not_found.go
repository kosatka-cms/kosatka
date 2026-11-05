// Package schema - the 404 log module's own ent schema: the paths asked
// for and not found, and the module's settings.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// NotFoundPath - a path the site answered 404 to, by the path's hash:
// how many times (hits), when first and last, where the last visitor came
// from (referrer, "" - nowhere told); ignored - not shown among the new
// ones.
type NotFoundPath struct{ ent.Schema }

// Annotations of the NotFoundPath.
func (NotFoundPath) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("not_found_paths")}
}

// Fields of the NotFoundPath.
func (NotFoundPath) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("path_hash").MaxLen(64).NotEmpty(),
		field.String("path").MaxLen(1024),
		field.Int("hits").SchemaType(schemakit.MySQLType("int")).Default(0),
		field.Time("first_seen").SchemaType(schemakit.MySQLType("datetime")),
		field.Time("last_seen").SchemaType(schemakit.MySQLType("datetime")),
		field.String("referrer").MaxLen(1024).Default(""),
		field.Bool("ignored").Default(false),
	}
}

// Indexes of the NotFoundPath.
func (NotFoundPath) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("path_hash").Unique().StorageKey("not_found_paths_hash"),
		index.Fields("ignored", "hits").StorageKey("not_found_paths_ignored_hits"),
		index.Fields("last_seen").StorageKey("not_found_paths_last_seen"),
	}
}

// NotFoundSettings - the module's settings (one row, id 1; none - the
// defaults): keep_days - a path not asked for longer is forgotten;
// show_count - the number of new paths by the Redirects menu item;
// ignore_patterns - the paths never logged, one per line, * - anything.
type NotFoundSettings struct{ ent.Schema }

// Annotations of the NotFoundSettings.
func (NotFoundSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("not_found_settings")}
}

// Fields of the NotFoundSettings.
func (NotFoundSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("keep_days").SchemaType(schemakit.MySQLType("int")).Default(90),
		field.Bool("show_count").Default(true),
		field.Text("ignore_patterns").SchemaType(schemakit.MySQLType("text")).Default(""),
	}
}
