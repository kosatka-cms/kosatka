package views

import (
	"context"
	"errors"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// viewFieldType - what Views do with a field type: its filters'
// operators (none - no filters by it), whether a row shows it (shown),
// results are ordered by it (sortable; values compared in Go, see
// sortNodeIDsByCriteria) and a glossary groups by its first
// character (glossary). Every field type is here (the completeness test
// TestFieldTypesDecidedInViews); one Views can't use has its reason.
type viewFieldType struct {
	operators                 []entity.ViewFilterOperator
	shown, sortable, glossary bool
}

// presenceOperators - empty/not_empty: every filterable field type's (not the
// builtins': a node's own column always has a value).
var presenceOperators = []entity.ViewFilterOperator{entity.ViewFilterOperatorEmpty, entity.ViewFilterOperatorNotEmpty}

// textOperators - a text's filters: an e-mail and a phone are filtered
// as text.
var textOperators = append([]entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorContains}, presenceOperators...)

var viewFieldTypes = map[entity.ContentTypeFieldType]viewFieldType{
	entity.FieldTypeText:   {operators: textOperators, shown: true, sortable: true, glossary: true},
	entity.FieldTypeEmail:  {operators: textOperators, shown: true},
	entity.FieldTypePhone:  {operators: textOperators, shown: true},
	entity.FieldTypeNumber: {operators: append([]entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorGte, entity.ViewFilterOperatorLte}, presenceOperators...), shown: true, sortable: true},
	entity.FieldTypeDate: {operators: append([]entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorGte, entity.ViewFilterOperatorLte,
		entity.ViewFilterOperatorOngoing, entity.ViewFilterOperatorNotEnded, entity.ViewFilterOperatorEnded}, presenceOperators...), shown: true, sortable: true},
	// Bool: no useful order.
	entity.FieldTypeBool: {operators: append([]entity.ViewFilterOperator{entity.ViewFilterOperatorEq}, presenceOperators...), shown: true},
	// Taxonomy: an order would need the terms' names.
	entity.FieldTypeTaxonomy: {operators: append([]entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorIn, entity.ViewFilterOperatorNotIn}, presenceOperators...), shown: true},
	// Image: a stored file has no meaningful comparison, only presenceOperators.
	entity.FieldTypeImage: {operators: presenceOperators, shown: true},
	// Reference: contextual only (validateFilter) - "referenced by /
	// related to the currently viewed node".
	entity.FieldTypeReference: {operators: []entity.ViewFilterOperator{entity.ViewFilterOperatorReferencedByCurrent, entity.ViewFilterOperatorRelatedToCurrent}, shown: true},
	// User: "names the profile's user" (contextual) or presenceOperators.
	entity.FieldTypeUser: {operators: append([]entity.ViewFilterOperator{entity.ViewFilterOperatorIsProfileUser}, presenceOperators...), shown: true},
	// List: one of the options' values (eq), one of several (in, not_in,
	// comma-separated); shown by its label.
	entity.FieldTypeList: {operators: append([]entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorIn, entity.ViewFilterOperatorNotIn}, presenceOperators...), shown: true},
	// Link: addresses, nothing to compare; shown by its formatter.
	entity.FieldTypeLink: {shown: true},
	// File: a download link of the content page only.
	entity.FieldTypeFile: {},
	// Blocks: shown on the content page; its sets' fields are filtered
	// instead (ViewFilterDef.ViaFieldStorageID).
	entity.FieldTypeBlocks: {},
	// Group: no value.
	entity.FieldTypeGroup: {},
}

// validOperatorsFor returns the FilterOperators valid for fieldType
// (viewFieldTypes); CreateFilter/UpdateFilter use it to reject
// combinations the query layer (nodeIDsForGenericFilter) doesn't
// implement. None - not filterable.
func validOperatorsFor(fieldType entity.ContentTypeFieldType) []entity.ViewFilterOperator {
	ops := viewFieldTypes[fieldType].operators
	if len(ops) == 0 {
		return nil
	}
	return slices.Clone(ops)
}

// validOperatorsForBuiltin - same idea as validOperatorsFor, for
// ViewFilterBuiltinField instead of a generic FieldStorageDef.
func validOperatorsForBuiltin(field entity.ViewFilterBuiltinField) []entity.ViewFilterOperator {
	switch field {
	case entity.ViewFilterBuiltinStatus, entity.ViewFilterBuiltinOnModeration, entity.ViewFilterBuiltinPromote, entity.ViewFilterBuiltinSticky:
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq}
	case entity.ViewFilterBuiltinCreatedBy:
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorIn}
	case entity.ViewFilterBuiltinTitle:
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorContains}
	case entity.ViewFilterBuiltinContentType:
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorIn}
	case entity.ViewFilterBuiltinSharesTermsWithCurrentNode, entity.ViewFilterBuiltinSimilarToCurrentNode:
		// Always Contextual; Value is the optional vocabulary ID.
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq}
	case entity.ViewFilterBuiltinHasCurrentTerm:
		// Always Contextual (enforced in validateFilter): Operator/Value are
		// never read, the term comes from ViewRunParams.ContextTermID. Eq is
		// the single choice the admin form shows.
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq}
	// --- base_entity == comment: same operators as the content analogs.
	case entity.ViewFilterBuiltinCommentStatus:
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq}
	case entity.ViewFilterBuiltinCommentCreatedBy:
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorIn}
	case entity.ViewFilterBuiltinCommentSubject, entity.ViewFilterBuiltinCommentContent:
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorContains}
	case entity.ViewFilterBuiltinCommentNodeType:
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorIn}
	case entity.ViewFilterBuiltinCommentIsReply:
		return []entity.ViewFilterOperator{entity.ViewFilterOperatorEq}
	default:
		return nil
	}
}

// builtins - what a View of one base entity may show, sort and filter
// by besides the generic fields, in the order the admin offers them.
type builtins struct {
	fields  []entity.ViewFieldBuiltinField
	sorts   []entity.ViewSortBuiltin
	filters []entity.ViewFilterBuiltinField
}

var (
	contentBuiltins = builtins{
		fields: []entity.ViewFieldBuiltinField{
			entity.ViewFieldBuiltinTitle,
			entity.ViewFieldBuiltinContentType,
			entity.ViewFieldBuiltinCreatedAt,
			entity.ViewFieldBuiltinViewCount,
			entity.ViewFieldBuiltinCommentCount,
			entity.ViewFieldBuiltinReadingTime,
			entity.ViewFieldBuiltinAuthor,
			entity.ViewFieldBuiltinLink,
		},
		sorts: []entity.ViewSortBuiltin{
			entity.ViewSortBuiltinCreatedAt,
			entity.ViewSortBuiltinViews,
			entity.ViewSortBuiltinTitle,
			entity.ViewSortBuiltinUpdatedAt,
			entity.ViewSortBuiltinViewsToday,
			entity.ViewSortBuiltinViewsWeek,
			entity.ViewSortBuiltinViewsMonth,
			entity.ViewSortBuiltinRandom,
			entity.ViewSortBuiltinSticky,
		},
		// ViewFilterBuiltinContentType is the only way to scope a View to
		// content types (see entity.ViewDisplayDef).
		filters: []entity.ViewFilterBuiltinField{
			entity.ViewFilterBuiltinContentType,
			entity.ViewFilterBuiltinStatus,
			entity.ViewFilterBuiltinOnModeration,
			entity.ViewFilterBuiltinPromote,
			entity.ViewFilterBuiltinSticky,
			entity.ViewFilterBuiltinCreatedBy,
			entity.ViewFilterBuiltinTitle,
			entity.ViewFilterBuiltinSimilarToCurrentNode,
			entity.ViewFilterBuiltinSharesTermsWithCurrentNode,
			entity.ViewFilterBuiltinHasCurrentTerm,
		},
	}
	commentBuiltins = builtins{
		fields: []entity.ViewFieldBuiltinField{
			entity.ViewFieldBuiltinCommentSubject,
			entity.ViewFieldBuiltinCommentContent,
			entity.ViewFieldBuiltinCommentAuthor,
			entity.ViewFieldBuiltinCommentCreatedAt,
			entity.ViewFieldBuiltinCommentNodeTitle,
		},
		sorts: []entity.ViewSortBuiltin{
			entity.ViewSortBuiltinCreatedAt,
			entity.ViewSortBuiltinCommentSubject,
		},
		// comment_node_type - the parent content's type (the comment's
		// analog of ViewFilterBuiltinContentType).
		filters: []entity.ViewFilterBuiltinField{
			entity.ViewFilterBuiltinCommentNodeType,
			entity.ViewFilterBuiltinCommentStatus,
			entity.ViewFilterBuiltinCommentCreatedBy,
			entity.ViewFilterBuiltinCommentSubject,
			entity.ViewFilterBuiltinCommentContent,
			entity.ViewFilterBuiltinCommentIsReply,
		},
	}
)

func builtinsOf(view entity.ViewDisplayDef) builtins {
	if view.IsComment() {
		return commentBuiltins
	}
	return contentBuiltins
}

// FieldBuiltins - the builtin fields view's base entity (content or
// comments) has, in the order the admin offers them.
func FieldBuiltins(view entity.ViewDisplayDef) []entity.ViewFieldBuiltinField {
	return builtinsOf(view).fields
}

// SortBuiltins - the builtin sorts view's base entity has, in the
// admin's order.
func SortBuiltins(view entity.ViewDisplayDef) []entity.ViewSortBuiltin {
	return builtinsOf(view).sorts
}

// FilterBuiltins - the builtin filters view's base entity has, in the
// admin's order.
func FilterBuiltins(view entity.ViewDisplayDef) []entity.ViewFilterBuiltinField {
	return builtinsOf(view).filters
}

// FieldBuiltinLabel - a builtin field's name in ctx's language: the
// admin's for it, and a list's column when the View field has no label
// of its own (the content link's is its text instead, "Read more":
// NodeLinkFieldInstance).
func FieldBuiltinLabel(ctx context.Context, b entity.ViewFieldBuiltinField) string {
	switch b {
	case entity.ViewFieldBuiltinTitle:
		return i18n.T(ctx, "Title")
	case entity.ViewFieldBuiltinContentType:
		return i18n.TC(ctx, "view field", "Content type")
	case entity.ViewFieldBuiltinCreatedAt:
		return i18n.T(ctx, "Creation date")
	case entity.ViewFieldBuiltinViewCount:
		return i18n.T(ctx, "Page views")
	case entity.ViewFieldBuiltinCommentCount:
		return i18n.T(ctx, "Comments")
	case entity.ViewFieldBuiltinReadingTime:
		return i18n.T(ctx, "Reading time")
	case entity.ViewFieldBuiltinAuthor, entity.ViewFieldBuiltinCommentAuthor:
		return i18n.T(ctx, "Author")
	case entity.ViewFieldBuiltinLink:
		return i18n.T(ctx, "Link to content")
	case entity.ViewFieldBuiltinCommentSubject:
		return i18n.T(ctx, "Subject")
	case entity.ViewFieldBuiltinCommentContent:
		return i18n.T(ctx, "Text")
	case entity.ViewFieldBuiltinCommentCreatedAt:
		return i18n.T(ctx, "Date")
	case entity.ViewFieldBuiltinCommentNodeTitle:
		return i18n.T(ctx, "Content item")
	}
	return string(b)
}

// FieldBuiltinLabels - FieldBuiltinLabel of every builtin field, for
// templates that print them with index.
func FieldBuiltinLabels(ctx context.Context) map[entity.ViewFieldBuiltinField]string {
	out := map[entity.ViewFieldBuiltinField]string{}
	for _, all := range []builtins{contentBuiltins, commentBuiltins} {
		for _, b := range all.fields {
			out[b] = FieldBuiltinLabel(ctx, b)
		}
	}
	return out
}

// SortableFieldType (and FilterableFieldType/DisplayableFieldType/
// GlossaryFieldType) - a View's sort, filter, field or glossary may
// target a field of the type; FilterOperators/
// FilterBuiltinOperators - the operators a filter by the type or the
// builtin may use. The service enforces them; the admin offers only these.
func SortableFieldType(ft entity.ContentTypeFieldType) bool {
	return viewFieldTypes[ft].sortable
}

// FilterableFieldType - a View's filter may target a field of the type.
func FilterableFieldType(ft entity.ContentTypeFieldType) bool {
	return len(viewFieldTypes[ft].operators) > 0
}

// DisplayableFieldType - a View's field may show a field of the type.
func DisplayableFieldType(ft entity.ContentTypeFieldType) bool {
	return viewFieldTypes[ft].shown
}

// GlossaryFieldType - a View's glossary may group by a field of the type.
func GlossaryFieldType(ft entity.ContentTypeFieldType) bool {
	return viewFieldTypes[ft].glossary
}

// FilterOperators - the operators a filter by a field of the type may
// use (none - not filterable).
func FilterOperators(ft entity.ContentTypeFieldType) []entity.ViewFilterOperator {
	return validOperatorsFor(ft)
}

// FilterBuiltinOperators - the operators a filter by the builtin may use.
func FilterBuiltinOperators(b entity.ViewFilterBuiltinField) []entity.ViewFilterOperator {
	return validOperatorsForBuiltin(b)
}

// FieldBuiltinAllowed - a ViewFieldDef may show the builtin (the
// admin offers only those).
func FieldBuiltinAllowed(b entity.ViewFieldBuiltinField) bool {
	return slices.Contains(contentBuiltins.fields, b) || slices.Contains(commentBuiltins.fields, b)
}

// FilterBuiltinAllowed - a ViewFilterDef may filter by the builtin.
func FilterBuiltinAllowed(b entity.ViewFilterBuiltinField) bool {
	return validOperatorsForBuiltin(b) != nil
}

// SortBuiltinAllowed - a ViewSortDef may sort by the builtin.
func SortBuiltinAllowed(b entity.ViewSortBuiltin) bool {
	return slices.Contains(contentBuiltins.sorts, b) || slices.Contains(commentBuiltins.sorts, b)
}

// ErrFieldTypeNotDisplayable - CreateViewField targets a
// FieldStorageDef whose FieldType can never be displayed (group).
var ErrFieldTypeNotDisplayable = errors.New("field type can't be displayed on a view")

// ErrFieldLinkTargetUnsupported - CreateViewField/UpdateViewField
// sets LinkTo to something other than entity.ViewFieldLinkNone/
// Content/File.
var ErrFieldLinkTargetUnsupported = errors.New("unsupported link target")
