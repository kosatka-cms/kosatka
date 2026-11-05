//go:build dbintegration

package ratings

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// insertRow runs an INSERT (? placeholders) of a core row the test needs
// and returns its ID.
func insertRow(t *testing.T, db testkit.DB, query string, args ...any) int {
	t.Helper()
	id, err := db.Dialect.InsertID(context.Background(), db.DB, db.Dialect.Rebind(query), args...)
	if err != nil {
		t.Fatal(err)
	}
	return int(id)
}

// TestRatingsLifecycle - the settings (the defaults without the row),
// the rated types, a user's and a guest's votes (their averages kept as
// shares of the scale they were given on), the rules of voting and the
// numbers the Views criteria get.
func TestRatingsLifecycle(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		repo := NewRepository(db.DB)
		s := NewService(repo)
		user := insertRow(t, db, "INSERT INTO users (username, slug, password_hash) VALUES (?, ?, ?)", "ann", "ann", "")
		sea := modulekit.Node{ID: insertRow(t, db, "INSERT INTO nodes (type, title, status) VALUES (?, ?, ?)", "article", "Sea", 1), Type: "article"}
		lake := modulekit.Node{ID: insertRow(t, db, "INSERT INTO nodes (type, title, status) VALUES (?, ?, ?)", "article", "Lake", 1), Type: "article"}
		page := modulekit.Node{ID: insertRow(t, db, "INSERT INTO nodes (type, title, status) VALUES (?, ?, ?)", "page", "About", 1), Type: "page"}
		ann, guest := Voter{UserID: user}, Voter{Guest: "a1b2"}

		if st, err := s.Settings(ctx); err != nil || st.Scale != DefaultScale || !st.AllowChange {
			t.Fatalf("default settings: %+v %v", st, err)
		}
		if err := s.Vote(ctx, sea, ann, 4); !errors.Is(err, ErrNotRated) {
			t.Errorf("a vote for a type not rated: %v", err)
		}
		if err := s.SetRatedTypes(ctx, []string{"article", "page"}, []string{"article"}); err != nil {
			t.Fatal(err)
		}
		if types, err := s.RatedTypes(ctx); err != nil || !slices.Equal(types, []string{"article"}) {
			t.Errorf("rated types: %v %v", types, err)
		}
		if err := s.Vote(ctx, page, ann, 4); !errors.Is(err, ErrNotRated) {
			t.Errorf("a vote for a page: %v", err)
		}
		if err := s.Vote(ctx, sea, ann, 6); !errors.Is(err, ErrScore) {
			t.Errorf("6 of 5 stars: %v", err)
		}

		// Ann: 4 of 5 for the sea, then 10 of 10 when the scale is 10; a
		// guest: 3 of 5 for the sea, 5 of 5 for the lake.
		for _, v := range []struct {
			node  modulekit.Node
			voter Voter
			score int
		}{{sea, ann, 4}, {sea, guest, 3}, {lake, guest, 5}} {
			if err := s.Vote(ctx, v.node, v.voter, v.score); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.SaveSettings(ctx, Settings{Scale: 10, AllowChange: true}); err != nil {
			t.Fatal(err)
		}
		if err := s.Vote(ctx, lake, ann, 10); err != nil {
			t.Fatal(err)
		}
		stats, err := repo.Stats(ctx, []int{sea.ID, lake.ID, page.ID})
		if err != nil {
			t.Fatal(err)
		}
		if st := stats[sea.ID]; st.Count != 2 || math.Abs(st.Share-0.7) > 1e-9 {
			t.Errorf("the sea: %+v, want 2 votes, share 0.7", st)
		}
		if st := stats[lake.ID]; st.Count != 2 || st.Share != 1 {
			t.Errorf("the lake: %+v", st)
		}
		if _, ok := stats[page.ID]; ok {
			t.Error("the page has no votes")
		}
		if mine, ok, err := repo.VoteOf(ctx, sea.ID, ann); err != nil || !ok || mine.On(10) != 8 {
			t.Errorf("Ann's vote for the sea on 10: %+v %v %v", mine, ok, err)
		}
		if votes, nodes, err := s.TypeTotals(ctx, "article"); err != nil || votes != 4 || nodes != 2 {
			t.Errorf("article totals: %d votes, %d nodes, %v", votes, nodes, err)
		}

		// What the Views criteria get, on the 10-star scale.
		avgs, err := s.Averages(ctx, []int{sea.ID, lake.ID, page.ID})
		if err != nil || math.Abs(avgs[sea.ID]-7) > 1e-9 || avgs[lake.ID] != 10 || len(avgs) != 2 {
			t.Errorf("averages: %v %v", avgs, err)
		}
		counts, err := s.Counts(ctx, []int{sea.ID, page.ID})
		if err != nil || counts[sea.ID] != 2 || counts[page.ID] != 0 || len(counts) != 2 {
			t.Errorf("counts: %v %v", counts, err)
		}
		weighted, err := s.WeightedAverages(ctx, []int{sea.ID, lake.ID}, 2)
		if err != nil || weighted[lake.ID] <= weighted[sea.ID] || weighted[lake.ID] >= 10 {
			t.Errorf("weighted: %v %v", weighted, err)
		}

		// Votes that can't change or be taken back.
		if err := s.SaveSettings(ctx, Settings{Scale: 10}); err != nil {
			t.Fatal(err)
		}
		if err := s.Vote(ctx, sea, ann, 8); err != nil {
			t.Errorf("the same vote again: %v", err)
		}
		if err := s.Vote(ctx, sea, ann, 2); !errors.Is(err, ErrFinal) {
			t.Errorf("a changed vote: %v", err)
		}
		if err := s.Withdraw(ctx, sea, ann); !errors.Is(err, ErrWithdraw) {
			t.Errorf("a vote taken back: %v", err)
		}
		if err := s.SaveSettings(ctx, Settings{Scale: 10, AllowWithdraw: true}); err != nil {
			t.Fatal(err)
		}
		if err := s.Withdraw(ctx, sea, ann); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := repo.VoteOf(ctx, sea.ID, ann); ok {
			t.Error("the vote taken back is there")
		}

		// The widget: the visitor's vote on the scale, what they may do.
		w, ok, err := s.WidgetFor(ctx, lake, guest, true, false)
		if err != nil || !ok || w.Mine != 10 || w.Count != 2 || w.CanVote || !w.CanWithdraw || len(w.Stars) != 10 {
			t.Errorf("the guest's widget of the lake: %+v %v %v", w, ok, err)
		}
		if _, ok, _ := s.WidgetFor(ctx, page, guest, true, false); ok {
			t.Error("a widget for a page")
		}
	})
}
