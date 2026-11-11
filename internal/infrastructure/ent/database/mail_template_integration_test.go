//go:build dbintegration

package database_test

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestMailTemplateRepository - an email's row made by its first change:
// a text saved keeps the email on, a switch keeps the text; both change
// the one row.
func TestMailTemplateRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		repo := database.NewMailTemplateRepository(db.DB)
		if all, err := repo.All(ctx); err != nil || len(all) != 0 {
			t.Fatalf("none changed: %v %v", all, err)
		}
		if err := repo.SaveText(ctx, "mod.a", entity.MailTemplate{Subject: "Hi", Body: "Text"}); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetEnabled(ctx, "mod.b", false); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetEnabled(ctx, "mod.a", false); err != nil {
			t.Fatal(err)
		}
		if err := repo.SaveText(ctx, "mod.b", entity.MailTemplate{Subject: "B"}); err != nil {
			t.Fatal(err)
		}
		all, err := repo.All(ctx)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]entity.StoredMail{
			"mod.a": {Enabled: false, MailTemplate: entity.MailTemplate{Subject: "Hi", Body: "Text"}},
			"mod.b": {Enabled: false, MailTemplate: entity.MailTemplate{Subject: "B"}},
		}
		if len(all) != 2 || all["mod.a"] != want["mod.a"] || all["mod.b"] != want["mod.b"] {
			t.Errorf("rows: %+v", all)
		}
		// The module uninstalled: its emails' rows go, another's stay.
		if err := repo.SaveText(ctx, "module.c", entity.MailTemplate{Subject: "C"}); err != nil {
			t.Fatal(err)
		}
		if err := database.NewModuleUninstallRepository(db.DB).DeleteMailTexts(ctx, "mod."); err != nil {
			t.Fatal(err)
		}
		if all, err := repo.All(ctx); err != nil || len(all) != 1 || all["module.c"].Subject != "C" {
			t.Errorf("after uninstalling mod: %+v %v", all, err)
		}
	})
}
