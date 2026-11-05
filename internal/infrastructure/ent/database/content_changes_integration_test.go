//go:build dbintegration

package database_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestOnContentChange - a node's change outside a transaction signals at
// once; inside one, only once it commits, never when it rolls back.
func TestOnContentChange(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		var changes atomic.Int32
		database.OnContentChange(client, func() { changes.Add(1) })
		create := func(ctx context.Context, slug string) error {
			return database.GetClient(ctx, client).Node.Create().SetType("article").SetTitle(slug).SetSlug(slug).SetCreatedBy(1).Exec(ctx)
		}

		if err := create(ctx, "outside"); err != nil {
			t.Fatal(err)
		}
		if got := changes.Load(); got != 1 {
			t.Fatalf("outside a transaction: %d changes, want 1", got)
		}

		uow := database.NewUnitOfWork(client)
		err := uow.Do(ctx, func(ctx context.Context) error {
			if err := create(ctx, "committed"); err != nil {
				return err
			}
			if got := changes.Load(); got != 1 {
				t.Errorf("before the commit: %d changes, want 1", got)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := changes.Load(); got != 2 {
			t.Fatalf("committed: %d changes, want 2", got)
		}

		failed := errors.New("refused")
		if err := uow.Do(ctx, func(ctx context.Context) error {
			if err := create(ctx, "rolled-back"); err != nil {
				return err
			}
			return failed
		}); !errors.Is(err, failed) {
			t.Fatalf("rollback: %v", err)
		}
		if got := changes.Load(); got != 2 {
			t.Errorf("rolled back: %d changes, want 2", got)
		}
	})
}
