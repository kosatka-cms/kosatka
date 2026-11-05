package viewquery

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func TestBuildCommentFilterPredicatesContextualAuthor(t *testing.T) {
	filters := entity.ViewFilterDefs{
		{BuiltinField: entity.ViewFilterBuiltinCommentCreatedBy, Operator: entity.ViewFilterOperatorEq, Contextual: true},
		{BuiltinField: entity.ViewFilterBuiltinCommentStatus, Operator: entity.ViewFilterOperatorEq, Value: "1"},
	}
	// The profile's user → one predicate per filter.
	preds, impossible, err := buildCommentFilterPredicates(filters, nil, 42)
	if err != nil || impossible || len(preds) != 2 {
		t.Fatalf("with context: %d predicates, impossible=%v, err=%v", len(preds), impossible, err)
	}
	// No user on the page → empty result, not unfiltered.
	if _, impossible, err := buildCommentFilterPredicates(filters, nil, 0); err != nil || !impossible {
		t.Fatalf("without context: impossible=%v, err=%v", impossible, err)
	}
}
