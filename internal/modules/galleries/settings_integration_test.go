//go:build dbintegration

package galleries

import (
	"context"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestSettingsRepository - a type without a row has the defaults, saving
// creates then updates its row, also with unchanged values; a node's switch is set and cleared, and
// goes with the node.
func TestSettingsRepository(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		d := testkit.Deps(db) // the core's, as Wire gets them on a site
		own := generated.NewClient(generated.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
		repo := NewSettingsRepository(d.DB, d.ContentTypes)
		now := time.Now()
		ctID := insertRow(t, db, "INSERT INTO content_types (machine_name, label, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			"place", "Place", 1, now, now)
		if s, err := repo.TypeSettings(ctx, "place"); err != nil || s != DefaultTypeSettings() {
			t.Errorf("no row: %+v %v", s, err)
		}
		// The same settings saved again (the type's form saved without
		// touching them) update the row, not create a second one.
		for _, want := range []TypeSettings{{DefaultEnabled: true, PictureField: "photo"}, {PictureField: "photo"}, {PictureField: "photo"}} {
			if err := repo.SaveTypeSettings(ctx, ctID, want); err != nil {
				t.Fatal(err)
			}
			if s, err := repo.TypeSettings(ctx, "place"); err != nil || s != want {
				t.Errorf("saved %+v, read %+v %v", want, s, err)
			}
		}
		if n, _ := own.GalleryContentType.Query().Count(ctx); n != 1 {
			t.Errorf("rows of the type: %d", n)
		}

		nodeID := insertRow(t, db, "INSERT INTO nodes (type, title, status) VALUES (?, ?, ?)", "place", "Harbour", 1)
		for _, on := range []bool{true, true, false, true} {
			if err := repo.SetNodeEnabled(ctx, nodeID, on); err != nil {
				t.Fatal(err)
			}
			if got, err := repo.NodeEnabled(ctx, nodeID); err != nil || got != on {
				t.Errorf("set %v, read %v %v", on, got, err)
			}
		}
		execSQL(t, db, "DELETE FROM nodes WHERE id = ?", nodeID)
		if n, _ := own.GalleryNode.Query().Count(ctx); n != 0 {
			t.Errorf("the switch outlived its node: %d", n)
		}
		execSQL(t, db, "DELETE FROM content_types WHERE id = ?", ctID)
		if n, _ := own.GalleryContentType.Query().Count(ctx); n != 0 {
			t.Errorf("the settings outlived their type: %d", n)
		}
	})
}
