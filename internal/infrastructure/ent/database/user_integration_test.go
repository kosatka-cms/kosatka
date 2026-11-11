//go:build dbintegration

package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestUserSearchIgnoresCase - PostgreSQL compares case-sensitively, the
// search mustn't.
func TestUserSearchIgnoresCase(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		seedUser(t, db)
		client := entClient(db)
		users, err := database.NewUserRepository(client.User, nil).Search(context.Background(), "ALI", 5)
		if err != nil || users == nil || len(*users) != 1 {
			t.Errorf("search: %+v %v", users, err)
		}
	})
}

// TestUserSearchActiveByName - a member's search: active users by name,
// whatever the case; not by e-mail, not a blocked or pending account.
func TestUserSearchActiveByName(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		seedUser(t, db)
		seedRow(t, db, `INSERT INTO users (id, username, slug, password_hash, email, status) VALUES
			(2, 'alina', 'users/alina', '', 'alina@example.com', 0), (3, 'alfred', 'users/alfred', '', 'boat@example.com', 2)`)
		repo := database.NewUserRepository(entClient(db).User, nil)
		ctx := context.Background()
		for q, want := range map[string]int{"ALI": 1, "example.com": 0, "alfred": 0} {
			if users, err := repo.SearchActiveByName(ctx, q, nil, 5); err != nil || users == nil || len(*users) != want {
				t.Errorf("%q: %+v %v, want %d", q, users, err, want)
			}
		}
	})
}

// TestUserByIDs - the users with the ids asked for, by id; an unknown id
// is absent, no ids - none.
func TestUserByIDs(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		users := database.NewUserRepository(entClient(db).User, nil)
		var ids []int
		for _, name := range []string{"ann", "bob"} {
			u, err := users.Create(ctx, entity.User{Username: name, Slug: "users/" + name, Email: name + "@example.com", PasswordHash: "x", Status: entity.UserStatusActive})
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, u.ID)
		}
		got, err := users.ByIDs(ctx, append(ids, 999999))
		if err != nil || len(got) != 2 || got[ids[0]].Username != "ann" || got[ids[1]].Slug != "users/bob" {
			t.Errorf("by ids: %+v %v", got, err)
		}
		if got, err := users.ByIDs(ctx, nil); err != nil || len(got) != 0 {
			t.Errorf("no ids: %+v %v", got, err)
		}
	})
}

// TestUserEmailStorage - an email is stored trimmed in lower case and
// found whatever the case typed; a user without one saves as NULL (no
// validation error), and a second account can't take an address.
func TestUserEmailStorage(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		client := entClient(db)
		users := database.NewUserRepository(client.User, nil)
		u, err := users.Create(ctx, entity.User{Username: "a", Slug: "users/a", Email: " Alice@Example.COM ", PasswordHash: "x", Status: entity.UserStatusActive})
		if err != nil || u.Email != "alice@example.com" {
			t.Fatalf("create: %+v %v", u, err)
		}
		if got, err := users.Find(ctx, entity.UserFilter{Email: "ALICE@example.com"}); err != nil || got.ID != u.ID {
			t.Errorf("find: %+v %v", got, err)
		}
		u.Email = ""
		if _, err := users.Update(ctx, *u); err != nil {
			t.Fatalf("update without an email: %v", err)
		}
		var email sql.NullString
		if err := db.DB.QueryRow(`SELECT email FROM users WHERE username = 'a'`).Scan(&email); err != nil || email.Valid {
			t.Errorf("stored email: %+v %v, want NULL", email, err)
		}
		if _, err := users.Create(ctx, entity.User{Username: "b", Slug: "users/b", Email: "bob@example.com", PasswordHash: "x"}); err != nil {
			t.Fatal(err)
		}
		if _, err := users.Create(ctx, entity.User{Username: "c", Slug: "users/c", Email: "BOB@example.com", PasswordHash: "x"}); err == nil {
			t.Error("a second account took bob@example.com")
		}
	})
}

// TestUserSearchInRoles - a user field's picker: active users by name of
// the field's roles only; no roles - any.
func TestUserSearchInRoles(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		seedRow(t, db, `INSERT INTO roles (id, name, created_at, updated_at) VALUES (5, 'editor', '2026-01-01 00:00:00', '2026-01-01 00:00:00'), (6, 'author', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`)
		seedRow(t, db, `INSERT INTO users (id, username, slug, password_hash, email, status, role_id) VALUES
			(2, 'marta', 'users/marta', '', 'marta@example.com', 1, 5), (3, 'mark', 'users/mark', '', 'mark@example.com', 1, 6)`)
		repo := database.NewUserRepository(entClient(db).User, nil)
		ctx := context.Background()
		for _, tc := range []struct {
			roles []int
			want  int
		}{{nil, 2}, {[]int{5}, 1}, {[]int{5, 6}, 2}, {[]int{9}, 0}} {
			if users, err := repo.SearchActiveByName(ctx, "mar", tc.roles, 5); err != nil || users == nil || len(*users) != tc.want {
				t.Errorf("roles %v: %+v %v, want %d", tc.roles, users, err, tc.want)
			}
		}
	})
}

// TestUsersWithRoles - the active users of the roles, by id: a blocked
// one, one of another role and one without a role left out; no roles -
// none.
func TestUsersWithRoles(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		seedRow(t, db, `INSERT INTO roles (id, name, created_at, updated_at) VALUES (5, 'editor', '2026-01-01 00:00:00', '2026-01-01 00:00:00'), (6, 'author', '2026-01-01 00:00:00', '2026-01-01 00:00:00'), (7, 'reviewer', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`)
		seedRow(t, db, `INSERT INTO users (id, username, slug, password_hash, email, status, role_id) VALUES
			(4, 'vera', 'users/vera', '', 'vera@example.com', 1, 7), (2, 'marta', 'users/marta', '', 'marta@example.com', 1, 5),
			(3, 'mark', 'users/mark', '', 'mark@example.com', 1, 6), (5, 'gleb', 'users/gleb', '', 'gleb@example.com', 0, 5),
			(6, 'nina', 'users/nina', '', 'nina@example.com', 1, NULL)`)
		repo := database.NewUserRepository(entClient(db).User, nil)
		ctx := context.Background()
		users, err := repo.UsersWithRoles(ctx, []int{5, 7})
		if err != nil {
			t.Fatal(err)
		}
		if len(users) != 2 || users[0].Username != "marta" || users[1].Username != "vera" || users[0].Email != "marta@example.com" {
			t.Errorf("editors and reviewers: %+v", users)
		}
		if users, err := repo.UsersWithRoles(ctx, nil); err != nil || len(users) != 0 {
			t.Errorf("no roles: %+v %v", users, err)
		}
	})
}

// TestUserSessionVersion - a new account's sessions are at version 0;
// ending them raises it, and an edit of the account keeps it.
func TestUserSessionVersion(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		users := database.NewUserRepository(entClient(db).User, nil)
		u, err := users.Create(ctx, entity.User{Username: "ann", Slug: "users/ann", Email: "ann@example.com", PasswordHash: "x", Status: entity.UserStatusActive})
		if err != nil {
			t.Fatal(err)
		}
		if status, version, err := users.SessionStateByID(ctx, u.ID); err != nil || status != entity.UserStatusActive || version != 0 {
			t.Fatalf("a new account: status %d, version %d, %v", status, version, err)
		}
		if v, err := users.BumpSessionVersion(ctx, u.ID); err != nil || v != 1 {
			t.Fatalf("ended: %d, %v", v, err)
		}
		u.Username = "anna"
		if _, err := users.Update(ctx, *u); err != nil {
			t.Fatal(err)
		}
		if _, version, err := users.SessionStateByID(ctx, u.ID); err != nil || version != 1 {
			t.Errorf("after an edit: version %d, %v", version, err)
		}
		if _, _, err := users.SessionStateByID(ctx, 999999); !entity.IsNotFound(err) {
			t.Errorf("no such user: %v", err)
		}
	})
}

// TestUserByIDOncePerRequest - in a request's ctx (entity.WithUserMemo)
// a user is read once: a change behind the repository's back isn't seen,
// one through it is; outside the memo, or in a transaction, every read
// goes to the database.
func TestUserByIDOncePerRequest(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		seedUser(t, db)
		users := database.NewUserRepository(entClient(db).User, nil)
		ctx := entity.WithUserMemo(context.Background())
		if u, err := users.ByID(ctx, 1); err != nil || u.Username != "alice" {
			t.Fatalf("first read: %+v %v", u, err)
		}
		seedRow(t, db, `UPDATE users SET username = 'alicia' WHERE id = 1`)
		if u, _ := users.ByID(ctx, 1); u.Username != "alice" {
			t.Errorf("read again in the request: %q, want the kept alice", u.Username)
		}
		if u, _ := users.ByID(context.Background(), 1); u.Username != "alicia" {
			t.Errorf("read without the memo: %q", u.Username)
		}
		u, _ := users.ByID(context.Background(), 1)
		u.Username, u.PasswordHash = "ann", "hash"
		if _, err := users.Update(ctx, *u); err != nil {
			t.Fatal(err)
		}
		if u, _ := users.ByID(ctx, 1); u.Username != "ann" {
			t.Errorf("after an update in the request: %q", u.Username)
		}
	})
}
