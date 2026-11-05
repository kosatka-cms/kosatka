package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// IntegrationSettings - Counters and codes (one row): verification tags, Metrika, Analytics, raw head/body code.
type IntegrationSettings struct {
	ent.Schema
}

// Annotations of the IntegrationSettings.
func (IntegrationSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("integration_settings")}
}

// Fields of the IntegrationSettings.
func (IntegrationSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int unsigned")).Annotations(notIncremental()), // the single settings row
		field.Text("verification_tags").SchemaType(mysqlType("text")),
		field.String("metrika_id").MaxLen(32).Default(""),
		field.Bool("metrika_clickmap").Default(true),
		field.Bool("metrika_track_links").Default(true),
		field.Bool("metrika_accurate_bounce").Default(true),
		field.Bool("metrika_webvisor").Default(false),
		field.String("ga_id").MaxLen(32).Default(""),
		field.Text("head_code").SchemaType(mysqlType("mediumtext")),
		field.Text("body_end_code").SchemaType(mysqlType("mediumtext")),
		field.Bool("body_end_skip_service_pages").Default(true),
		// consent_* - the visitors' consent to the codes (entity.Consent):
		// the banner on, its mode (consent | notice), its text ("" - the
		// default), its "more" page, its version (a new one asks everyone
		// again); each code's group ("" - no consent needed, analytics,
		// marketing).
		field.Bool("consent_enabled").Default(false),
		field.String("consent_mode").MaxLen(16).Default("consent"),
		field.Text("consent_text").SchemaType(mysqlType("text")).Optional(),
		field.String("consent_policy_url").MaxLen(255).Default(""),
		field.Int("consent_version").Default(1),
		field.String("metrika_group").MaxLen(16).Default("analytics"),
		field.String("ga_group").MaxLen(16).Default("analytics"),
		field.String("head_code_group").MaxLen(16).Default(""),
		field.String("body_end_group").MaxLen(16).Default(""),
	}
}
