package entity

import (
	"context"
	"slices"
	"strconv"
	"strings"
)

// SEO templates (default meta tags): a node page's title and
// meta description come from the node's own override (PageSEO), else
// the first matching SEORuleDef, else its type's SEOTemplate, else the
// default SEOTemplate, else the built-in fallback.

// SEOTemplateDefaultScope - the scope of the template every node falls
// back to ("Content").
const SEOTemplateDefaultScope = "node"

// SEOTemplateTypeScope - the scope of a content type's template
// (e.g. "Content: Article" is node__article).
func SEOTemplateTypeScope(machineName string) string {
	return SEOTemplateDefaultScope + "__" + machineName
}

// SEOTemplateTermScope - the template every taxonomy term page falls
// back to ("Taxonomy term").
const SEOTemplateTermScope = "taxonomy_term"

// SEOTemplateVocabularyScope - a vocabulary's term pages template
// (e.g. "Taxonomy term: Tags").
func SEOTemplateVocabularyScope(vocabularyID int) string {
	return SEOTemplateTermScope + "__" + strconv.Itoa(vocabularyID)
}

// SEOTemplate - token templates of one scope; an empty template defers
// to the next level.
type SEOTemplate struct {
	Scope               string
	TitleTemplate       string
	DescriptionTemplate string
	// ImageTemplate - node scopes only: the og:image, e.g.
	// [node:field:<image field>].
	ImageTemplate string
	// SchemaType - node scopes only: the schema.org type of the pages'
	// JSON-LD (SchemaTypes); "" - the default scope's, SchemaNone - none.
	SchemaType string
	// SchemaProps - its properties' templates by name (SchemaPropDef).
	SchemaProps map[string]string
}

// SEOTemplates - by scope.
type SEOTemplates map[string]SEOTemplate

// SEORuleDef - "content type + optional taxonomy field and its terms →
// own templates": like BreadcrumbRuleDef, with templates instead of a menu
// link. The lowest Weight wins.
type SEORuleDef struct {
	ID          int
	ContentType string
	// FieldStorageID - the taxonomy field the condition reads; nil = the
	// rule matches every node of ContentType.
	FieldStorageID *int
	// TermIDs - comma-separated term IDs for the field condition.
	TermIDs             string
	TitleTemplate       string
	DescriptionTemplate string
	Weight              int
}

// TermIDList - TermIDs parsed (invalid entries skipped).
func (r SEORuleDef) TermIDList() []int { return ParseTermIDs(r.TermIDs) }

// HasTerm reports whether id is one of the rule's terms.
func (r SEORuleDef) HasTerm(id int) bool { return slices.Contains(r.TermIDList(), id) }

// FieldStorageIDValue - FieldStorageID or 0.
func (r SEORuleDef) FieldStorageIDValue() int {
	if r.FieldStorageID == nil {
		return 0
	}
	return *r.FieldStorageID
}

// SEORuleDefs - a list of SEO rules.
type SEORuleDefs []SEORuleDef

// ParseTermIDs - comma-separated term IDs (invalid entries skipped).
func ParseTermIDs(csv string) []int {
	var out []int
	for _, s := range strings.Split(csv, ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
}

// SEOTemplateRepository - storage port for the SEO templates.
type SEOTemplateRepository interface {
	SEOTemplates(ctx context.Context) (SEOTemplates, error)
	SaveSEOTemplate(ctx context.Context, t SEOTemplate) error
}

// SEORuleRepository - storage port for the SEO rules.
type SEORuleRepository interface {
	// SEORules - all rules ordered by weight, then id.
	SEORules(ctx context.Context) (SEORuleDefs, error)
	SEORuleByID(ctx context.Context, id int) (*SEORuleDef, error)
	CreateSEORule(ctx context.Context, r SEORuleDef) (*SEORuleDef, error)
	UpdateSEORule(ctx context.Context, r SEORuleDef) error
	UpdateSEORuleWeight(ctx context.Context, id, weight int) error
	DeleteSEORule(ctx context.Context, id int) error
}
