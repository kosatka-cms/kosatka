package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// TestLastWeight - a new item goes after the heaviest; the first one at 0.
func TestLastWeight(t *testing.T) {
	w := func(n int) int { return n }
	if got := lastWeight([]int{0, 30, 10}, w); got != 40 {
		t.Errorf("after 30: %d", got)
	}
	if got := lastWeight([]int(nil), w); got != 0 {
		t.Errorf("none: %d", got)
	}
}

// TestReorder - the keys get 0, 10, 20… in their order; a failure says
// which list and key.
func TestReorder(t *testing.T) {
	got := map[string]int{}
	set := func(_ context.Context, k string, w int) error { got[k] = w; return nil }
	if err := reorder(context.Background(), nil, []string{"b", "a", "c"}, set, "the list"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]int{"b": 0, "a": 10, "c": 20}) {
		t.Errorf("weights %v", got)
	}
	failing := func(context.Context, string, int) error { return errors.New("db down") }
	if err := reorder(context.Background(), nil, []string{"x"}, failing, "the list"); err == nil || err.Error() != "reordering the list (x): db down" {
		t.Errorf("error %v", err)
	}
}
