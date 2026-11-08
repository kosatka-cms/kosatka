package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ViewDisplayDef - an admin-created listing (page, block or attachment)
// with its own settings and ViewFilterDef/ViewSortDef/ViewFieldDef rows.
// Content types are just another filter (ViewFilterDef.BuiltinField ==
// "content_type"). view_id is nil for a display not grouped into a View;
// once set, is_default/override_* decide whether each section is the
// display's own or inherited from the View's Default display.
type ViewDisplayDef struct {
	ent.Schema
}

// Annotations of the ViewDisplayDef.
func (ViewDisplayDef) Annotations() []schema.Annotation {
	return []schema.Annotation{
		table("view_displays"),
	}
}

// Fields of the ViewDisplayDef.
func (ViewDisplayDef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		// base_entity - which underlying table this display's Filters/
		// Sort/Fields/RunView target (the "base table"). Immutable
		// once created - see entity.ViewBaseEntity's own doc comment.
		field.String("base_entity").MaxLen(16).Default("content"),
		// view_id - the owning View (container). Nil for a "lone" display
		// that's never been cloned (see entity.ViewDisplayDef.ViewID).
		field.Int("view_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// is_default - true for the one per-View display that's never
		// resolved as a page/block, only ever the shared baseline other
		// displays in the same View inherit from.
		field.Bool("is_default").Default(false),
		// override_settings/filters/sort/fields - per-section "override" vs
		// "use default" state relative to the View's Default display. Ignored when
		// is_default is true.
		field.Bool("override_settings").Default(false),
		field.Bool("override_filters").Default(false),
		field.Bool("override_sort").Default(false),
		field.Bool("override_fields").Default(false),
		field.String("machine_name").MaxLen(64).NotEmpty().Unique(),
		field.String("label").MaxLen(255).NotEmpty(),
		// meta_title/meta_description - SEO fields distinct from
		// Label (the on-page heading/nav title): meaningful only for
		// display_type == "page". Empty meta_title falls back to
		// Label.
		field.String("meta_title").MaxLen(255).Optional(),
		field.Text("meta_description").SchemaType(mysqlType("mediumtext")).Optional(),
		// url_path - the display's page path. Empty/NULL for display_type ==
		// "block"; a never-resolved placeholder when is_default is true.
		field.String("url_path").MaxLen(255).Optional().Unique(),
		// display_type - entity.ViewDisplayType: "page" (own URL), "block" (no
		// URL; placed as a block), "attachment", "term", "user_tab".
		field.String("display_type").MaxLen(10).Default("page"),
		// attach_* - display_type == "attachment" only (a View attached to node
		// detail pages), see entity.ViewDisplayDef.AttachContentTypes.
		field.String("attach_content_types").MaxLen(255).Optional(),
		field.Int("attach_weight").SchemaType(mysqlType("int")).Default(0),
		// term_vocabularies - entity.ViewDisplayDef.TermVocabularies.
		field.String("term_vocabularies").MaxLen(255).Default(""),
		// profile_tab_path / profile_tab_access -
		// entity.ViewDisplayDef.ProfileTabPath/ProfileTabAccess.
		field.String("profile_tab_path").MaxLen(64).Default(""),
		field.String("profile_tab_access").MaxLen(16).Default("all"),
		// theme_template - entity.ViewDisplayDef.ThemeTemplate.
		field.String("theme_template").MaxLen(64).Default(""),
		field.String("attach_title").MaxLen(255).Optional(),
		field.Bool("attach_show_title").Default(true),
		// display_style - entity.ViewDisplayStyle: "grid" | "list" | "table"
		// (columns from view_fields) | "carousel".
		field.String("display_style").MaxLen(10).Default("grid"),
		// grid_columns - display_style == "grid" only. Default 5 matches the
		// default .list-grid CSS.
		field.Int("grid_columns").SchemaType(mysqlType("int")).Default(5),
		// overflow_after - 0 renders every item in display_style; >0 renders only
		// the first N that way and the rest as a compact list below (see
		// entity.ViewDisplayDef.OverflowAfter). Independent of display_style.
		field.Int("overflow_after").SchemaType(mysqlType("int")).Default(0),
		// carousel_slides_per_view/autoplay/loop/show_navigation/show_pagination -
		// display_style == "carousel" only. Defaults: 1 slide, no autoplay or
		// loop, navigation and pagination on.
		field.Int("carousel_slides_per_view").SchemaType(mysqlType("int")).Default(1),
		field.Bool("carousel_autoplay").Default(false),
		field.Bool("carousel_loop").Default(false),
		field.Bool("carousel_show_navigation").Default(true),
		field.Bool("carousel_show_pagination").Default(true),
		// carousel_link_field_storage_id - nil means slides aren't wrapped in a
		// link; otherwise the whole item links to this text field's value
		// (the settings offer the text fields; only text values are read,
		// views.CarouselLinkURLs).
		field.Int("carousel_link_field_storage_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		// carousel_effect - Swiper transition: "slide" (default) or "fade". A plain
		// string so other Swiper effects need no schema change.
		field.String("carousel_effect").MaxLen(10).Default("slide"),
		field.Bool("show_pagination").Default(true),
		// pagination_ajax - only when show_pagination is true: pager links fetch
		// the next page and replace #view-list-container's content (see
		// ViewPageFallback's X-Requested-With branch and initViewPagination in
		// web/static/js/app.js). Off by default; plain ?page=N links need no JS.
		field.Bool("pagination_ajax").Default(false),
		// show_glossary - opt-in letter-index bar. RunView adds an in-memory
		// title starts-with filter from ?letter=X (service.glossaryLetterFilter).
		field.Bool("show_glossary").Default(false),
		// glossary_field_storage_id - nil means the glossary uses Node.Title;
		// otherwise a FieldTypeText field (enforced by views.GlossaryFieldType).
		field.Int("glossary_field_storage_id").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}).Optional().Nillable(),
		field.Int("page_size").SchemaType(mysqlType("int")).Default(30),
		// skip_items - the first skip_items items of the result are left out
		// (entity.ViewDisplayDef.SkipItems); 0 = none.
		field.Int("skip_items").SchemaType(mysqlType("int")).Default(0),
		// header_text/footer_text - rich text rendered before/after the list. Part
		// of the Settings bundle (inherited from the View's Default with it).
		// Sanitized on save by views.DefaultService.
		field.Text("header_text").SchemaType(mysqlType("mediumtext")).Optional(),
		field.Text("footer_text").SchemaType(mysqlType("mediumtext")).Optional(),
		// overflow_header_text - plain-text heading rendered before the compact
		// overflow list (see overflow_after), only when that list is non-empty.
		field.String("overflow_header_text").MaxLen(255).Optional(),
		// custom_css - raw CSS rendered as an inline <style> on the display's page.
		// Not HTML-sanitized; only "</style>" is rejected on save. Not namespaced:
		// #view-list-container is shared by all View pages, so admins must scope
		// their own selectors.
		field.Text("custom_css").SchemaType(mysqlType("mediumtext")).Optional(),
		// row_template - row (card) markup for grid/list/carousel displays with
		// tokens {{field:<machine_name>}} (rendered field) and {{url}}; empty means
		// fields one after another. Sanitized on save like rewrite_html.
		field.Text("row_template").SchemaType(mysqlType("mediumtext")).Optional(),
		// group_by_field/group_date_format - machine name of a display field whose
		// value groups the rows (one heading per group); group_date_format for a
		// date field: "" (as rendered), "year", "month", "day".
		field.String("group_by_field").MaxLen(64).Optional(),
		field.String("group_date_format").MaxLen(8).Optional(),
		// published_snapshot/published_at - published_snapshot is a JSON-encoded
		// entity.ViewSnapshot (the last-published effective settings, filters, sorts
		// and fields); published_at nil means never published. Written only via
		// SetPublishedSnapshot, not UpdateView.
		field.Text("published_snapshot").SchemaType(mysqlType("mediumtext")).Optional(),
		field.Time("published_at").SchemaType(mysqlType("datetime")).Optional().Nillable(),
		field.Int("status").SchemaType(mysqlType("tinyint")).Default(1), // 1=enabled (resolves on the public side), 0=disabled
		field.Int("created_by").SchemaType(map[string]string{dialect.MySQL: "int unsigned"}),
		field.Time("created_at").SchemaType(mysqlType("datetime")),
		field.Time("updated_at").SchemaType(mysqlType("datetime")),
	}
}

// Indexes of the ViewDisplayDef.
func (ViewDisplayDef) Indexes() []ent.Index {
	return []ent.Index{
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("carousel_link_field_storage_id"),
		index.Fields("glossary_field_storage_id"),
		index.Fields("view_id"),
	}
}

// Edges of the ViewDisplayDef.
func (ViewDisplayDef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("view", View.Type).Field("view_id").Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.From("filters", ViewFilterDef.Type).Ref("view_display"),
		edge.From("sorts", ViewSortDef.Type).Ref("view_display"),
		edge.From("view_fields", ViewFieldDef.Type).Ref("view_display"),
		edge.To("glossary_field_storage", FieldStorageDef.Type).Field("glossary_field_storage_id").Unique().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("carousel_link_field_storage", FieldStorageDef.Type).Field("carousel_link_field_storage_id").Unique().Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}
