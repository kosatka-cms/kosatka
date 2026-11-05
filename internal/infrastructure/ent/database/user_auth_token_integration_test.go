//go:build dbintegration

package database_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestUserAuthTokenRepository - a token is valid while unused and not
// expired, and stays unused when looked at; consumed once; each user's
// newest unused token of a kind and when it expires (an expired one too,
// a used one not).
func TestUserAuthTokenRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedRow(t, db, `INSERT INTO users (id, username, slug, password_hash, created_at) VALUES (2, 'bob', 'bob', '', '2026-01-01 00:00:00')`)
		repo := database.NewUserAuthTokenRepository(db.DB)
		kind := entity.UserAuthTokenKindInvite
		soon := time.Now().Add(time.Hour).Truncate(time.Second)
		hash := func(c string) string { return strings.Repeat(c, 64) }
		if err := repo.Insert(ctx, 1, hash("a"), kind, soon); err != nil {
			t.Fatal(err)
		}
		if err := repo.Insert(ctx, 2, hash("b"), kind, time.Now().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
		if uid, ok, err := repo.Valid(ctx, hash("a"), kind); err != nil || !ok || uid != 1 {
			t.Errorf("valid: %d %v %v", uid, ok, err)
		}
		if _, ok, _ := repo.Valid(ctx, hash("b"), kind); ok {
			t.Error("an expired token valid")
		}
		pending, err := repo.Pending(ctx, kind, []int{1, 2, 3})
		if err != nil || len(pending) != 2 || !pending[1].Equal(soon) || pending[2].IsZero() {
			t.Errorf("pending: %v %v", pending, err)
		}
		if uid, ok, err := repo.Consume(ctx, hash("a"), kind); err != nil || !ok || uid != 1 {
			t.Fatalf("consume: %d %v %v", uid, ok, err)
		}
		if _, ok, _ := repo.Valid(ctx, hash("a"), kind); ok {
			t.Error("a used token valid")
		}
		if pending, _ := repo.Pending(ctx, kind, []int{1, 2}); len(pending) != 1 || pending[2].IsZero() {
			t.Errorf("pending after use: %v", pending)
		}
	})
}
