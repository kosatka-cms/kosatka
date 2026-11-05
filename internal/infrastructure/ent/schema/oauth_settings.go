package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// OAuthSettings - A social login provider's settings (no row: the config's keys apply).
type OAuthSettings struct {
	ent.Schema
}

// Annotations of the OAuthSettings.
func (OAuthSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("oauth_settings")}
}

// Fields of the OAuthSettings.
func (OAuthSettings) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").StorageKey("provider").MaxLen(16).NotEmpty(),
		field.Bool("enabled").Default(true),
		field.String("client_id").MaxLen(255).Default(""),
		// client_secret - sealed with the site's key (internal/lib/secretbox) when it
		// has one.
		field.String("client_secret").MaxLen(512).Default(""),
	}
}
