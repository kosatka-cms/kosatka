//go:build dbintegration

package polls

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/polls/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestPollRepositoryTransactions - a poll is created and updated whole:
// a failing option (too long a label) leaves no node or poll behind on
// create, and the previous title and options on update.
func TestPollRepositoryTransactions(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		d := testkit.Deps(db)
		repo := NewPollRepository(db.DB, d.NodeRepo, d.Users)
		if _, err := db.DB.Exec(`INSERT INTO users (username, slug, password_hash) VALUES ('author', 'users/author', '')`); err != nil {
			t.Fatal(err)
		}
		var authorID int
		if err := db.DB.QueryRow(`SELECT id FROM users WHERE username = 'author'`).Scan(&authorID); err != nil {
			t.Fatal(err)
		}
		poll := func(title string, options ...string) *Poll {
			p := &Poll{MaxAnswers: 1, Node: modulekit.Node{Title: title, Slug: "poll/" + strings.ToLower(title), Status: 1, CreatedBy: modulekit.User{ID: authorID}}}
			for _, o := range options {
				p.Options = append(p.Options, PollOption{Label: o})
			}
			return p
		}
		long := strings.Repeat("x", 600)

		if _, err := repo.Create(ctx, poll("Broken", "Yes", long)); err == nil {
			t.Fatal("a too long option must fail")
		}
		var n int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT COUNT(*) FROM nodes WHERE type = ?`), string(manifest.ContentType)).Scan(&n); err != nil || n != 0 {
			t.Fatalf("a failed create left %d poll nodes %v", n, err)
		}

		created, err := repo.Create(ctx, poll("Best", "Yes", "No"))
		if err != nil || len(created.Options) != 2 {
			t.Fatalf("create: %+v %v", created, err)
		}
		upd := *created
		upd.Node.Title = "Changed"
		upd.Options = append(PollOptions{}, created.Options...)
		upd.Options = append(upd.Options, PollOption{Label: long})
		if _, err := repo.Update(ctx, &upd); err == nil {
			t.Fatal("a too long new option must fail")
		}
		got, err := repo.ByNodeID(ctx, created.Node.ID)
		if err != nil || got.Node.Title != "Best" || len(got.Options) != 2 {
			t.Errorf("after a failed update: %+v %v", got, err)
		}
		if bySlug, err := repo.BySlug(ctx, "poll/best"); err != nil || bySlug.ID != created.ID {
			t.Errorf("by slug: %+v %v", bySlug, err)
		}
		if _, err := repo.BySlug(ctx, "poll/none"); !modulekit.IsNotFound(err) {
			t.Errorf("a missing slug: %v", err)
		}

		// Another poll's option named in an edit isn't touched: the IDs
		// come from the form.
		other, err := repo.Create(ctx, poll("Other", "Red", "Blue"))
		if err != nil {
			t.Fatal(err)
		}
		edit := *created
		edit.Options = PollOptions{created.Options[0], created.Options[1], {ID: other.Options[0].ID, Label: "Taken over"}}
		if _, err := repo.Update(ctx, &edit); err != nil {
			t.Fatal(err)
		}
		if got, err := repo.ByNodeID(ctx, other.Node.ID); err != nil || got.Options[0].Label != "Red" {
			t.Errorf("another poll's option after an edit: %+v %v", got.Options, err)
		}
		if _, err := db.DB.Exec(db.Dialect.Rebind(`DELETE FROM nodes WHERE id = ?`), other.Node.ID); err != nil {
			t.Fatal(err)
		}

		// Votes: the log names the voter (the core's user).
		if saved, err := repo.SaveUserVotes(ctx, created.ID, authorID, []int{created.Options[1].ID}, false); err != nil || !saved {
			t.Fatalf("the author's vote: saved %v, %v", saved, err)
		}
		if rows, err := repo.VoteAuditRows(ctx, created.ID); err != nil || len(rows) != 1 || rows[0].Username != "author" || rows[0].OptionLabel != "No" {
			t.Errorf("the vote log: %+v %v", rows, err)
		}
		// The counts: per option and in all, another voter's too.
		if _, err := db.DB.Exec(`INSERT INTO users (username, slug, password_hash) VALUES ('reader', 'users/reader', '')`); err != nil {
			t.Fatal(err)
		}
		var readerID int
		if err := db.DB.QueryRow(`SELECT id FROM users WHERE username = 'reader'`).Scan(&readerID); err != nil {
			t.Fatal(err)
		}
		if saved, err := repo.SaveUserVotes(ctx, created.ID, readerID, []int{created.Options[1].ID}, false); err != nil || !saved {
			t.Fatalf("the reader's vote: saved %v, %v", saved, err)
		}
		if counts, total, err := repo.VoteCounts(ctx, created.ID); err != nil || total != 2 || counts[created.Options[1].ID] != 2 || counts[created.Options[0].ID] != 0 {
			t.Errorf("counts: %v %d %v", counts, total, err)
		}
		// Without a revote a second vote isn't saved; with one it replaces.
		if saved, err := repo.SaveUserVotes(ctx, created.ID, readerID, []int{created.Options[0].ID}, false); err != nil || saved {
			t.Errorf("a second vote without revote: saved %v, %v", saved, err)
		}
		if saved, err := repo.SaveUserVotes(ctx, created.ID, readerID, []int{created.Options[0].ID}, true); err != nil || !saved {
			t.Errorf("a revote: saved %v, %v", saved, err)
		}
		if got, err := repo.UserOptionVotes(ctx, created.ID, readerID); err != nil || len(got) != 1 || got[0] != created.Options[0].ID {
			t.Errorf("the reader's votes after the revote: %v %v", got, err)
		}
		// One user's first votes at once (a double click): one is saved,
		// the others find it - never two sets.
		if _, err := db.DB.Exec(`INSERT INTO users (username, slug, password_hash) VALUES ('clicker', 'users/clicker', '')`); err != nil {
			t.Fatal(err)
		}
		var clickerID int
		if err := db.DB.QueryRow(`SELECT id FROM users WHERE username = 'clicker'`).Scan(&clickerID); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		results := make([]bool, 6)
		for i := range results {
			wg.Add(1)
			go func() {
				defer wg.Done()
				saved, err := repo.SaveUserVotes(ctx, created.ID, clickerID, []int{created.Options[i%2].ID}, false)
				if err != nil {
					t.Errorf("a vote at once: %v", err)
				}
				results[i] = saved
			}()
		}
		wg.Wait()
		savedOnce := 0
		for _, ok := range results {
			if ok {
				savedOnce++
			}
		}
		if got, err := repo.UserOptionVotes(ctx, created.ID, clickerID); err != nil || savedOnce != 1 || len(got) != 1 {
			t.Errorf("votes at once: %d saved, the user's votes %v %v", savedOnce, got, err)
		}
		// The node gone, its poll and votes go with it (the module's keys
		// to the core's nodes: ON DELETE CASCADE).
		if _, err := db.DB.Exec(db.Dialect.Rebind(`DELETE FROM nodes WHERE id = ?`), created.Node.ID); err != nil {
			t.Fatal(err)
		}
		var polls, votes int
		if err := db.DB.QueryRow(`SELECT (SELECT COUNT(*) FROM polls), (SELECT COUNT(*) FROM poll_votes)`).Scan(&polls, &votes); err != nil || polls != 0 || votes != 0 {
			t.Errorf("after the node's deletion: %d polls, %d votes %v", polls, votes, err)
		}
	})
}

// TestPollServiceActions - a poll created is "Content created" in the
// action log; one published and given another address on save - those
// two, with the old and the new address.
func TestPollServiceActions(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		d := testkit.Deps(db)
		svc := NewPollService(NewPollRepository(db.DB, d.NodeRepo, d.Users), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		if _, err := db.DB.Exec(`INSERT INTO users (username, slug, password_hash) VALUES ('author', 'users/author', '')`); err != nil {
			t.Fatal(err)
		}
		var authorID int
		if err := db.DB.QueryRow(`SELECT id FROM users WHERE username = 'author'`).Scan(&authorID); err != nil {
			t.Fatal(err)
		}
		got := testkit.CaptureActions(t)
		created, err := svc.Create(ctx, &Poll{MaxAnswers: 1, Options: PollOptions{{Label: "Yes"}, {Label: "No"}},
			Node: modulekit.Node{Title: "Best", Slug: "poll/best", CreatedBy: modulekit.User{ID: authorID}}})
		if err != nil {
			t.Fatal(err)
		}
		upd := *created
		upd.Node.Status, upd.Node.Slug = 1, "poll/better"
		if _, err := svc.Update(ctx, &upd); err != nil {
			t.Fatal(err)
		}
		if len(*got) != 3 || (*got)[0].Type != "node.create" || (*got)[1].Type != "node.publish" || (*got)[2].Type != "node.address" {
			t.Fatalf("actions: %+v", *got)
		}
		if c := (*got)[2].Changes; len(c) != 1 || c[0].From != "/poll/best" || c[0].To != "/poll/better" {
			t.Errorf("the address: %+v", c)
		}
	})
}
