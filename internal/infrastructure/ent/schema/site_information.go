package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// SiteInformation - Site information (one row).
type SiteInformation struct {
	ent.Schema
}

// Annotations of the SiteInformation.
func (SiteInformation) Annotations() []schema.Annotation {
	return []schema.Annotation{table("site_information")}
}

// Fields of the SiteInformation.
func (SiteInformation) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(mysqlType("int")).Annotations(notIncremental(), entsql.Default("1")),
		field.String("site_name").MaxLen(255).Default("Kosatka CMS"),
		field.String("slogan").MaxLen(512).Default(""),
		field.String("site_email").MaxLen(255).Default(""),
		field.String("logo").MaxLen(255).Optional().Nillable(),
		// icon - the site's icon (favicon), entity.SiteInformation.Icon.
		field.String("icon").MaxLen(255).Optional().Nillable(),
		field.String("front_feed_view").MaxLen(64).Default(""),
		field.String("timezone").MaxLen(64).Optional().Nillable(),
		field.String("default_language").MaxLen(12).Optional().Nillable(),
		field.Text("page_404").SchemaType(mysqlType("text")).Optional().Nillable(),
		field.Text("page_403").SchemaType(mysqlType("text")).Optional().Nillable(),
		field.String("theme").MaxLen(64).Optional().Nillable(),
		field.String("base_url").MaxLen(255).Optional().Nillable(),
		field.Text("robots_extra").SchemaType(mysqlType("text")).Optional().Nillable(),
		// languages - the site's languages besides the default one, each
		// with its addresses' prefix (entity.SiteLanguage); none - one
		// language, no prefixes.
		field.JSON("languages", []entity.SiteLanguage{}).Optional(),
		// schema_org - who runs the site, for search engines.
		field.JSON("schema_org", entity.SiteSchemaOrg{}).Optional(),
	}
}
