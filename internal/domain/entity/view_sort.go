package entity

import (
	"time"
)

// ViewSortBuiltin - the built-in sort a ViewSortDef with a nil
// FieldStorageID applies. "views" sorts by node_counter.total_count, like
// AdminNodesSortViews.
type ViewSortBuiltin string

// The built-in sorts (ViewSortBuiltin).
const (
	ViewSortBuiltinCreatedAt ViewSortBuiltin = "created_at"
	ViewSortBuiltinViews     ViewSortBuiltin = "views"
	// ViewSortBuiltinTitle sorts by node title (alphabetical).
	ViewSortBuiltinTitle ViewSortBuiltin = "title"
	// ViewSortBuiltinUpdatedAt sorts by node updated_at (e.g. for a
	// moderation queue, like AdminNodesSortUpdated).
	ViewSortBuiltinUpdatedAt ViewSortBuiltin = "updated_at"
	// ViewSortBuiltinViewsToday sorts by node_counter.daycount (today's
	// views). RunView also excludes nodes not viewed today at all.
	ViewSortBuiltinViewsToday ViewSortBuiltin = "views_today"
	// ViewSortBuiltinViewsWeek and ViewSortBuiltinViewsMonth sort by the
	// views of the last 7 and 30 days, today included (node_view_days,
	// ViewDays). RunView also excludes nodes not viewed in them.
	ViewSortBuiltinViewsWeek  ViewSortBuiltin = "views_7d"
	ViewSortBuiltinViewsMonth ViewSortBuiltin = "views_30d"
	// ViewSortBuiltinCommentSubject - comment base entity only: sorts by
	// Comment.Subject. ViewSortBuiltinCreatedAt is reused for
	// Comment.CreatedAt (interpreted by BaseEntity at the query layer).
	ViewSortBuiltinCommentSubject ViewSortBuiltin = "comment_subject"
	// ViewSortBuiltinRandom - a shuffle, not a comparator; Direction is
	// ignored.
	ViewSortBuiltinRandom ViewSortBuiltin = "random"
	// ViewSortBuiltinSticky - Node.Sticky: descending puts the sticky
	// nodes first ("sticky at the top of lists"), the next criteria
	// order each group.
	ViewSortBuiltinSticky ViewSortBuiltin = "sticky"
)

// ViewDays - the days of views a sort counts (ViewSortBuiltinViewsWeek
// 7, ViewSortBuiltinViewsMonth 30), 0 for the other sorts.
func (b ViewSortBuiltin) ViewDays() int {
	switch b {
	case ViewSortBuiltinViewsWeek:
		return 7
	case ViewSortBuiltinViewsMonth:
		return 30
	}
	return 0
}

// ViewSortDef - one sort criterion of a ViewDisplayDef. Criteria apply
// in SortOrder: the first is the primary key, later ones break ties,
// like a multi-column ORDER BY.
type ViewSortDef struct {
	ID            int
	ViewDisplayID int
	// FieldStorageID nil means this criterion sorts by BuiltinField.
	// Only text/number/date fields are sortable
	// (views.SortableFieldType).
	FieldStorageID *int
	BuiltinField   ViewSortBuiltin
	Direction      string // "asc" | "desc"
	// SortOrder - this criterion's position in the list; SortOrder==0
	// is the primary sort key.
	SortOrder int
	// Params - a module's criterion's settings in this View
	// (ViewModuleSort.Params); nil for the others.
	Params map[string]string `json:"Params,omitempty"`
	// ExposedKey - the criterion offered to visitors: its value of the
	// page's ViewSortParam; empty - not offered (see
	// ViewSortDefs.WithChoice).
	ExposedKey string `json:"ExposedKey,omitempty"`
	// ExposedLabel - the option's label in the visitors' choice.
	ExposedLabel string `json:"ExposedLabel,omitempty"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ViewSortDefs - a list of a display's sort criteria.
type ViewSortDefs []ViewSortDef

// ViewSortParam - the query parameter of a View page naming the visitor's
// choice of its exposed sort criteria (ViewSortDef.ExposedKey).
const ViewSortParam = "sort"

// The other query parameters a View page reads itself: the page of the
// list, the glossary's letter. No exposed filter takes them (nor sort).
const (
	ViewPageParam   = "page"
	ViewLetterParam = "letter"
)

// Exposed - the criterion is offered to visitors.
func (s ViewSortDef) Exposed() bool { return s.ExposedKey != "" }

// Exposed - the criteria offered to visitors, in order.
func (ss ViewSortDefs) Exposed() ViewSortDefs {
	var out ViewSortDefs
	for _, s := range ss {
		if s.Exposed() {
			out = append(out, s)
		}
	}
	return out
}

// WithChoice - the criteria a run applies (ss in order) when the visitor
// chose key: the exposed criteria are alternatives - one applies, the one
// with that ExposedKey (none such, or key empty - the first exposed), in
// the place of the first exposed; the others keep their places, so a
// "sticky first" criterion before them still leads.
func (ss ViewSortDefs) WithChoice(key string) ViewSortDefs {
	exposed := ss.Exposed()
	if len(exposed) == 0 {
		return ss
	}
	chosen := exposed[0]
	for _, s := range exposed {
		if s.ExposedKey == key {
			chosen = s
			break
		}
	}
	out := make(ViewSortDefs, 0, len(ss)-len(exposed)+1)
	placed := false
	for _, s := range ss {
		switch {
		case !s.Exposed():
			out = append(out, s)
		case !placed:
			out = append(out, chosen)
			placed = true
		}
	}
	return out
}

// FieldStorageIDValue - FieldStorageID, 0 when the criterion targets a
// builtin.
func (s ViewSortDef) FieldStorageIDValue() int {
	if s.FieldStorageID == nil {
		return 0
	}
	return *s.FieldStorageID
}

// SortBySortOrder - see ViewFilterDefs.SortBySortOrder.
func (ss ViewSortDefs) SortBySortOrder() ViewSortDefs {
	sorted := make(ViewSortDefs, len(ss))
	copy(sorted, ss)
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
