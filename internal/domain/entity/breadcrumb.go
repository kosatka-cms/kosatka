package entity

import (
	"context"
	"slices"
	"time"
)

// BreadcrumbItem - one link of a page's breadcrumb trail; an empty URL
// is the current page (rendered as text).
type BreadcrumbItem struct {
	URL   string
	Label string
}

// BreadcrumbRuleDef - "content type + optional taxonomy field and its terms
// → menu link": a node page whose type matches, and whose taxonomy field
// holds one of TermIDs or a descendant of one, gets the trail of MenuLinkID
// plus its own title. The lowest Weight wins.
type BreadcrumbRuleDef struct {
	ID          int
	ContentType string
	// FieldStorageID - the taxonomy field the condition reads; nil = the
	// rule matches every node of ContentType.
	FieldStorageID *int
	// TermIDs - comma-separated term IDs for the field condition.
	TermIDs    string
	MenuLinkID int
	Weight     int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TermIDList - TermIDs parsed (invalid entries skipped).
func (r BreadcrumbRuleDef) TermIDList() []int { return ParseTermIDs(r.TermIDs) }

// HasTerm reports whether id is one of the rule's terms.
func (r BreadcrumbRuleDef) HasTerm(id int) bool {
	return slices.Contains(r.TermIDList(), id)
}

// FieldStorageIDValue - FieldStorageID or 0.
func (r BreadcrumbRuleDef) FieldStorageIDValue() int {
	if r.FieldStorageID == nil {
		return 0
	}
	return *r.FieldStorageID
}

// BreadcrumbRuleDefs - a list of breadcrumb rules.
type BreadcrumbRuleDefs []BreadcrumbRuleDef

// BreadcrumbRuleRepository - storage port for the breadcrumb rules.
type BreadcrumbRuleRepository interface {
	// BreadcrumbRules - all rules ordered by weight, then id.
	BreadcrumbRules(ctx context.Context) (BreadcrumbRuleDefs, error)
	BreadcrumbRuleByID(ctx context.Context, id int) (*BreadcrumbRuleDef, error)
	CreateBreadcrumbRule(ctx context.Context, r BreadcrumbRuleDef) (*BreadcrumbRuleDef, error)
	UpdateBreadcrumbRule(ctx context.Context, r BreadcrumbRuleDef) (*BreadcrumbRuleDef, error)
	UpdateBreadcrumbRuleWeight(ctx context.Context, id, weight int) error
	DeleteBreadcrumbRule(ctx context.Context, id int) error
}
