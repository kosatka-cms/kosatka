package viewquery

import (
	"math"
	"slices"
	"testing"
	"time"
)

// TestSimilarRankingSortKeys - the similar ranking orders by score, then
// by how many terms are shared, then by the View's own criteria, and the
// last ties keep the input (node ID) order.
func TestSimilarRankingSortKeys(t *testing.T) {
	// weight - similarQuery's weight of a term n published nodes carry.
	weight := func(n int) float64 { return 1 / math.Log(1+float64(n)) }
	ranking := similarRanking{
		// One rare term (3 nodes) beats two common ones (100 nodes each).
		1: {Score: 2 * weight(100), Shared: 2},
		2: {Score: weight(3), Shared: 1},
		// The same score summed in another order: a tie, not float noise.
		3: {Score: weight(5) + weight(50) + weight(7), Shared: 3},
		4: {Score: weight(7) + weight(5) + weight(50), Shared: 3},
		// Equal score, more shared terms first.
		5: {Score: 0.5, Shared: 1},
		6: {Score: 0.5, Shared: 2},
		// A full tie: the View's criterion decides, then the ID.
		7: {Score: 0.1, Shared: 1},
		8: {Score: 0.1, Shared: 1},
		9: {Score: 0.1, Shared: 1},
	}
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	created := map[int]time.Time{3: day, 4: day.AddDate(0, 0, 1), 7: day, 8: day.AddDate(0, 0, 1), 9: day}
	ids := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}

	got := sortIDsByKeys(ids, ranking.sortKeys())
	if want := []int{3, 4, 2, 6, 5, 1, 7, 8, 9}; !slices.Equal(got, want) {
		t.Fatalf("without the View's criteria: got %v, want %v", got, want)
	}
	newestFirst := sortKey{tim: created, isTime: true, desc: true}
	got = sortIDsByKeys(ids, append(ranking.sortKeys(), newestFirst))
	if want := []int{4, 3, 2, 6, 5, 1, 8, 7, 9}; !slices.Equal(got, want) {
		t.Fatalf("with a newest-first criterion: got %v, want %v", got, want)
	}
}
