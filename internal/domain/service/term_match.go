package service

import (
	"context"
	"log/slog"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// Rules of the form "type + term field + terms" (breadcrumb, SEO) match
// a node when one of its terms in the field, or an ancestor of one, is a
// rule term.

// termsMatch - one of nodeTerms, or one of its ancestors (parents: term
// → parent), satisfies has.
func termsMatch(has func(int) bool, nodeTerms []int, parents map[int]int) bool {
	for _, id := range nodeTerms {
		for depth := 0; id > 0 && depth < 32; depth++ {
			if has(id) {
				return true
			}
			id = parents[id]
		}
	}
	return false
}

// matchRule - the index of the first of rules (in their order) for a
// content item of nodeType: its type, and, when the rule names a taxonomy
// field, one of the item's terms in it is the rule's or under one
// (fieldTerms - the field's vocabulary and the item's terms in it); -1 -
// none. of gives a rule's type, field and terms (SEO and breadcrumb rules).
func matchRule[R any](ctx context.Context, rules []R, of func(R) (nodeType string, fieldStorageID *int, hasTerm func(int) bool), nodeType string, fieldTerms func(storageID int) (int, []int), terms breadcrumbTermsLoader, logger *slog.Logger) int {
	parents := map[int]map[int]int{} // vocabulary → term → parent, loaded on demand
	for i, r := range rules {
		ruleType, field, has := of(r)
		if ruleType != nodeType {
			continue
		}
		if field == nil {
			return i
		}
		vid, nodeTerms := fieldTerms(*field)
		if len(nodeTerms) == 0 {
			continue
		}
		if _, ok := parents[vid]; !ok {
			parents[vid] = loadTermParents(ctx, terms, logger, vid)
		}
		if termsMatch(has, nodeTerms, parents[vid]) {
			return i
		}
	}
	return -1
}

// loadTermParents - term → parent of a vocabulary.
func loadTermParents(ctx context.Context, terms breadcrumbTermsLoader, logger *slog.Logger, vid int) map[int]int {
	out := map[int]int{}
	if terms == nil || vid <= 0 {
		return out
	}
	list, err := terms.TermsByVocabularyID(ctx, vid)
	if err != nil {
		logger.Error("error loading vocabulary terms for a term rule", "error", err, "vid", vid)
		return out
	}
	for _, t := range list {
		out[t.ID] = t.ParentID
	}
	return out
}

// dcFieldTerms - a loaded node's taxonomy field (by storage): its
// vocabulary and the node's terms in it.
func dcFieldTerms(dc entity.DynamicContent, storageID int) (int, []int) {
	for _, f := range dc.Fields {
		if f.FieldStorageID == storageID && f.FieldType == entity.FieldTypeTaxonomy {
			return f.VocabularyID(), nodeTermIDs(dc.Values[f.ID])
		}
	}
	return 0, nil
}

// nodeTermIDs - a taxonomy field value: int (single) or []int (multiple).
func nodeTermIDs(v any) []int {
	switch val := v.(type) {
	case int:
		return []int{val}
	case []int:
		return val
	}
	return nil
}
