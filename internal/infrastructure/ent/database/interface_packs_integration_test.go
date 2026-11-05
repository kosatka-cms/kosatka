//go:build dbintegration

package database_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestInterfacePackRepository - a downloaded language's pack and its
// strings (more than a statement's batch, plural ones, in a context);
// installed again - in place of the old ones; another language and the
// site's own changes left alone; deleted - the pack and its strings.
func TestInterfacePackRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		repo := database.NewInterfacePackRepository(db.DB)
		site := database.NewInterfaceStringRepository(db.DB)
		at := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
		if err := site.Save(ctx, entity.InterfaceString{Lang: "de", Source: "Comments", Text: "Anmerkungen", UpdatedAt: at}); err != nil {
			t.Fatal(err)
		}
		var de []entity.InterfaceString
		for i := range 1203 {
			de = append(de, entity.InterfaceString{Source: fmt.Sprintf("String %d", i), Text: fmt.Sprintf("Zeichenkette %d", i)})
		}
		de = append(de,
			entity.InterfaceString{Source: "Comments", Text: "Kommentare"},
			entity.InterfaceString{Source: "@count comment", Forms: []string{"@count Kommentar", "@count Kommentare"}},
			entity.InterfaceString{Context: "menu", Source: "Open", Text: "Öffnen"})
		pack := entity.InterfacePack{Lang: "de", Version: "0.1.0", SHA256: fmt.Sprintf("%064x", 1), Strings: len(de), Draft: true, InstalledAt: at}
		if err := repo.Install(ctx, pack, de); err != nil {
			t.Fatal(err)
		}
		if err := repo.Install(ctx, entity.InterfacePack{Lang: "fr", Version: "0.1.0", SHA256: fmt.Sprintf("%064x", 2), Strings: 1, InstalledAt: at},
			[]entity.InterfaceString{{Source: "Comments", Text: "Commentaires"}}); err != nil {
			t.Fatal(err)
		}
		count := func() map[string]int {
			t.Helper()
			rows, err := repo.Strings(ctx)
			if err != nil {
				t.Fatal(err)
			}
			out := map[string]int{}
			for _, r := range rows {
				out[r.Lang]++
				if r.Lang == "de" && r.Source == "@count comment" && (len(r.Forms) != 2 || r.Forms[1] != "@count Kommentare") {
					t.Errorf("plural = %+v", r)
				}
				if r.Lang == "de" && r.Source == "Open" && (r.Context != "menu" || r.Text != "Öffnen") {
					t.Errorf("in a context = %+v", r)
				}
			}
			return out
		}
		if got := count(); got["de"] != 1206 || got["fr"] != 1 {
			t.Errorf("strings = %v", got)
		}
		packs, err := repo.Packs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(packs) != 2 || packs[0].Lang != "de" || !packs[0].Draft || packs[0].Strings != 1206 || packs[0].SHA256 != pack.SHA256 || !packs[0].InstalledAt.Equal(at) {
			t.Errorf("packs = %+v", packs)
		}

		pack.Version, pack.Draft, pack.Strings = "0.2.0", false, 1
		if err := repo.Install(ctx, pack, []entity.InterfaceString{{Source: "Comments", Text: "Kommentare"}}); err != nil {
			t.Fatal(err)
		}
		if got := count(); got["de"] != 1 || got["fr"] != 1 {
			t.Errorf("after an update = %v", got)
		}
		if packs, _ := repo.Packs(ctx); len(packs) != 2 || packs[0].Version != "0.2.0" || packs[0].Draft {
			t.Errorf("packs after an update = %+v", packs)
		}

		if err := repo.Delete(ctx, "de"); err != nil {
			t.Fatal(err)
		}
		if got := count(); got["de"] != 0 || got["fr"] != 1 {
			t.Errorf("after deleting = %v", got)
		}
		if packs, _ := repo.Packs(ctx); len(packs) != 1 || packs[0].Lang != "fr" {
			t.Errorf("packs after deleting = %+v", packs)
		}
		if rows, err := site.All(ctx); err != nil || len(rows) != 1 || rows[0].Text != "Anmerkungen" {
			t.Errorf("the site's changes = %+v (%v)", rows, err)
		}
	})
}
