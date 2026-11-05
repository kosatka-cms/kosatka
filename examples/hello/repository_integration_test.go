//go:build dbintegration

package hello

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestRepository - greetings are stored in the module's own table (its
// schema installed with the engine's, testkit.SchemaDB) and listed newest
// first.
func TestRepository(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		greetings := NewRepository(db.DB)
		for _, text := range []string{"Hi", "Hello there"} {
			if _, err := greetings.Add(ctx, text); err != nil {
				t.Fatal(err)
			}
		}
		list, err := greetings.List(ctx)
		if err != nil || len(list) != 2 || list[0].Text != "Hello there" || list[1].Text != "Hi" {
			t.Errorf("greetings: %+v %v", list, err)
		}
	})
}
