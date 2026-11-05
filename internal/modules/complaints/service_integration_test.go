//go:build dbintegration

package complaints

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestComplainRefused - a reason not offered, "Other" without a note, the
// visitor's own content, a draft, a type without complaints, comments
// when they take none, an unknown kind: refused, nothing in the queue.
func TestComplainRefused(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		f := newFixture(t, db)
		for _, c := range []struct {
			name, kind, reason, note string
			id                       int
			user                     *modulekit.User
			want                     error
		}{
			{"a reason not offered", KindNode, "Boring", "", f.harbour, f.bob, ErrReason},
			{"Other without a note", KindNode, OtherReason, "  ", f.harbour, f.bob, ErrReason},
			{"own content", KindNode, "Insults", "", f.harbour, f.ann, ErrNotAllowed},
			{"own comment", KindComment, "Insults", "", f.comment, f.ann, ErrNotAllowed},
			{"a draft", KindNode, "Insults", "", f.draft, f.bob, ErrNotFound},
			{"a type without complaints", KindNode, "Insults", "", f.about, f.bob, ErrNotAllowed},
			{"no such content", KindNode, "Insults", "", 9999, f.bob, ErrNotFound},
			{"no such comment", KindComment, "Insults", "", 9999, f.bob, ErrNotFound},
			{"an unknown kind", "user", "Insults", "", f.ann.ID, f.bob, ErrNotFound},
		} {
			if err := f.svc.Complain(ctx, c.user, c.kind, c.id, c.reason, c.note); !errors.Is(err, c.want) {
				t.Errorf("%s: %v, want %v", c.name, err, c.want)
			}
		}
		if err := f.repo.SaveSettings(ctx, Settings{NodeTypes: []string{"article"}, HideAfter: 2}); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Complain(ctx, f.bob, KindComment, f.comment, "Insults", ""); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("comments taking none: %v", err)
		}
		if n, _ := f.repo.CountNew(ctx); n != 0 || len(f.mail.sent) != 0 {
			t.Errorf("refused complaints: %d in the queue, %d e-mails", n, len(f.mail.sent))
		}
	})
}

// TestComplainAndHide - the first complaint about a content item mails
// the site (its words and links), a visitor's second one changes nothing;
// two visitors' unpublish it; "Keep" publishes it again and takes it out
// of the queue; a complaint after that brings it back (mailed again)
// without hiding it - the complaints before the decision don't count.
func TestComplainAndHide(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		f := newFixture(t, db)
		if err := f.svc.Complain(ctx, f.bob, KindNode, f.harbour, "Spam or advertising", ""); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Complain(ctx, f.bob, KindNode, f.harbour, "Insults", "again"); err != nil {
			t.Fatal(err)
		}
		tg := f.target(t, KindNode, f.harbour)
		if tg == nil || tg.Count != 1 || tg.Hidden || tg.AuthorID != f.ann.ID || len(tg.Reasons) != 1 || tg.Reasons[0].Reason != "Spam or advertising" {
			t.Fatalf("after bob's complaints: %+v", tg)
		}
		if len(f.mail.sent) != 1 {
			t.Fatalf("e-mails: %d, want 1", len(f.mail.sent))
		}
		m := f.mail.sent[0]
		if m.key != MailNewComplaint || m.to != "team@example.com" || m.data.Extra["[complaint:about]"] != "\"Harbour\"" ||
			m.data.Extra["[complaint:url]"] != "https://example.com/harbour" || m.data.Extra["[complaint:note]"] != "-" ||
			m.data.Extra["[complaint:queue-url]"] != "https://example.com/admin/content/complaints" {
			t.Errorf("the e-mail: %+v", m)
		}
		if nodeStatus(t, f, f.harbour) != 1 {
			t.Fatal("one complaint unpublished the content")
		}

		if err := f.svc.Complain(ctx, f.cat, KindNode, f.harbour, OtherReason, "Copied from elsewhere"); err != nil {
			t.Fatal(err)
		}
		tg = f.target(t, KindNode, f.harbour)
		if nodeStatus(t, f, f.harbour) != 0 || tg.Count != 2 || !tg.Hidden || len(tg.Notes) != 1 || tg.Notes[0].Text != "Copied from elsewhere" {
			t.Fatalf("two complaints: the content's status %d, %+v", nodeStatus(t, f, f.harbour), tg)
		}
		if n, _ := f.repo.CountNew(ctx); n != 1 || len(f.mail.sent) != 1 {
			t.Errorf("the queue %d, e-mails %d: want 1, 1", n, len(f.mail.sent))
		}

		if err := f.svc.Decide(ctx, f.mod, tg.ID, ResolutionKept); err != nil {
			t.Fatal(err)
		}
		tg = f.target(t, KindNode, f.harbour)
		if nodeStatus(t, f, f.harbour) != 1 || tg.Status != StatusResolved || tg.Resolution != ResolutionKept || tg.ResolvedBy != f.mod.ID || tg.Hidden {
			t.Fatalf("kept: the content's status %d, %+v", nodeStatus(t, f, f.harbour), tg)
		}
		if n, _ := f.repo.CountNew(ctx); n != 0 {
			t.Errorf("the queue after the decision: %d", n)
		}

		if err := f.svc.Complain(ctx, f.dan, KindNode, f.harbour, "Insults", ""); err != nil {
			t.Fatal(err)
		}
		tg = f.target(t, KindNode, f.harbour)
		if tg.Status != StatusNew || tg.Hidden || tg.Resolution != "" || nodeStatus(t, f, f.harbour) != 1 || len(f.mail.sent) != 2 {
			t.Errorf("a complaint after the decision: %+v, status %d, %d e-mails", tg, nodeStatus(t, f, f.harbour), len(f.mail.sent))
		}
	})
}

// TestCommentDecisions - two complaints about a comment put it back to
// waiting for approval (the reason "reported"); "Spam" marks it; another
// comment unpublished goes back to waiting, one deleted is gone; a content
// item's spam is no action; deleting a content item removes its
// complaints.
func TestCommentDecisions(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		f := newFixture(t, db)
		for _, u := range []*modulekit.User{f.bob, f.cat} {
			if err := f.svc.Complain(ctx, u, KindComment, f.comment, "Spam or advertising", ""); err != nil {
				t.Fatal(err)
			}
		}
		c, err := f.deps.Comments.ByID(ctx, f.comment)
		if err != nil || c.Status != modulekit.CommentPending || !strings.HasPrefix(c.HoldReason, modulekit.CommentReasonReported) {
			t.Fatalf("a comment two complained about: %+v %v", c, err)
		}
		if about := f.mail.sent[0].data.Extra["[complaint:about]"]; about != "a comment by ann on \"Harbour\"" {
			t.Errorf("the e-mail's about: %q", about)
		}
		tg := f.target(t, KindComment, f.comment)
		if tg == nil || tg.NodeID != f.harbour || !tg.Hidden {
			t.Fatalf("the comment's row: %+v", tg)
		}
		if err := f.svc.Decide(ctx, f.mod, tg.ID, ResolutionUnpublish+"x"); !errors.Is(err, ErrAction) {
			t.Errorf("an unknown action: %v", err)
		}
		if err := f.svc.Decide(ctx, f.mod, tg.ID, ResolutionSpam); err != nil {
			t.Fatal(err)
		}
		if c, _ := f.deps.Comments.ByID(ctx, f.comment); c.Status != modulekit.CommentSpam {
			t.Errorf("marked spam: %+v", c)
		}

		second := insertRow(t, db, "INSERT INTO comments (content, nid, created_by) VALUES ('Rude', ?, ?)", f.harbour, f.dan.ID)
		third := insertRow(t, db, "INSERT INTO comments (content, nid, created_by) VALUES ('Ruder', ?, ?)", f.harbour, f.dan.ID)
		for _, id := range []int{second, third} {
			if err := f.svc.Complain(ctx, f.bob, KindComment, id, "Insults", ""); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.svc.Decide(ctx, f.mod, f.target(t, KindComment, second).ID, ResolutionUnpublish); err != nil {
			t.Fatal(err)
		}
		if c, _ := f.deps.Comments.ByID(ctx, second); c.Status != modulekit.CommentPending {
			t.Errorf("unpublished comment: %+v", c)
		}
		if err := f.svc.Decide(ctx, f.mod, f.target(t, KindComment, third).ID, ResolutionDeleted); err != nil {
			t.Fatal(err)
		}
		if _, err := f.deps.Comments.ByID(ctx, third); !modulekit.IsNotFound(err) {
			t.Errorf("deleted comment: %v", err)
		}

		if err := f.svc.Complain(ctx, f.bob, KindNode, f.harbour, "Insults", ""); err != nil {
			t.Fatal(err)
		}
		node := f.target(t, KindNode, f.harbour)
		if err := f.svc.Decide(ctx, f.mod, node.ID, ResolutionSpam); !errors.Is(err, ErrAction) {
			t.Errorf("spam for content: %v", err)
		}
		if err := f.repo.DeleteByNode(ctx, f.harbour); err != nil {
			t.Fatal(err)
		}
		if list, total, _ := f.repo.List(ctx, StatusNew, 100, 0); total != 0 || len(list) != 0 {
			t.Errorf("the content deleted, its complaints left: %+v", list)
		}
		if list, _, _ := f.repo.List(ctx, StatusResolved, 100, 0); len(list) != 0 {
			t.Errorf("the content deleted, its resolved complaints left: %+v", list)
		}
	})
}

// TestUnpublishAndDelete - a moderator unpublishes a content item, or
// deletes one (no trash here: deleted for good).
func TestUnpublishAndDelete(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		f := newFixture(t, db)
		other := insertRow(t, db, "INSERT INTO nodes (type, title, slug, status, created_by) VALUES ('article', 'Sails', 'sails', 1, ?)", f.ann.ID)
		for _, id := range []int{f.harbour, other} {
			if err := f.svc.Complain(ctx, f.bob, KindNode, id, "Insults", ""); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.svc.Decide(ctx, f.mod, f.target(t, KindNode, f.harbour).ID, ResolutionUnpublish); err != nil {
			t.Fatal(err)
		}
		if nodeStatus(t, f, f.harbour) != 0 {
			t.Error("unpublished content is published")
		}
		if err := f.svc.Decide(ctx, f.mod, f.target(t, KindNode, other).ID, ResolutionDeleted); err != nil {
			t.Fatal(err)
		}
		if _, err := f.deps.NodeRepo.ByID(ctx, other); !modulekit.IsNotFound(err) {
			t.Errorf("deleted content: %v", err)
		}
		if tg := f.target(t, KindNode, other); tg == nil || tg.Resolution != ResolutionDeleted {
			t.Errorf("deleted content's row: %+v", tg)
		}
		if err := f.svc.Decide(ctx, f.mod, 9999, ResolutionKept); !modulekit.IsNotFound(err) {
			t.Errorf("no such row: %v", err)
		}
	})
}
