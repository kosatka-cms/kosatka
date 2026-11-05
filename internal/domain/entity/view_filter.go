package entity

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// ViewFilterOperator - the comparison a ViewFilterDef applies. The
// allowed subset depends on the target field's type (or builtin).
type ViewFilterOperator string

// The filter operators (ViewFilterOperator).
const (
	ViewFilterOperatorEq ViewFilterOperator = "eq"
	ViewFilterOperatorIn ViewFilterOperator = "in"
	// ViewFilterOperatorNotIn - "has none of these values" (same
	// comma-separated Value as In). Nodes with no value also match.
	ViewFilterOperatorNotIn    ViewFilterOperator = "not_in"
	ViewFilterOperatorContains ViewFilterOperator = "contains"
	ViewFilterOperatorGte      ViewFilterOperator = "gte"
	ViewFilterOperatorLte      ViewFilterOperator = "lte"
	// ViewFilterOperatorEmpty/NotEmpty - the generic field has no value /
	// has a value; no Value needed. Not offered for builtins, which
	// always have a value (see service.validOperatorsForBuiltin).
	ViewFilterOperatorEmpty    ViewFilterOperator = "empty"
	ViewFilterOperatorNotEmpty ViewFilterOperator = "not_empty"
	// ViewFilterOperatorOngoing/NotEnded/Ended - a date field's value is
	// going on now / hasn't ended (going on or to come) / has ended, by
	// DateValue.Ended and Ongoing (a date without a time - by the site's
	// today); a field with several dates - any of them going on or not
	// ended, all of them ended. No Value needed.
	ViewFilterOperatorOngoing  ViewFilterOperator = "ongoing"
	ViewFilterOperatorNotEnded ViewFilterOperator = "not_ended"
	ViewFilterOperatorEnded    ViewFilterOperator = "ended"
	// ViewFilterOperatorStartsWith - case-insensitive prefix match backing
	// the glossary letter filter (ViewDisplayDef.ShowGlossary). Not
	// selectable in the admin UI: only service.glossaryLetterFilter
	// builds it, bypassing validateFilter.
	ViewFilterOperatorStartsWith ViewFilterOperator = "starts_with"
	// ViewFilterOperatorReferencedByCurrent/RelatedToCurrent - only for a
	// Contextual filter on a FieldTypeReference field: nodes the current
	// node references via that field; RelatedToCurrent also includes
	// nodes referencing the current one via the same field (a symmetric
	// relation editable from either side).
	ViewFilterOperatorReferencedByCurrent ViewFilterOperator = "referenced_by_current"
	ViewFilterOperatorRelatedToCurrent    ViewFilterOperator = "related_to_current"
	// ViewFilterOperatorIsProfileUser - only for a Contextual filter on a
	// FieldTypeUser field: nodes naming the user of the page (a profile
	// tab's user, the user of a %-address View) in that field.
	ViewFilterOperatorIsProfileUser ViewFilterOperator = "is_profile_user"
)

// IsDateState - o is one of the date state operators (ongoing, not
// ended, ended): it takes no value.
func (o ViewFilterOperator) IsDateState() bool {
	return o == ViewFilterOperatorOngoing || o == ViewFilterOperatorNotEnded || o == ViewFilterOperatorEnded
}

// ViewFilterBuiltinField - built-in columns (or pseudo-columns) a
// ViewFilterDef targets when FieldStorageID is nil.
type ViewFilterBuiltinField string

// The built-in filter targets: the node's, then the comment base
// entity's.
const (
	ViewFilterBuiltinStatus       ViewFilterBuiltinField = "status"
	ViewFilterBuiltinOnModeration ViewFilterBuiltinField = "on_moderation"
	// ViewFilterBuiltinPromote / ViewFilterBuiltinSticky - Node.Promote
	// ("on the front page") and Node.Sticky; Value "1" or "0".
	ViewFilterBuiltinPromote   ViewFilterBuiltinField = "promote"
	ViewFilterBuiltinSticky    ViewFilterBuiltinField = "sticky"
	ViewFilterBuiltinCreatedBy ViewFilterBuiltinField = "created_by"
	ViewFilterBuiltinTitle     ViewFilterBuiltinField = "title"
	// ViewFilterBuiltinContentType - the node's type. Value is a content
	// type machine name (eq) or a comma-separated list (in). The only
	// way to scope a display to specific content types.
	ViewFilterBuiltinContentType ViewFilterBuiltinField = "content_type"
	// ViewFilterBuiltinSimilarToCurrentNode - Contextual builtin: the
	// published nodes sharing taxonomy terms with the current node, ranked
	// live (via taxonomy_index) by those terms weighted by rarity - the
	// fewer nodes have a term, the more it counts. Value - optional
	// vocabulary ID to compare only its terms; empty = every vocabulary.
	// Spans every type unless combined with a content_type filter; the
	// ranking replaces the View's order, its sort criteria break ties.
	ViewFilterBuiltinSimilarToCurrentNode ViewFilterBuiltinField = "similar_to_current_node"
	// ViewFilterBuiltinSharesTermsWithCurrentNode - Contextual builtin:
	// nodes sharing at least one taxonomy term with the current node (live,
	// via taxonomy_index). Value - optional vocabulary ID to restrict to;
	// empty = any vocabulary.
	ViewFilterBuiltinSharesTermsWithCurrentNode ViewFilterBuiltinField = "shares_terms_with_current"
	// ViewFilterBuiltinHasCurrentTerm - Contextual builtin for term
	// displays: nodes indexed under the term being rendered
	// (ViewRunParams.ContextTermID). No term in context = empty result.
	ViewFilterBuiltinHasCurrentTerm ViewFilterBuiltinField = "has_current_term"

	// Comment base entity builtins, mirroring the content ones where a
	// comment analog exists.
	ViewFilterBuiltinCommentStatus    ViewFilterBuiltinField = "comment_status"
	ViewFilterBuiltinCommentSubject   ViewFilterBuiltinField = "comment_subject"
	ViewFilterBuiltinCommentContent   ViewFilterBuiltinField = "comment_content"
	ViewFilterBuiltinCommentCreatedBy ViewFilterBuiltinField = "comment_created_by"
	// ViewFilterBuiltinCommentNodeType - the parent node's type; same
	// Value/operator convention as ViewFilterBuiltinContentType.
	ViewFilterBuiltinCommentNodeType ViewFilterBuiltinField = "comment_node_type"
	// ViewFilterBuiltinCommentIsReply - whether the comment is a reply
	// (ParentID set). Operator eq only, Value "0"/"1".
	ViewFilterBuiltinCommentIsReply ViewFilterBuiltinField = "comment_is_reply"
)

// ViewFilterDef - one filter criterion of a ViewDisplayDef. Its value
// comes from exactly one source: fixed (Value), exposed (ExposedParam,
// read from the query string) or contextual (the current page).
type ViewFilterDef struct {
	ID int
	// ViewDisplayID - the owning ViewDisplayDef.ID.
	ViewDisplayID int
	// FieldStorageID nil means this filter reads BuiltinField instead
	// of a generic field.
	FieldStorageID *int
	// ViaFieldStorageID - a blocks field (FieldTypeBlocks): FieldStorageID
	// is a field of its blocks' sets, and the filter keeps the nodes with
	// a block that matches; nil - the node's own field.
	ViaFieldStorageID *int
	BuiltinField      ViewFilterBuiltinField
	Operator          ViewFilterOperator
	// Value - the fixed filter value, used only when Exposed==false.
	Value string
	// ValueIsRelative - date fields only: Value is a relative expression
	// ("now", "+7 days", "-1 month") resolved against today on every
	// query (see ParseRelativeDate) instead of a literal "2006-01-02"
	// date. Not used for exposed filters.
	ValueIsRelative bool
	Exposed         bool
	// ExposedParam - the query parameter read when Exposed==true (e.g.
	// "category" for ?category=foo).
	ExposedParam string
	// ExposedLabel - the filter's label on the public exposed-filters
	// form. Empty falls back at render time (buildExposedFilterFields) to
	// the target field's label; never stored as a default.
	ExposedLabel string
	// Contextual - the value comes from the current page instead of
	// Value/ExposedParam. Valid only for the created_by builtin, the
	// *_current builtins and reference fields with the *Current
	// operators (enforced in views.DefaultService.validateFilter); resolved
	// from ViewRunParams.Context*. Where no context exists (a page View, a
	// cache-time block render) the filter yields an empty result, not an
	// unfiltered one - see RunView.
	Contextual bool
	// SortOrder - position in the admin filter list and the public
	// exposed-filters form.
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ViewFilterDefs - a list of a display's filters.
type ViewFilterDefs []ViewFilterDef

// HasContextual reports whether any filter is Contextual. Such
// View-blocks can't be pre-rendered by CacheService and need a live
// per-request render.
func (fs ViewFilterDefs) HasContextual() bool {
	for _, f := range fs {
		if f.Contextual {
			return true
		}
	}
	return false
}

var relativeDateExprRe = regexp.MustCompile(`^([+-]?\d+)\s*(days?|weeks?|months?|years?)$`)

// ParseRelativeDate resolves a relative date expression ("now", "+7
// days", "-1 month", "+2 weeks", "+1 year"; empty = "now") against
// today's date in the site time zone, returned as UTC midnight - the
// same date-only precision FieldValueDate stores, so comparisons match
// literal dates. Used by both the query layer and the save-time
// validator.
func ParseRelativeDate(expr string, now time.Time) (time.Time, error) {
	local := sitetime.In(now)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	expr = strings.ToLower(strings.TrimSpace(expr))
	if expr == "" || expr == "now" {
		return today, nil
	}
	m := relativeDateExprRe.FindStringSubmatch(expr)
	if m == nil {
		return time.Time{}, fmt.Errorf("invalid relative date expression %q (expected \"now\", \"+N days\", \"-N months\", etc.)", expr)
	}
	n, _ := strconv.Atoi(m[1])
	switch {
	case strings.HasPrefix(m[2], "day"):
		return today.AddDate(0, 0, n), nil
	case strings.HasPrefix(m[2], "week"):
		return today.AddDate(0, 0, n*7), nil
	case strings.HasPrefix(m[2], "month"):
		return today.AddDate(0, n, 0), nil
	default: // year(s)
		return today.AddDate(n, 0, 0), nil
	}
}

// FieldStorageIDValue - FieldStorageID, 0 when the filter targets a
// builtin.
func (f ViewFilterDef) FieldStorageIDValue() int {
	if f.FieldStorageID == nil {
		return 0
	}
	return *f.FieldStorageID
}

// ViaFieldStorageIDValue - ViaFieldStorageID, 0 for nil (templates).
func (f ViewFilterDef) ViaFieldStorageIDValue() int {
	if f.ViaFieldStorageID == nil {
		return 0
	}
	return *f.ViaFieldStorageID
}

// Target - the filter's target as the admin's picker names it:
// "field:<storage>", "block:<blocks field>:<storage>" (ViaFieldStorageID)
// or "builtin:<name>".
func (f ViewFilterDef) Target() string {
	switch {
	case f.FieldStorageID != nil && f.ViaFieldStorageID != nil:
		return fmt.Sprintf("block:%d:%d", *f.ViaFieldStorageID, *f.FieldStorageID)
	case f.FieldStorageID != nil:
		return fmt.Sprintf("field:%d", *f.FieldStorageID)
	}
	return "builtin:" + string(f.BuiltinField)
}

// SortBySortOrder returns a copy sorted by SortOrder, ties broken by ID
// so equal SortOrders keep a deterministic order.
func (fs ViewFilterDefs) SortBySortOrder() ViewFilterDefs {
	sorted := make(ViewFilterDefs, len(fs))
	copy(sorted, fs)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			a, b := sorted[j-1], sorted[j]
			if a.SortOrder < b.SortOrder || (a.SortOrder == b.SortOrder && a.ID <= b.ID) {
				break
			}
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	return sorted
}
