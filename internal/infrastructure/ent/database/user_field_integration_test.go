//go:build dbintegration

package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

func TestUserFieldAndTokenRepositories(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)

		fields := database.NewUserFieldRepository(db.DB)
		id, err := fields.CreateField(ctx, entity.UserFieldDef{MachineName: "site", Label: "Site", MaxLen: 100})
		if err != nil {
			t.Fatal(err)
		}
		if err := fields.UpdateField(ctx, entity.UserFieldDef{ID: id, Label: "Website", MaxLen: 200, Weight: 1}); err != nil {
			t.Fatal(err)
		}
		if list, err := fields.Fields(ctx); err != nil || len(list) != 1 || list[0].Label != "Website" || list[0].MaxLen != 200 {
			t.Errorf("fields: %+v %v", list, err)
		}
		for _, v := range []string{"a.example", "b.example", "b.example"} { // creates, updates, saves unchanged
			if err := fields.SetValues(ctx, 1, map[int]string{id: v}); err != nil {
				t.Fatal(err)
			}
		}
		if vals, err := fields.Values(ctx, 1); err != nil || vals[id] != "b.example" {
			t.Errorf("values: %+v %v", vals, err)
		}
		_ = fields.SetValues(ctx, 1, map[int]string{id: " "})
		if vals, _ := fields.Values(ctx, 1); len(vals) != 0 {
			t.Errorf("value not deleted: %+v", vals)
		}
		_ = fields.SetValues(ctx, 1, map[int]string{id: "c.example"})
		if err := fields.DeleteField(ctx, id); err != nil {
			t.Fatal(err)
		}
		if vals, _ := fields.Values(ctx, 1); len(vals) != 0 {
			t.Errorf("values of the deleted field: %+v", vals)
		}

		tokens := database.NewUserAuthTokenRepository(db.DB)
		if err := tokens.Insert(ctx, 1, "h1", "reset", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if uid, ok, err := tokens.Consume(ctx, "h1", "reset"); err != nil || !ok || uid != 1 {
			t.Errorf("consume: %d %v %v", uid, ok, err)
		}
		if _, ok, err := tokens.Consume(ctx, "h1", "reset"); err != nil || ok {
			t.Errorf("consumed twice: %v %v", ok, err)
		}
		_ = tokens.Insert(ctx, 1, "h2", "reset", time.Now().Add(-time.Minute))
		if _, ok, _ := tokens.Consume(ctx, "h2", "reset"); ok {
			t.Error("expired token consumed")
		}
		_ = tokens.Insert(ctx, 1, "h3", "login", time.Now().Add(time.Hour))
		if err := tokens.InvalidateUserKind(ctx, 1, "login"); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := tokens.Consume(ctx, "h3", "login"); ok {
			t.Error("invalidated token consumed")
		}
	})
}
