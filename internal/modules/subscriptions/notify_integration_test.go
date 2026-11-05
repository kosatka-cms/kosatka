//go:build dbintegration

package subscriptions

import (
	"context"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// fakeTerms - the terms of every node: ids.
type fakeTerms []int

func (f fakeTerms) FieldTermIDsForNode(context.Context, int) ([]int, error) { return f, nil }

// kinds - userID's notifications as their kinds, oldest first.
func kinds(t *testing.T, repo *Repo, userID int) []string {
	t.Helper()
	list, _, err := repo.Notifications(context.Background(), userID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(list))
	for i, n := range list {
		out[len(list)-1-i] = n.Kind
	}
	return out
}

// TestCommentPosted - a comment tells the author of the comment it
// replies to (a reply, unless they turned the replies off: then as a
// follower) and the content's followers, nobody twice nor its author, who
// then follows the content; a comment on content not public tells nobody;
// a user who unsubscribed isn't subscribed again by commenting. Those who
// turned the e-mails off get theirs not to be mailed.
func TestCommentPosted(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		f := newFixture(t, db)
		deps := testkit.Deps(db)
		repo := NewRepo(db.DB)
		n := NewNotifier(repo, deps.NodeRepo, deps.Comments, fakeTerms(nil))
		post := func(nodeID, author int, parent *int) int {
			t.Helper()
			var id int
			if parent == nil {
				id = insertRow(t, db, "INSERT INTO comments (content, nid, created_by) VALUES ('Ahoy', ?, ?)", nodeID, author)
			} else {
				id = insertRow(t, db, "INSERT INTO comments (content, nid, created_by, parent_id) VALUES ('Ahoy', ?, ?, ?)", nodeID, author, *parent)
			}
			if err := n.CommentPosted(ctx, modulekit.Comment{ID: id, Nid: nodeID, CreatedBy: author, ParentID: parent}); err != nil {
				t.Fatal(err)
			}
			return id
		}
		if err := repo.Subscribe(ctx, f.bob, Target{NodeID: f.harbour}); err != nil {
			t.Fatal(err)
		}
		if err := repo.SaveUserSettings(ctx, f.bob, UserSettings{Mail: MailNever, Replies: true, AutoOwn: true, AutoCommented: true}); err != nil {
			t.Fatal(err)
		}

		first := post(f.harbour, f.cat, nil) // bob is told; cat follows now
		post(f.harbour, f.ann, &first)       // cat: a reply; bob: a comment; ann follows now
		if err := repo.SaveUserSettings(ctx, f.cat, UserSettings{Mail: MailNow, Replies: false, AutoOwn: true, AutoCommented: true}); err != nil {
			t.Fatal(err)
		}
		post(f.harbour, f.ann, &first) // cat, replies off: a comment (a follower); bob: a comment

		if got := kinds(t, repo, f.bob); !slices.Equal(got, []string{KindComment, KindComment, KindComment}) {
			t.Errorf("bob: %v", got)
		}
		if got := kinds(t, repo, f.cat); !slices.Equal(got, []string{KindReply, KindComment}) {
			t.Errorf("cat: %v", got)
		}
		if got := kinds(t, repo, f.ann); len(got) != 0 {
			t.Errorf("ann, the author of the comments after cat's: %v", got)
		}
		if on, _ := repo.Subscribed(ctx, f.ann, Target{NodeID: f.harbour}); !on {
			t.Error("ann doesn't follow the content she commented on")
		}
		waiting, err := repo.WaitingMail(ctx)
		if err != nil || len(waiting[f.bob]) != 0 || len(waiting[f.cat]) != 2 {
			t.Errorf("the waiting e-mail: bob (e-mails off) %d, cat %d, %v", len(waiting[f.bob]), len(waiting[f.cat]), err)
		}

		// A draft: its commenter follows it, nobody is told.
		if err := repo.Subscribe(ctx, f.cat, Target{NodeID: f.draft}); err != nil {
			t.Fatal(err)
		}
		post(f.draft, f.bob, nil)
		if got := kinds(t, repo, f.cat); len(got) != 2 {
			t.Errorf("a comment on a draft told cat: %v", got)
		}

		// bob unsubscribes; his comment doesn't subscribe him again.
		if err := repo.Unsubscribe(ctx, f.bob, Target{NodeID: f.harbour}); err != nil {
			t.Fatal(err)
		}
		post(f.harbour, f.bob, nil)
		if on, _ := repo.Subscribed(ctx, f.bob, Target{NodeID: f.harbour}); on {
			t.Error("commenting subscribed bob again after he unsubscribed")
		}
		post(f.harbour, f.cat, nil)
		if got := kinds(t, repo, f.bob); len(got) != 3 {
			t.Errorf("bob, unsubscribed, was told: %v", got)
		}
	})
}

// TestNodePublished - content becoming public tells the followers of its
// terms once (published again - nobody twice), its author not, who then
// follows its comments.
func TestNodePublished(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		f := newFixture(t, db)
		deps := testkit.Deps(db)
		repo := NewRepo(db.DB)
		n := NewNotifier(repo, deps.NodeRepo, deps.Comments, fakeTerms{f.sailing})
		for _, user := range []int{f.ann, f.bob} {
			if err := repo.Subscribe(ctx, user, Target{TermID: f.sailing}); err != nil {
				t.Fatal(err)
			}
		}
		node := modulekit.Node{ID: f.harbour, Status: 1, CreatedBy: modulekit.User{ID: f.ann}}
		for range 2 {
			if err := n.NodePublished(ctx, node); err != nil {
				t.Fatal(err)
			}
		}
		list, _, err := repo.Notifications(ctx, f.bob, 0, 10)
		if err != nil || len(list) != 1 || list[0].Kind != KindContent || list[0].TermID != f.sailing || list[0].ActorID != f.ann {
			t.Errorf("bob: %+v %v", list, err)
		}
		if got := kinds(t, repo, f.ann); len(got) != 0 {
			t.Errorf("the author was told of her own content: %v", got)
		}
		if on, _ := repo.Subscribed(ctx, f.ann, Target{NodeID: f.harbour}); !on {
			t.Error("the author doesn't follow her content's comments")
		}
	})
}
