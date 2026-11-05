//go:build dbintegration

package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestTwoFactorRepository - on with its secret, step and codes (again -
// replaced); a step taken once and only forward; a recovery code spent
// once; new codes replace the old; who has it on; off - the secret and
// the codes gone; a deleted user takes theirs along.
func TestTwoFactorRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedRow(t, db, `INSERT INTO users (id, username, slug, password_hash) VALUES (2, 'bob', 'bob', '')`)
		repo := database.NewTwoFactorRepository(db.DB)
		at := time.Now().UTC().Truncate(time.Second)

		if tf, err := repo.ByUser(ctx, 1); err != nil || tf != nil {
			t.Fatalf("off at first: %+v %v", tf, err)
		}
		if err := repo.Enable(ctx, 1, "OLD", 5, []string{"x"}, at); err != nil {
			t.Fatal(err)
		}
		if err := repo.Enable(ctx, 1, "SECRET", 100, []string{"a", "b", "c"}, at); err != nil {
			t.Fatal(err)
		}
		tf, err := repo.ByUser(ctx, 1)
		if err != nil || tf == nil || tf.Secret != "SECRET" || tf.LastStep != 100 || tf.CodesLeft != 3 || !tf.EnabledAt.Equal(at) {
			t.Fatalf("on: %+v %v", tf, err)
		}
		for step, want := range map[int64]bool{100: false, 99: false} {
			if ok, err := repo.AcceptStep(ctx, 1, step); err != nil || ok != want {
				t.Errorf("step %d: %v %v", step, ok, err)
			}
		}
		if ok, _ := repo.AcceptStep(ctx, 1, 101); !ok {
			t.Error("the next step refused")
		}
		if ok, _ := repo.UseRecoveryCode(ctx, 1, "b"); !ok {
			t.Error("a code refused")
		}
		if ok, _ := repo.UseRecoveryCode(ctx, 1, "b"); ok {
			t.Error("a code spent twice")
		}
		if ok, _ := repo.UseRecoveryCode(ctx, 1, "x"); ok {
			t.Error("a replaced code still works")
		}
		if tf, _ := repo.ByUser(ctx, 1); tf.CodesLeft != 2 {
			t.Errorf("codes left %d", tf.CodesLeft)
		}
		if err := repo.ReplaceRecoveryCodes(ctx, 1, []string{"d"}); err != nil {
			t.Fatal(err)
		}
		if ok, _ := repo.UseRecoveryCode(ctx, 1, "a"); ok {
			t.Error("an old code after new ones")
		}
		if err := repo.Enable(ctx, 2, "BOB", 1, nil, at); err != nil {
			t.Fatal(err)
		}
		if on, err := repo.EnabledAmong(ctx, []int{1, 2, 3}); err != nil || !on[1] || !on[2] || on[3] {
			t.Errorf("among: %v %v", on, err)
		}
		if err := repo.Disable(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if tf, _ := repo.ByUser(ctx, 1); tf != nil {
			t.Error("still on")
		}
		if _, err := db.DB.Exec(`DELETE FROM users WHERE id = 2`); err != nil {
			t.Fatal(err)
		}
		if tf, _ := repo.ByUser(ctx, 2); tf != nil {
			t.Error("a deleted user's left behind")
		}
	})
}
