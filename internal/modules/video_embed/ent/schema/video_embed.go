// Package schema - the video embed module's own ent schema: what the
// video services say about the videos linked in texts, and the module's
// settings.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// VideoEmbed - a video linked in a text, as its service describes it
// (oEmbed or the page's Open Graph tags), by the link's hash: title,
// thumbnail (thumbnail_url - the service's, thumbnail_file - the site's
// copy under the public files, "" - none), the player's proportions;
// status - "ok", or "failed" (asked again a day later).
type VideoEmbed struct{ ent.Schema }

// Annotations of the VideoEmbed.
func (VideoEmbed) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("video_embeds")}
}

// Fields of the VideoEmbed.
func (VideoEmbed) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("url_hash").MaxLen(64).NotEmpty(),
		field.Text("url").SchemaType(schemakit.MySQLType("text")),
		field.String("provider").MaxLen(32),
		field.String("title").MaxLen(512).Default(""),
		field.Text("thumbnail_url").SchemaType(schemakit.MySQLType("text")).Default(""),
		field.String("thumbnail_file").MaxLen(255).Default(""),
		field.Int("width").SchemaType(schemakit.MySQLType("int")).Default(0),
		field.Int("height").SchemaType(schemakit.MySQLType("int")).Default(0),
		field.String("status").MaxLen(16).Default("ok"),
		field.Time("fetched_at").SchemaType(schemakit.MySQLType("datetime")),
	}
}

// Indexes of the VideoEmbed.
func (VideoEmbed) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("url_hash").Unique().StorageKey("video_embeds_url_hash"),
	}
}

// VideoEmbedSettings - the module's settings (one row, id 1; none - the
// defaults): providers - the services on, by name, space-separated;
// click_to_play - the player loads on a click, a preview before it;
// local_previews - the previews are the site's copies.
type VideoEmbedSettings struct{ ent.Schema }

// Annotations of the VideoEmbedSettings.
func (VideoEmbedSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("video_embed_settings")}
}

// Fields of the VideoEmbedSettings.
func (VideoEmbedSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.String("providers").MaxLen(255).Default(""),
		field.Bool("click_to_play").Default(true),
		field.Bool("local_previews").Default(true),
	}
}
