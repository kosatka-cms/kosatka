//go:build dbintegration

package database_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestRichTextFieldStorageIDs - a text storage holds HTML only when every
// instance of it does (text formats or an HTML widget): one plain
// instance makes it plain text.
func TestRichTextFieldStorageIDs(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		repo := database.NewContentTypeRepository(client)
		now := time.Now()

		newType := func(name string) int {
			ct, err := client.ContentTypeDef.Create().SetMachineName(name).SetLabel(name).
				SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return ct.ID
		}
		newStorage := func(name string) int {
			st, err := client.FieldStorageDef.Create().SetMachineName(name).SetFieldType("text").
				SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return st.ID
		}
		attach := func(typeID, storageID int, config map[string]any) {
			if err := client.FieldInstanceDef.Create().SetContentTypeID(typeID).SetFieldStorageID(storageID).SetLabel("F").
				SetConfig(config).SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}

		a, b := newType("article"), newType("page")
		body, mixed, plain := newStorage("body"), newStorage("mixed"), newStorage("plain")
		attach(a, body, map[string]any{"allowed_text_formats": []any{1}})
		attach(b, body, map[string]any{"widget": "wysiwyg"})
		attach(a, mixed, map[string]any{"widget": "safe_html"})
		attach(b, mixed, map[string]any{"widget": "textarea"})
		attach(a, plain, map[string]any{"widget": "input"})

		got, err := repo.RichTextFieldStorageIDs(ctx, []int{body, mixed, plain})
		if err != nil {
			t.Fatal(err)
		}
		if want := map[int]bool{body: true, mixed: false, plain: false}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}
