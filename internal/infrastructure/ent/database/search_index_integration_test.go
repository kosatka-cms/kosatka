//go:build dbintegration

package database_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/searchword"
	"github.com/kosatka-cms/kosatka/internal/testdb"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// setSiteLanguage sets the site's default language (site_information).
func setSiteLanguage(t *testing.T, db testdb.DB, lang string) {
	t.Helper()
	if n, err := entClient(db).SiteInformation.Query().Count(context.Background()); err != nil {
		t.Fatal(err)
	} else if n == 0 {
		seedRow(t, db, `INSERT INTO site_information (id) VALUES (1)`)
	}
	seedRow(t, db, `UPDATE site_information SET default_language = '`+lang+`' WHERE id = 1`)
}

// searchWordsOf - the node's words of the field (0 - the title), sorted.
func searchWordsOf(t *testing.T, client *generated.Client, nodeID, fieldID int) string {
	t.Helper()
	rows, err := client.SearchWord.Query().Where(searchword.Nid(nodeID), searchword.FieldIDEQ(fieldID)).All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	words := make([]string, len(rows))
	for i, r := range rows {
		words[i] = r.Word
	}
	slices.Sort(words)
	return fmt.Sprint(words)
}

// TestSearchIndexWrites - a node's title words are written with its row
// (stemmed in the node's language), a text field's with its values (the
// HTML left out), each kept when the other is saved; a value removed
// takes its words; a rebuild writes the same.
func TestSearchIndexWrites(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		// The new nodes' language (the node's hooks).
		ctx := i18n.WithLang(context.Background(), "ru")
		seedUser(t, db)
		setSiteLanguage(t, db, "ru")
		client := entClient(db)
		now := time.Now()
		ct, err := client.ContentTypeDef.Create().SetMachineName("article").SetLabel("Article").
			SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_body").SetFieldType(string(entity.FieldTypeText)).
			SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.FieldInstanceDef.Create().SetContentTypeID(ct.ID).SetFieldStorageID(st.ID).SetLabel("Body").
			SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
			t.Fatal(err)
		}
		types := database.NewContentTypeRepository(client)
		fields, err := types.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			t.Fatal(err)
		}
		nodes := database.NewNodeRepository(client)

		id, err := nodes.CreateRow(ctx, entity.Node{Type: "article", Title: "Синие коты", Slug: "cats", Status: 1, CreatedBy: entity.User{ID: 1}})
		if err != nil {
			t.Fatal(err)
		}
		if got := searchWordsOf(t, client, id, 0); got != "[кот син]" {
			t.Errorf("title words %s", got)
		}
		if err := types.SetFieldValues(ctx, id, fields, map[int]any{fields[0].ID: `<p class="blue">Ёлки <b>пушистые</b></p>`}, nil); err != nil {
			t.Fatal(err)
		}
		if got := searchWordsOf(t, client, id, st.ID); got != "[елк пушист]" {
			t.Errorf("field words %s", got)
		}
		if err := nodes.UpdateRow(ctx, entity.Node{ID: id, Title: "Рыжий кот", Slug: "cats", Status: 1}); err != nil {
			t.Fatal(err)
		}
		if title, body := searchWordsOf(t, client, id, 0), searchWordsOf(t, client, id, st.ID); title != "[кот рыж]" || body != "[елк пушист]" {
			t.Errorf("after the title changed: %s %s", title, body)
		}
		en, err := nodes.CreateRow(i18n.WithLang(ctx, "en"), entity.Node{Type: "article", Title: "Running cats", Slug: "en", Status: 1, CreatedBy: entity.User{ID: 1}})
		if err != nil {
			t.Fatal(err)
		}
		if got := searchWordsOf(t, client, en, 0); got != "[cat run]" {
			t.Errorf("an English node's title words %s", got)
		}
		if n, err := database.RebuildSearchIndex(ctx, client); err != nil || n != 2 {
			t.Fatalf("rebuild: %d %v", n, err)
		}
		if title, body := searchWordsOf(t, client, id, 0), searchWordsOf(t, client, id, st.ID); title != "[кот рыж]" || body != "[елк пушист]" {
			t.Errorf("after a rebuild: %s %s", title, body)
		}
		if err := types.SetFieldValues(ctx, id, fields, map[int]any{}, nil); err != nil {
			t.Fatal(err)
		}
		if got := searchWordsOf(t, client, id, st.ID); got != "[]" {
			t.Errorf("field words after the value went: %s", got)
		}
	})
}

// TestSetLangcodeReindexes - a node's language set through the core: its
// title's and its text values' words are stemmed in the new language.
func TestSetLangcodeReindexes(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := i18n.WithLang(context.Background(), "ru")
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()
		ct, err := client.ContentTypeDef.Create().SetMachineName("article").SetLabel("Article").
			SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_body").SetFieldType(string(entity.FieldTypeText)).
			SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.FieldInstanceDef.Create().SetContentTypeID(ct.ID).SetFieldStorageID(st.ID).SetLabel("Body").
			SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
			t.Fatal(err)
		}
		types := database.NewContentTypeRepository(client)
		fields, err := types.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			t.Fatal(err)
		}
		nodes := database.NewNodeRepository(client)
		// Created in Russian (the context's language): English words kept whole.
		id, err := nodes.CreateRow(ctx, entity.Node{Type: "article", Title: "Running cats", Slug: "cats", Status: 1, CreatedBy: entity.User{ID: 1}})
		if err != nil {
			t.Fatal(err)
		}
		if err := types.SetFieldValues(ctx, id, fields, map[int]any{fields[0].ID: "<p>Jumping dogs</p>"}, nil); err != nil {
			t.Fatal(err)
		}
		if title := searchWordsOf(t, client, id, 0); title != "[cats running]" {
			t.Fatalf("in Russian: %s", title)
		}
		if err := nodes.SetLangcode(ctx, id, "en"); err != nil {
			t.Fatal(err)
		}
		if title, body := searchWordsOf(t, client, id, 0), searchWordsOf(t, client, id, st.ID); title != "[cat run]" || body != "[dog jump]" {
			t.Errorf("after the language set to English: %s %s", title, body)
		}
		if n, err := nodes.ByID(ctx, id); err != nil || n.Langcode != "en" {
			t.Errorf("the node's language: %+v %v", n, err)
		}
	})
}
