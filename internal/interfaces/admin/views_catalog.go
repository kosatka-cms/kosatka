package admin

import (
	"context"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// operatorsAttr renders ops as the space-separated string admin_views.js
// reads from a target <option>'s data-operators attribute.
func operatorsAttr(ops []entity.ViewFilterOperator) string {
	strs := make([]string, len(ops))
	for i, op := range ops {
		strs[i] = string(op)
	}
	return strings.Join(strs, " ")
}

// viewFilterOperators - every entity.ViewFilterOperator, for the filter
// form's "Operator" select; the service narrows which are valid per field.
var viewFilterOperators = []entity.ViewFilterOperator{
	entity.ViewFilterOperatorEq,
	entity.ViewFilterOperatorIn,
	entity.ViewFilterOperatorNotIn,
	entity.ViewFilterOperatorContains,
	entity.ViewFilterOperatorGte,
	entity.ViewFilterOperatorLte,
	entity.ViewFilterOperatorEmpty,
	entity.ViewFilterOperatorNotEmpty,
	entity.ViewFilterOperatorOngoing,
	entity.ViewFilterOperatorNotEnded,
	entity.ViewFilterOperatorEnded,
	entity.ViewFilterOperatorReferencedByCurrent,
	entity.ViewFilterOperatorRelatedToCurrent,
	entity.ViewFilterOperatorIsProfileUser,
}

// viewTagFormats - every entity.ViewTagFormat, for the field form's "Tag
// format" select (taxonomy fields only).
var viewTagFormats = []entity.ViewTagFormat{
	entity.ViewTagFormatPlain,
	entity.ViewTagFormatLinked,
	entity.ViewTagFormatBadges,
}

// sortBuiltinLabels - labels for the Sort card's "Field" column and the
// sort modal's builtin options.
var sortBuiltinLabels = map[entity.ViewSortBuiltin]string{
	entity.ViewSortBuiltinCreatedAt:  i18n.N("Creation date"),
	entity.ViewSortBuiltinViews:      i18n.N("Page views"),
	entity.ViewSortBuiltinTitle:      i18n.N("Name"),
	entity.ViewSortBuiltinUpdatedAt:  i18n.N("Update date"),
	entity.ViewSortBuiltinViewsToday: i18n.N("Page views today"),
	entity.ViewSortBuiltinViewsWeek:  i18n.N("Page views, 7 days"),
	entity.ViewSortBuiltinViewsMonth: i18n.N("Page views, 30 days"),
	entity.ViewSortBuiltinRandom:     i18n.N("Random"),
	entity.ViewSortBuiltinSticky:     i18n.N("Sticky (descending: sticky first)"),
	// --- base_entity == comment ---
	entity.ViewSortBuiltinCommentSubject: i18n.N("Subject"),
}

// translatedLabelMap - a copy of an i18n.N-marked label map with every
// label translated into the request's language, for templates that
// print the labels with index.
func translatedLabelMap[K comparable](ctx context.Context, labels map[K]string) map[K]string {
	out := make(map[K]string, len(labels))
	for k, label := range labels {
		out[k] = i18n.T(ctx, label)
	}
	return out
}
