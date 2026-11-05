// Package schema - the forms module's ent schema, apart from the core's.
// A form is a core node and a submission's author a core user: Node and
// User are the module's stand-ins for their tables (migrations.Sets).
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// Webform - the forms module's form: a node of type "form" (its title,
// address, SEO) with its own elements and submissions. Its elements are rows, not fields of a content type: each form
// has its own.
type Webform struct {
	ent.Schema
}

// Annotations of the Webform.
func (Webform) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("webforms")}
}

// Fields of the Webform.
func (Webform) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("nid").SchemaType(schemakit.MySQLType("int unsigned")),
		// description - the text above the form (HTML).
		field.Text("description").SchemaType(schemakit.MySQLType("mediumtext")).Optional(),
		// confirmation - the message shown after a submission (HTML).
		field.Text("confirmation").SchemaType(schemakit.MySQLType("mediumtext")).Optional(),
		// confirmation_url - the page a submission leads to instead of the
		// message (a path on the site; "" - the message).
		field.String("confirmation_url").MaxLen(500).Default(""),
		// submit_label - the submit button's text ("" - the default).
		field.String("submit_label").MaxLen(128).Default(""),
		// closed - the form takes no submissions.
		field.Bool("closed").Default(false),
		// notify_email - addresses told about every submission, comma
		// separated ("" - nobody).
		field.String("notify_email").MaxLen(500).Default(""),
		// limit_per_user - one submission per signed-in user (visitors
		// must sign in to send it).
		field.Bool("limit_per_user").Default(false),
		// max_submissions - the form closes after this many (0 - no
		// limit).
		field.Int("max_submissions").SchemaType(schemakit.MySQLType("int")).Default(0),
		// closes_at - the form closes at this time (NULL - never).
		field.Time("closes_at").SchemaType(schemakit.MySQLType("datetime")).Optional().Nillable(),
	}
}

// Edges of the Webform.
func (Webform) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("node", Node.Type).Field("nid").Required().Unique().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("elements", WebformElement.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("submissions", WebformSubmission.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the Webform.
func (Webform) Indexes() []ent.Index {
	return []ent.Index{index.Fields("nid").Unique().StorageKey("webforms_nid")}
}

// WebformElement - one element (question) of a form.
type WebformElement struct {
	ent.Schema
}

// Annotations of the WebformElement.
func (WebformElement) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("webform_elements")}
}

// Fields of the WebformElement.
func (WebformElement) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("webform_id").SchemaType(schemakit.MySQLType("int unsigned")),
		// machine_name - the element's key in exports and e-mails, unique
		// within its form, kept when the label changes.
		field.String("machine_name").MaxLen(64).NotEmpty(),
		// type - entity.WebformElementType.
		field.String("type").MaxLen(32).NotEmpty(),
		field.String("label").MaxLen(500).NotEmpty(),
		field.Text("description").SchemaType(schemakit.MySQLType("text")).Optional(),
		field.Bool("required").Default(false),
		// options - the choices of a select/radios/checkboxes element, one
		// per line; a consent element's link (its policy page).
		field.Text("options").SchemaType(schemakit.MySQLType("text")).Optional(),
		field.Int("weight").SchemaType(schemakit.MySQLType("int")).Default(0),
		// visible_if_element / visible_if_value - the element is shown
		// only when an earlier element (machine name) has this value
		// ("" - any value); "" - always shown.
		field.String("visible_if_element").MaxLen(64).Default(""),
		field.String("visible_if_value").MaxLen(500).Default(""),
		// prefill_param - the address's query parameter whose value fills
		// the element when the form opens ("" - none): a product's page
		// links to /form/…?product=… and the question is answered.
		field.String("prefill_param").MaxLen(64).Default(""),
	}
}

// Edges of the WebformElement.
func (WebformElement) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("webform", Webform.Type).Ref("elements").Field("webform_id").Unique().Required(),
		edge.To("values", WebformSubmissionValue.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the WebformElement.
func (WebformElement) Indexes() []ent.Index {
	return []ent.Index{index.Fields("webform_id", "machine_name").Unique().StorageKey("webform_elements_key")}
}

// WebformSubmission - one sent form.
type WebformSubmission struct {
	ent.Schema
}

// Annotations of the WebformSubmission.
func (WebformSubmission) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("webform_submissions")}
}

// Fields of the WebformSubmission.
func (WebformSubmission) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("webform_id").SchemaType(schemakit.MySQLType("int unsigned")),
		// user_id - the signed-in sender (NULL - anonymous, or the
		// account was deleted).
		field.Int("user_id").SchemaType(schemakit.MySQLType("int unsigned")).Optional().Nillable(),
		field.String("remote_ip").MaxLen(64).Default(""),
		// source_url - the site's page the form was sent from (a path; "" -
		// unknown): the page it's embedded in, or the one that linked to
		// the form's own page.
		field.String("source_url").MaxLen(500).Default(""),
		field.Time("created_at").SchemaType(schemakit.MySQLType("datetime")).Default(time.Now),
	}
}

// Edges of the WebformSubmission.
func (WebformSubmission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("webform", Webform.Type).Ref("submissions").Field("webform_id").Unique().Required(),
		edge.To("user", User.Type).Field("user_id").Unique().Annotations(entsql.OnDelete(entsql.SetNull)),
		edge.To("values", WebformSubmissionValue.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the WebformSubmission.
func (WebformSubmission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("webform_id", "created_at"),
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("user_id"),
	}
}

// WebformSubmissionValue - an element's value in a submission; a
// checkboxes element has a row per checked choice (delta).
type WebformSubmissionValue struct {
	ent.Schema
}

// Annotations of the WebformSubmissionValue.
func (WebformSubmissionValue) Annotations() []schema.Annotation {
	return []schema.Annotation{schemakit.Table("webform_submission_values")}
}

// Fields of the WebformSubmissionValue.
func (WebformSubmissionValue) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("submission_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("element_id").SchemaType(schemakit.MySQLType("int unsigned")),
		field.Int("delta").SchemaType(schemakit.MySQLType("int")).Default(0),
		field.Text("value").SchemaType(schemakit.MySQLType("mediumtext")),
	}
}

// Indexes of the WebformSubmissionValue.
func (WebformSubmissionValue) Indexes() []ent.Index {
	return []ent.Index{
		// Foreign key columns: PostgreSQL doesn't index them itself (MySQL
		// does), and deleting the row they refer to looks them up.
		index.Fields("element_id"),
		index.Fields("submission_id"),
	}
}

// Edges of the WebformSubmissionValue.
func (WebformSubmissionValue) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("submission", WebformSubmission.Type).Ref("values").Field("submission_id").Unique().Required(),
		edge.From("element", WebformElement.Type).Ref("values").Field("element_id").Unique().Required(),
	}
}
