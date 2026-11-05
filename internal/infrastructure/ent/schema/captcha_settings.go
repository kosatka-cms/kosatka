package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// CaptchaSettings - reCAPTCHA settings (one row).
type CaptchaSettings struct {
	ent.Schema
}

// Annotations of the CaptchaSettings.
func (CaptchaSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{table("captcha_settings")}
}

// Fields of the CaptchaSettings.
func (CaptchaSettings) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("tinyint unsigned")).Annotations(notIncremental()), // the single settings row
		field.Bool("enabled").Default(true),
		field.String("site_key").MaxLen(255).Default(""),
		// secret_key - sealed with the site's key (internal/lib/secretbox) when it
		// has one.
		field.String("secret_key").MaxLen(512).Default(""),
		field.Bool("on_login").Default(true),
		field.Bool("on_register").Default(true),
		field.Bool("on_password").Default(true),
		// module_forms - the modules' CAPTCHA forms (entity.Module.
		// CaptchaForms) as JSON form → shown; a form missing here shows it
		// by its default.
		field.Text("module_forms").SchemaType(mysqlType("text")).Default(""),
	}
}
