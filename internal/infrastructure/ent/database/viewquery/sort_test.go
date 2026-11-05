package viewquery

import (
	"slices"
	"testing"
	"time"
)

func TestSortIDsByKeysSingleKey(t *testing.T) {
	t.Run("string key", func(t *testing.T) {
		ids := []int{1, 2, 3}
		values := map[int]string{1: "b", 2: "a", 3: "c"}

		got := sortIDsByKeys(ids, []sortKey{{str: values}})
		want := []int{2, 1, 3}
		if !slices.Equal(got, want) {
			t.Fatalf("asc: got %v, want %v", got, want)
		}

		got = sortIDsByKeys(ids, []sortKey{{str: values, desc: true}})
		want = []int{3, 1, 2}
		if !slices.Equal(got, want) {
			t.Fatalf("desc: got %v, want %v", got, want)
		}
	})

	t.Run("number key", func(t *testing.T) {
		ids := []int{1, 2, 3}
		values := map[int]float64{1: 2.5, 2: 1.5, 3: 3.5}

		got := sortIDsByKeys(ids, []sortKey{{num: values, isNum: true}})
		want := []int{2, 1, 3}
		if !slices.Equal(got, want) {
			t.Fatalf("asc: got %v, want %v", got, want)
		}
	})

	t.Run("time key", func(t *testing.T) {
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		ids := []int{1, 2, 3}
		values := map[int]time.Time{
			1: base.AddDate(0, 0, 2),
			2: base,
			3: base.AddDate(0, 0, 1),
		}

		got := sortIDsByKeys(ids, []sortKey{{tim: values, isTime: true}})
		want := []int{2, 3, 1}
		if !slices.Equal(got, want) {
			t.Fatalf("asc: got %v, want %v", got, want)
		}
	})
}

// TestSortIDsByKeysTieBreak: a second key only matters when the first-key
// values tie.
func TestSortIDsByKeysTieBreak(t *testing.T) {
	// The primary key ties 1&2 and, separately, 3&4 - the numeric secondary
	// key must break each tie without disturbing the primary grouping.
	ids := []int{1, 2, 3, 4}
	primary := map[int]string{1: "a", 2: "a", 3: "b", 4: "b"}
	secondary := map[int]float64{1: 20, 2: 10, 3: 5, 4: 15}

	got := sortIDsByKeys(ids, []sortKey{
		{str: primary},
		{num: secondary, isNum: true},
	})
	want := []int{2, 1, 3, 4}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v (primary groups a-then-b, secondary ascending within each group)", got, want)
	}
}
