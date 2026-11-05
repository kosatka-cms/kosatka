//go:build dbintegration

package database_test

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestSettingsKeptAfterCommit - settings written in a transaction: a read
// outside it before the commit sees, and keeps, the old row; once the
// transaction commits the next read sees the new one (afterWrite), and a
// rolled back write changes nothing.
func TestSettingsKeptAfterCommit(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		uow := database.NewUnitOfWork(database.ClientFor(db.DB))
		comments := database.NewCommentSettingsRepository(db.DB)
		site := database.NewSiteInformationRepository(db.DB)
		if err := comments.Update(ctx, entity.CommentSettings{LinksLimit: 2}); err != nil {
			t.Fatal(err)
		}
		if err := uow.Do(ctx, func(ctx context.Context) error {
			if err := comments.Update(ctx, entity.CommentSettings{LinksLimit: 7}); err != nil {
				return err
			}
			if err := site.SetTheme(ctx, "fixture"); err != nil {
				return err
			}
			// Outside the transaction: the committed rows, kept.
			if s, err := comments.Get(context.Background()); err != nil || s.LinksLimit != 2 {
				t.Errorf("comment settings before the commit: %+v %v", s, err)
			}
			if s, err := site.Get(context.Background()); err != nil || (s.Theme != nil && *s.Theme == "fixture") {
				t.Errorf("site information before the commit: %+v %v", s, err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if s, err := comments.Get(ctx); err != nil || s.LinksLimit != 7 {
			t.Errorf("comment settings after the commit: %+v %v, want the new row", s, err)
		}
		if s, err := site.Get(ctx); err != nil || s.Theme == nil || *s.Theme != "fixture" {
			t.Errorf("site information after the commit: %+v %v, want the new theme", s, err)
		}

		_ = uow.Do(ctx, func(ctx context.Context) error {
			if err := comments.Update(ctx, entity.CommentSettings{LinksLimit: 9}); err != nil {
				return err
			}
			return context.Canceled // rolled back
		})
		if s, err := comments.Get(ctx); err != nil || s.LinksLimit != 7 {
			t.Errorf("comment settings after a rollback: %+v %v", s, err)
		}
	})
}
