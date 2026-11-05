//go:build dbintegration

package privatemessages

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// testUsers - the core's users on db (as Wire gets them), with alice (1)
// and bob (2).
func testUsers(t *testing.T, db testkit.DB) modulekit.UserRepository {
	t.Helper()
	if _, err := db.DB.Exec(`INSERT INTO users (id, username, slug, password_hash) VALUES (1, 'alice', 'users/alice', ''), (2, 'bob', 'users/bob', '')`); err != nil {
		t.Fatal(err)
	}
	return testkit.Deps(db).Users
}

// TestMembersOnce - a thread's member is one row: a repeated id at
// creation doesn't add another (the unique index would refuse it); the
// members and the messages' authors are the core's users.
func TestMembersOnce(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		users := testUsers(t, db)
		threads := NewThreadRepo(db.DB, users)
		thread, err := threads.Create(ctx, Thread{Subject: "hi"}, []int{2, 1, 2})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := NewMessageRepo(db.DB, users).Create(ctx, Message{ThreadID: thread.ID, AuthorID: 2, Body: "hello"}); err != nil {
			t.Fatal(err)
		}
		got, err := threads.ByID(ctx, thread.ID)
		if err != nil || len(got.Members) != 2 {
			t.Fatalf("members: %+v %v", got, err)
		}
		for _, m := range got.Members {
			if m.User == nil || m.User.ID != m.UserID {
				t.Errorf("member %d: user %+v", m.UserID, m.User)
			}
		}
		page, err := NewMessageRepo(db.DB, users).List(ctx, MessageFilter{ThreadID: thread.ID}, Pagination{Limit: 10})
		if err != nil || len(page.Messages) != 1 || page.Messages[0].Author == nil || page.Messages[0].Author.Username != "bob" {
			t.Errorf("the thread's messages: %+v %v", page, err)
		}
	})
}

// TestDeleteByAuthor - a user's messages and memberships go, and the
// threads left with no messages; the other member's reply stays.
func TestDeleteByAuthor(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		users := testUsers(t, db)
		// Thread 1: alice and bob talk; thread 2: only alice.
		for _, q := range []string{
			`INSERT INTO pm_threads (id) VALUES (1), (2)`,
			`INSERT INTO pm_thread_members (thread_id, user_id) VALUES (1, 1), (1, 2), (2, 1)`,
			`INSERT INTO pm_messages (id, thread_id, author_id, body) VALUES (1, 1, 1, 'hi'), (2, 2, 1, 'note')`,
			`INSERT INTO pm_messages (id, thread_id, author_id, body) VALUES (3, 1, 2, 'hello')`,
		} {
			if _, err := db.DB.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
		messages := NewMessageRepo(db.DB, users)
		alice := 1
		if n, err := messages.TotalCount(ctx, MessageFilter{AuthorID: &alice}); err != nil || n != 2 {
			t.Errorf("alice's messages: %d %v", n, err)
		}
		if err := messages.DeleteByAuthor(ctx, 1); err != nil {
			t.Fatal(err)
		}
		for q, want := range map[string]int{
			`SELECT COUNT(*) FROM pm_messages`:                           1,
			`SELECT COUNT(*) FROM pm_threads`:                            1,
			`SELECT COUNT(*) FROM pm_thread_members WHERE thread_id = 1`: 1,
		} {
			var n int
			if err := db.DB.QueryRow(q).Scan(&n); err != nil || n != want {
				t.Errorf("%s: %d, want %d %v", q, n, want, err)
			}
		}
	})
}

// TestThreadsLastMessage - every listed thread has its own newest message
// with its author.
func TestThreadsLastMessage(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		users := testUsers(t, db)
		threads := NewThreadRepo(db.DB, users)
		messages := NewMessageRepo(db.DB, users)
		want := map[int]string{}
		for _, subject := range []string{"one", "two", "three"} {
			thread, err := threads.Create(ctx, Thread{Subject: subject}, []int{1, 2})
			if err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{subject + ": first", subject + ": last"} {
				if _, err := messages.Create(ctx, Message{ThreadID: thread.ID, AuthorID: 2, Body: body}); err != nil {
					t.Fatal(err)
				}
			}
			want[thread.ID] = subject + ": last"
		}
		list, err := threads.List(ctx, ThreadFilter{UserID: 1})
		if err != nil || len(*list) != 3 {
			t.Fatalf("list: %+v %v", list, err)
		}
		for _, th := range *list {
			if th.LastMessage == nil || th.LastMessage.Body != want[th.ID] || th.LastMessage.Author == nil || th.LastMessage.Author.Username != "bob" {
				t.Errorf("thread %d: last %+v", th.ID, th.LastMessage)
			}
		}
	})
}
