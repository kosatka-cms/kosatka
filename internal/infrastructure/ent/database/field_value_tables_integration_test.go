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

// TestFieldValueTables - what goes through the shared value table
// (valueTableOf): counting a field's values on one type, deleting a node's
// values from every table, deleting a field with its values on its own
// type only.
func TestFieldValueTables(t *testing.T) {
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
		newStorage := func(name string, typ entity.ContentTypeFieldType, config map[string]any) int {
			st, err := client.FieldStorageDef.Create().SetMachineName(name).SetFieldType(string(typ)).SetConfig(config).
				SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return st.ID
		}
		attach := func(typeID, storageID int) int {
			in, err := client.FieldInstanceDef.Create().SetContentTypeID(typeID).SetFieldStorageID(storageID).SetLabel("F").
				SetConfig(map[string]any{}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return in.ID
		}
		fieldsOf := func(typeID int) entity.FieldInstanceDefs {
			fields, err := repo.FieldsByContentTypeID(ctx, typeID)
			if err != nil {
				t.Fatal(err)
			}
			return fields
		}

		articles, pages := newType("article"), newType("page")
		subtitle := newStorage("field_subtitle", entity.FieldTypeText, map[string]any{})
		tags := newStorage("field_tags", entity.FieldTypeList, map[string]any{"multiple": true, "allowed_values": []any{"a", "b"}})
		articleSubtitle := attach(articles, subtitle)
		attach(articles, tags)
		attach(pages, subtitle)
		seedNode(t, db, 1, "article", "One", "article/one")
		seedNode(t, db, 2, "article", "Two", "article/two")
		seedNode(t, db, 3, "page", "Three", "page/three")
		for node, typ := range map[int]int{1: articles, 2: articles, 3: pages} {
			values := map[int]any{}
			for _, f := range fieldsOf(typ) {
				if f.FieldStorageID == subtitle {
					values[f.ID] = "sub"
				} else {
					values[f.ID] = []string{"a", "b"}
				}
			}
			if err := repo.SetFieldValues(ctx, node, fieldsOf(typ), values, nil); err != nil {
				t.Fatal(err)
			}
		}

		// Counted on the type's own nodes: 2 articles; the multi-value
		// table - one row per value.
		if n, err := repo.CountFieldValues(ctx, articles, subtitle, entity.FieldTypeText, false); err != nil || n != 2 {
			t.Errorf("subtitle values on articles: %d %v", n, err)
		}
		if n, err := repo.CountFieldValues(ctx, articles, tags, entity.FieldTypeList, true); err != nil || n != 4 {
			t.Errorf("tag values on articles: %d %v", n, err)
		}
		if n, err := repo.CountFieldValues(ctx, pages, subtitle, entity.FieldTypeText, false); err != nil || n != 1 {
			t.Errorf("subtitle values on pages: %d %v", n, err)
		}

		// A node's values go from every table with it (their foreign keys).
		if _, err := db.DB.ExecContext(ctx, db.Dialect.Rebind(`DELETE FROM nodes WHERE id = ?`), 2); err != nil {
			t.Fatal(err)
		}
		if vals, err := repo.FieldValues(ctx, 2, fieldsOf(articles)); err != nil || len(vals) != 0 {
			t.Errorf("node 2 values left: %v %v", vals, err)
		}

		// Detaching the subtitle from articles drops the articles' values
		// only; the pages keep theirs.
		if err := repo.DeleteField(ctx, articleSubtitle); err != nil {
			t.Fatal(err)
		}
		if n, _ := client.FieldValueText.Query().Count(ctx); n != 1 {
			t.Errorf("text values after detaching from articles: %d, want the page's 1", n)
		}
		if n, err := repo.CountFieldValues(ctx, articles, tags, entity.FieldTypeList, true); err != nil || n != 2 {
			t.Errorf("tag values after: %d %v", n, err)
		}
	})
}
