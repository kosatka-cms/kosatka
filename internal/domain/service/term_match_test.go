package service

import (
	"context"
	"testing"
)

// TestMatchRule - the first rule for the item's type whose term (if it
// asks for one) the item has, in the rules' order; none - -1.
func TestMatchRule(t *testing.T) {
	type rule struct {
		nodeType string
		field    *int
		term     int
	}
	field := 3
	rules := []rule{{"page", nil, 0}, {"article", &field, 5}, {"article", nil, 0}}
	of := func(r rule) (string, *int, func(int) bool) {
		return r.nodeType, r.field, func(id int) bool { return id == r.term }
	}
	with := func(ids ...int) func(int) (int, []int) { return func(int) (int, []int) { return 1, ids } }
	ctx := context.Background()
	if i := matchRule(ctx, rules, of, "article", with(5), nil, nil); i != 1 {
		t.Errorf("the item has the term: %d, want 1", i)
	}
	if i := matchRule(ctx, rules, of, "article", with(7), nil, nil); i != 2 {
		t.Errorf("another term: %d, want the type's catch-all 2", i)
	}
	if i := matchRule(ctx, rules, of, "news", with(5), nil, nil); i != -1 {
		t.Errorf("no rule for the type: %d", i)
	}
}
