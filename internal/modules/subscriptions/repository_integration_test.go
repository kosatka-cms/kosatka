//go:build dbintegration

package subscriptions

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// insertRow runs an INSERT on db and returns the new row's id.
func insertRow(t *testing.T, db testkit.DB, query string, args ...any) int {
	t.Helper()
	id, err := db.Dialect.InsertID(context.Background(), db.DB, db.Dialect.Rebind(query), args...)
	if err != nil {
		t.Fatal(err)
	}
	return int(id)
}

// fixture - users ann, bob and cat (active, with addresses), a public
// article "Harbour" by ann, a draft by ann, and the term "Sailing".
type fixture struct {
	ann, bob, cat  int
	harbour, draft int
	sailing        int
}

func newFixture(t *testing.T, db testkit.DB) fixture {
	t.Helper()
	user := func(name string) int {
		return insertRow(t, db, "INSERT INTO users (username, slug, password_hash, email, status) VALUES (?, ?, '', ?, 1)", name, "users/"+name, name+"@example.com")
	}
	f := fixture{ann: user("ann"), bob: user("bob"), cat: user("cat")}
	f.harbour = insertRow(t, db, "INSERT INTO nodes (type, title, slug, status, created_by) VALUES ('article', 'Harbour', 'harbour', 1, ?)", f.ann)
	f.draft = insertRow(t, db, "INSERT INTO nodes (type, title, slug, status, created_by) VALUES ('article', 'Draft', 'draft', 0, ?)", f.ann)
	vocabulary := insertRow(t, db, "INSERT INTO taxonomy_vocabulary (name, slug) VALUES ('Tags', 'tags')")
	f.sailing = insertRow(t, db, "INSERT INTO taxonomy_term_data (name, slug, vid) VALUES ('Sailing', 'sailing', ?)", vocabulary)
	return f
}

// TestSubscriptionsLifecycle - following content and terms: a user's
// unsubscribing keeps a muted row that an automatic subscription doesn't
// revive (subscribing again does), the followers are those not muted,
// each subscription has its own unsubscribe token.
func TestSubscriptionsLifecycle(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		f := newFixture(t, db)
		repo := NewRepo(db.DB)
		harbour, sailing := Target{NodeID: f.harbour}, Target{TermID: f.sailing}

		for _, step := range []func() error{
			func() error { return repo.Subscribe(ctx, f.bob, harbour) },
			func() error { return repo.Subscribe(ctx, f.bob, harbour) }, // twice: one row
			func() error { return repo.AutoSubscribe(ctx, f.cat, harbour) },
			func() error { return repo.Subscribe(ctx, f.cat, sailing) },
		} {
			if err := step(); err != nil {
				t.Fatal(err)
			}
		}
		followers, err := repo.Followers(ctx, harbour)
		if err != nil || len(followers) != 2 || followers[f.bob].Token == "" || followers[f.bob].Token == followers[f.cat].Token {
			t.Fatalf("the followers of Harbour: %+v %v", followers, err)
		}

		if err := repo.Unsubscribe(ctx, f.cat, harbour); err != nil {
			t.Fatal(err)
		}
		if err := repo.AutoSubscribe(ctx, f.cat, harbour); err != nil {
			t.Fatal(err)
		}
		if on, err := repo.Subscribed(ctx, f.cat, harbour); err != nil || on {
			t.Errorf("an automatic subscription brought back cat's unsubscribed one: %v %v", on, err)
		}
		if followers, _ := repo.Followers(ctx, harbour); len(followers) != 1 {
			t.Errorf("the followers after cat unsubscribed: %+v", followers)
		}
		if err := repo.Subscribe(ctx, f.cat, harbour); err != nil {
			t.Fatal(err)
		}
		if on, _ := repo.Subscribed(ctx, f.cat, harbour); !on {
			t.Error("subscribing again didn't turn the subscription back on")
		}

		subs, err := repo.UserSubscriptions(ctx, f.cat)
		if err != nil || len(subs) != 2 || subs[0].TermID != f.sailing || subs[1].NodeID != f.harbour {
			t.Fatalf("cat's subscriptions, newest first: %+v %v", subs, err)
		}
		if s, err := repo.SubscriptionByToken(ctx, subs[0].Token); err != nil || s == nil || s.UserID != f.cat || s.TermID != f.sailing {
			t.Errorf("by token: %+v %v", s, err)
		}
		if s, err := repo.SubscriptionByToken(ctx, "missing"); err != nil || s != nil {
			t.Errorf("an unknown token: %+v %v", s, err)
		}
		if s, err := repo.SubscriptionByID(ctx, f.bob, subs[0].ID); err != nil || s != nil {
			t.Errorf("bob got cat's subscription by its id: %+v %v", s, err)
		}
		if got, err := repo.TermFollowers(ctx, []int{f.sailing}); err != nil || len(got) != 1 || got[f.cat].TermID != f.sailing {
			t.Errorf("the followers of Sailing: %+v %v", got, err)
		}
	})
}

// TestNotificationsReadAndMail - the unread count; opening one marks read
// the unread ones of the same content and kind (a group of comments),
// "mark all" the rest; the waiting e-mail is that of the mailed users
// only; the cleanup deletes the old ones.
func TestNotificationsReadAndMail(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		f := newFixture(t, db)
		repo := NewRepo(db.DB)
		now := time.Now().Truncate(time.Second)
		ns := []Notification{
			{UserID: f.bob, Kind: KindComment, NodeID: f.harbour, ActorID: f.ann, CreatedAt: now.Add(-3 * time.Hour)},
			{UserID: f.bob, Kind: KindComment, NodeID: f.harbour, ActorID: f.cat, CreatedAt: now.Add(-2 * time.Hour)},
			{UserID: f.bob, Kind: KindContent, NodeID: f.harbour, ActorID: f.ann, TermID: f.sailing, CreatedAt: now.Add(-time.Hour)},
			{UserID: f.cat, Kind: KindReply, NodeID: f.harbour, ActorID: f.ann, CreatedAt: now.AddDate(0, 0, -100)},
		}
		if err := repo.AddNotifications(ctx, ns, map[int]bool{f.bob: true}); err != nil {
			t.Fatal(err)
		}
		if n, err := repo.UnreadCount(ctx, f.bob); err != nil || n != 3 {
			t.Errorf("bob's unread: %d %v", n, err)
		}
		list, total, err := repo.Notifications(ctx, f.bob, 0, 10)
		if err != nil || total != 3 || len(list) != 3 || list[0].Kind != KindContent || list[0].TermID != f.sailing {
			t.Fatalf("bob's notifications, newest first: %+v %d %v", list, total, err)
		}
		if n, err := repo.Notification(ctx, f.cat, list[0].ID); err != nil || n != nil {
			t.Errorf("cat got bob's notification: %+v %v", n, err)
		}

		if err := repo.MarkRead(ctx, list[1]); err != nil {
			t.Fatal(err)
		}
		if n, _ := repo.UnreadCount(ctx, f.bob); n != 1 {
			t.Errorf("after opening a comment: %d unread, want the content one only", n)
		}
		if err := repo.MarkAllRead(ctx, f.bob); err != nil {
			t.Fatal(err)
		}
		if n, _ := repo.UnreadCount(ctx, f.bob); n != 0 {
			t.Errorf("after marking all read: %d unread", n)
		}

		waiting, err := repo.WaitingMail(ctx)
		if err != nil || len(waiting) != 1 || len(waiting[f.bob]) != 3 || waiting[f.bob][0].ActorID != f.ann {
			t.Fatalf("the waiting e-mail, bob's only, oldest first: %+v %v", waiting, err)
		}
		if err := repo.setMailState(ctx, []int{waiting[f.bob][0].ID}, mailSent); err != nil {
			t.Fatal(err)
		}
		if waiting, _ := repo.WaitingMail(ctx); len(waiting[f.bob]) != 2 {
			t.Errorf("a sent one still waits: %+v", waiting)
		}

		if n, err := repo.DeleteOlder(ctx, now.Add(-keepNotifications)); err != nil || n != 1 {
			t.Errorf("the cleanup deleted %d %v, want cat's old one", n, err)
		}
	})
}

// TestUserSettings - the defaults for a user who never saved theirs; the
// saved ones read back.
func TestUserSettings(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		f := newFixture(t, db)
		repo := NewRepo(db.DB)
		saved := UserSettings{Mail: MailDaily, Replies: false, AutoOwn: true, AutoCommented: false}
		if err := repo.SaveUserSettings(ctx, f.bob, saved); err != nil {
			t.Fatal(err)
		}
		if err := repo.SaveUserSettings(ctx, f.bob, saved); err != nil { // again: the same row
			t.Fatal(err)
		}
		got, err := repo.UserSettings(ctx, []int{f.ann, f.bob})
		if err != nil || got[f.ann] != DefaultUserSettings() || got[f.bob] != saved {
			t.Errorf("settings: %+v %v", got, err)
		}
	})
}
