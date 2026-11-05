//go:build dbintegration

package forms

import (
	"context"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	formsent "github.com/kosatka-cms/kosatka/internal/modules/forms/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestWebformRepository - a form is stored with its node and elements and
// found by slug; an update keeps an element's machine name, adds and
// removes elements (with their values); a submission keeps one row per
// value; deleting the node takes everything.
func TestWebformRepository(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		d := testkit.Deps(db)
		forms := formsent.NewClient(formsent.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
		repo := NewWebformRepository(d.DB, d.NodeRepo, d.Users)
		if _, err := db.DB.Exec(`INSERT INTO users (username, slug, password_hash) VALUES ('author', 'users/author', '')`); err != nil {
			t.Fatal(err)
		}
		var author modulekit.User
		if err := db.DB.QueryRow(`SELECT id FROM users WHERE username = 'author'`).Scan(&author.ID); err != nil {
			t.Fatal(err)
		}

		closes := time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)
		nodeID, err := repo.Create(ctx, Webform{
			LimitPerUser: true, MaxSubmissions: 100, ClosesAt: &closes,
			Node:        modulekit.Node{Title: "Contact", Slug: "form/contact", Status: 1, CreatedBy: modulekit.User{ID: author.ID}, CommentMode: modulekit.CommentModeHidden},
			Description: "Write to us", SubmitLabel: "Send it", NotifyEmail: "editor@example.test",
			Elements: WebformElements{
				{MachineName: "name", Type: WebformText, Label: "Name", Required: true, PrefillParam: "who"},
				{MachineName: "topics", Type: WebformCheckboxes, Label: "Topics", Options: []string{"Ads", "Errors"},
					VisibleIfElement: "name", VisibleIfValue: "Ann"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		wf, err := repo.BySlug(ctx, "form/contact")
		if err != nil || !wf.LimitPerUser || wf.MaxSubmissions != 100 || wf.ClosesAt == nil || !wf.ClosesAt.Equal(closes) {
			t.Fatalf("limits: %+v %v", wf, err)
		}
		if err != nil || wf.Node.ID != nodeID || wf.SubmitLabel != "Send it" || wf.NotifyEmail != "editor@example.test" || len(wf.Elements) != 2 ||
			wf.Elements[1].MachineName != "topics" || len(wf.Elements[1].Options) != 2 ||
			wf.Elements[1].VisibleIfElement != "name" || wf.Elements[1].VisibleIfValue != "Ann" || wf.Node.CommentMode != modulekit.CommentModeHidden ||
			wf.Elements[0].PrefillParam != "who" || wf.Elements[1].PrefillParam != "" {
			t.Fatalf("stored: %+v %v", wf, err)
		}

		uid := author.ID
		if _, err := repo.AddSubmission(ctx, WebformSubmission{WebformID: wf.ID, UserID: &uid, RemoteIP: "10.0.0.1", SourceURL: "/catalog/tank?size=200",
			Values: map[int][]string{wf.Elements[0].ID: {"Ann"}, wf.Elements[1].ID: {"Ads", "Errors"}}}); err != nil {
			t.Fatal(err)
		}
		if n, _ := forms.WebformSubmissionValue.Query().Count(ctx); n != 3 {
			t.Errorf("values: %d", n)
		}
		if _, err := repo.AddSubmission(ctx, WebformSubmission{WebformID: wf.ID, Values: map[int][]string{wf.Elements[0].ID: {"Bob"}},
			CreatedAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
		if n, err := repo.CountSubmissions(ctx, wf.ID); err != nil || n != 2 {
			t.Errorf("count: %d %v", n, err)
		}
		if sent, err := repo.HasUserSubmission(ctx, wf.ID, author.ID); err != nil || !sent {
			t.Errorf("author's submission: %v %v", sent, err)
		}
		if sent, err := repo.HasUserSubmission(ctx, wf.ID, author.ID+1000); err != nil || sent {
			t.Errorf("another user's: %v %v", sent, err)
		}
		subs, err := repo.Submissions(ctx, wf.ID, 10, 0)
		if err != nil || len(subs) != 2 || subs[0].Values[wf.Elements[0].ID][0] != "Bob" || subs[0].Username != "" ||
			subs[1].Username != "author" || subs[1].UserSlug != "users/author" || len(subs[1].Values[wf.Elements[1].ID]) != 2 {
			t.Errorf("submissions: %+v %v", subs, err)
		}
		if page2, err := repo.Submissions(ctx, wf.ID, 1, 1); err != nil || len(page2) != 1 || page2[0].ID != subs[1].ID {
			t.Errorf("page 2: %+v %v", page2, err)
		}
		one, err := repo.Submission(ctx, wf.ID, subs[1].ID)
		if err != nil || one == nil || one.Username != "author" || len(one.Values[wf.Elements[1].ID]) != 2 || one.SourceURL != "/catalog/tank?size=200" || subs[0].SourceURL != "" {
			t.Errorf("one submission: %+v %v", one, err)
		}
		if other, err := repo.Submission(ctx, wf.ID+1, subs[1].ID); err != nil || other != nil {
			t.Errorf("another form's submission: %+v %v", other, err)
		}
		if newer, older, err := repo.SubmissionNeighbors(ctx, *one); err != nil || newer != subs[0].ID || older != 0 {
			t.Errorf("neighbours of the oldest: %d %d %v", newer, older, err)
		}
		if newer, older, err := repo.SubmissionNeighbors(ctx, subs[0]); err != nil || newer != 0 || older != subs[1].ID {
			t.Errorf("neighbours of the newest: %d %d %v", newer, older, err)
		}
		counts, err := repo.AnswerCounts(ctx, []int{wf.Elements[0].ID, wf.Elements[1].ID}, []int{wf.Elements[1].ID})
		if err != nil || counts.Answered[wf.Elements[0].ID] != 2 || counts.Answered[wf.Elements[1].ID] != 1 ||
			counts.Values[wf.Elements[1].ID]["Ads"] != 1 || counts.Values[wf.Elements[1].ID]["Errors"] != 1 || counts.Values[wf.Elements[0].ID] != nil {
			t.Errorf("answer counts: %+v %v", counts, err)
		}
		if ids, err := repo.SubmissionIDsBefore(ctx, wf.ID, time.Now().Add(30*time.Minute)); err != nil || len(ids) != 1 || ids[0] != subs[1].ID {
			t.Errorf("before: %v %v", ids, err)
		}
		if vs, err := repo.SubmissionValues(ctx, []int{subs[1].ID}, []int{wf.Elements[1].ID}); err != nil || len(vs) != 2 {
			t.Errorf("values of an element: %v %v", vs, err)
		}
		if v, err := repo.SubmissionValue(ctx, wf.ID, subs[1].ID, wf.Elements[1].ID, 1); err != nil || v != "Errors" {
			t.Errorf("one value: %q %v", v, err)
		}
		if v, err := repo.SubmissionValue(ctx, wf.ID+1, subs[1].ID, wf.Elements[1].ID, 1); err != nil || v != "" {
			t.Errorf("another form's value: %q %v", v, err)
		}

		name := wf.Elements[0]
		name.Label = "Your name"
		name.MachineName = "ignored"
		wf.Elements = WebformElements{
			{MachineName: "email", Type: WebformEmail, Label: "E-mail"},
			name,
		}
		wf.Node.Title = "Contact us"
		wf.ClosesAt = nil
		if err := repo.Update(ctx, *wf); err != nil {
			t.Fatal(err)
		}
		got, err := repo.ByNodeID(ctx, nodeID)
		if err != nil || got.Node.Title != "Contact us" || len(got.Elements) != 2 || got.ClosesAt != nil {
			t.Fatalf("updated: %+v %v", got, err)
		}
		if got.Elements[0].MachineName != "email" || got.Elements[1].ID != name.ID || got.Elements[1].MachineName != "name" || got.Elements[1].Label != "Your name" {
			t.Errorf("elements after update: %+v", got.Elements)
		}
		if n, _ := forms.WebformSubmissionValue.Query().Count(ctx); n != 2 {
			t.Errorf("values of the removed element must go: %d left", n)
		}

		// The node's row goes (its foreign keys take the form's rows).
		if _, err := db.DB.ExecContext(ctx, db.Dialect.Rebind(`DELETE FROM nodes WHERE id = ?`), nodeID); err != nil {
			t.Fatal(err)
		}
		for name, n := range map[string]int{
			"forms":       forms.Webform.Query().CountX(ctx),
			"elements":    forms.WebformElement.Query().CountX(ctx),
			"submissions": forms.WebformSubmission.Query().CountX(ctx),
			"values":      forms.WebformSubmissionValue.Query().CountX(ctx),
		} {
			if n != 0 {
				t.Errorf("%s left after the node: %d", name, n)
			}
		}
	})
}
