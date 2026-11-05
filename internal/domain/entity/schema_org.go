package entity

import "github.com/kosatka-cms/kosatka/pkg/i18n"

// SchemaNone - a content type's markup switched off
// (SEOTemplate.SchemaType); "" is the default scope's.
const SchemaNone = "none"

// SchemaTypeDef - a schema.org type a content type's pages are marked
// up as (JSON-LD): the vocabulary's, not a site's. Every one gets the
// page's name, description, image and address; Creative - its dates;
// Article - its author and the site as the publisher too.
type SchemaTypeDef struct {
	// Name - the schema.org type ("Article").
	Name string
	// Label - the admin's name of it (i18n.N).
	Label string
	// Creative - a creative work: datePublished, dateModified.
	Creative bool
	// Article - headline instead of name, the author, the publisher.
	Article bool
	// Props - the properties the type's templates fill, beside those.
	Props []SchemaPropDef
	// Objects - the type of the object a dotted property ("offers.price")
	// goes into, by its first part ("offers" → "Offer").
	Objects map[string]string
}

// SchemaPropDef - a property a content type's template fills: Name - its
// schema.org name ("keywords"), or a dotted one of an object
// ("location.name"); Label and Hint - the admin's (i18n.N).
type SchemaPropDef struct {
	Name  string
	Label string
	Hint  string
}

// SchemaTypes - the types offered, in the order shown.
var SchemaTypes = []SchemaTypeDef{
	{Name: "Article", Label: i18n.N("Article"), Creative: true, Article: true, Props: articleProps},
	{Name: "NewsArticle", Label: i18n.N("News article"), Creative: true, Article: true, Props: articleProps},
	{Name: "BlogPosting", Label: i18n.N("Blog post"), Creative: true, Article: true, Props: articleProps},
	{Name: "WebPage", Label: i18n.N("Web page"), Creative: true, Props: []SchemaPropDef{
		{Name: "keywords", Label: i18n.N("Keywords"), Hint: i18n.N("Comma-separated, e.g. a tags field.")},
	}},
	{Name: "Event", Label: i18n.N("Event"), Objects: map[string]string{"location": "Place"}, Props: []SchemaPropDef{
		{Name: "startDate", Label: i18n.N("Start"), Hint: i18n.N("A date field with :iso, e.g. [node:field:date:iso].")},
		{Name: "endDate", Label: i18n.N("End"), Hint: i18n.N("A date field with :iso.")},
		{Name: "location.name", Label: i18n.N("Place"), Hint: i18n.N("Its name.")},
		{Name: "location.address", Label: i18n.N("Place's address")},
	}},
	{Name: "Product", Label: i18n.N("Product or service"), Objects: map[string]string{"offers": "Offer", "brand": "Brand"}, Props: []SchemaPropDef{
		{Name: "brand.name", Label: i18n.N("Brand")},
		{Name: "sku", Label: i18n.N("SKU")},
		{Name: "offers.price", Label: i18n.N("Price"), Hint: i18n.N("A number field.")},
		{Name: "offers.priceCurrency", Label: i18n.N("Currency"), Hint: i18n.N("ISO 4217, e.g. EUR.")},
		{Name: "offers.availability", Label: i18n.N("Availability"), Hint: i18n.N("A schema.org address, e.g. https://schema.org/InStock.")},
	}},
}

// articleProps - the properties of the article kinds.
var articleProps = []SchemaPropDef{
	{Name: "keywords", Label: i18n.N("Keywords"), Hint: i18n.N("Comma-separated, e.g. a tags field.")},
	{Name: "articleSection", Label: i18n.N("Section"), Hint: i18n.N("E.g. a category field.")},
}

// SchemaTypeByName - the schema.org type name, if offered.
func SchemaTypeByName(name string) (SchemaTypeDef, bool) {
	for _, t := range SchemaTypes {
		if t.Name == name {
			return t, true
		}
	}
	return SchemaTypeDef{}, false
}

// Who runs the site (SiteSchemaOrg.Kind).
const (
	SchemaOrgOrganization  = "Organization"
	SchemaOrgLocalBusiness = "LocalBusiness"
	SchemaOrgPerson        = "Person"
)

// SiteSchemaOrg - who runs the site, for search engines: the publisher
// of its articles, its card in the results (the front page's JSON-LD).
type SiteSchemaOrg struct {
	// Kind - SchemaOrgOrganization (also ""), SchemaOrgLocalBusiness,
	// SchemaOrgPerson.
	Kind string `json:"kind,omitempty"`
	// Name - "" - the site's name.
	Name string `json:"name,omitempty"`
	// SameAs - its pages elsewhere (social networks, directories).
	SameAs []string `json:"same_as,omitempty"`
	Phone  string   `json:"phone,omitempty"`
	Email  string   `json:"email,omitempty"`
	// Address - a local business's.
	Address string `json:"address,omitempty"`
	// SearchBox - the site's search in the results (WebSite's
	// SearchAction).
	SearchBox bool `json:"search_box,omitempty"`
}

// TypeName - the schema.org type of Kind.
func (o SiteSchemaOrg) TypeName() string {
	switch o.Kind {
	case SchemaOrgLocalBusiness, SchemaOrgPerson:
		return o.Kind
	}
	return SchemaOrgOrganization
}
