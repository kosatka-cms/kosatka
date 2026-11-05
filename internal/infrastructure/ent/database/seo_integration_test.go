//go:build dbintegration

package database_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

func TestSEORepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seo := database.NewSEORepository(db.DB)
		tpl := entity.SEOTemplate{Scope: "node", TitleTemplate: "[title]", DescriptionTemplate: "[summary]", ImageTemplate: "[image]",
			SchemaType: "Event", SchemaProps: map[string]string{"startDate": "[node:field:date:iso]"}}
		for range 2 { // creates the row, then updates it
			if err := seo.SaveSEOTemplate(ctx, tpl); err != nil {
				t.Fatal(err)
			}
		}
		if got, err := seo.SEOTemplates(ctx); err != nil || !reflect.DeepEqual(got["node"], tpl) || len(got) != 1 {
			t.Errorf("templates: %+v %v", got, err)
		}

		seedRow(t, db, `INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (3, 'tags', 'taxonomy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		storage := 3
		rule, err := seo.CreateSEORule(ctx, entity.SEORuleDef{ContentType: "news", FieldStorageID: &storage, TermIDs: "1,2", TitleTemplate: "T", Weight: 5})
		if err != nil || rule.ID == 0 {
			t.Fatalf("create rule: %+v %v", rule, err)
		}
		got, err := seo.SEORuleByID(ctx, rule.ID)
		if err != nil || !reflect.DeepEqual(got, rule) {
			t.Errorf("rule:\n%+v\n%+v %v", got, rule, err)
		}
		rule.FieldStorageID = nil
		if err := seo.UpdateSEORule(ctx, *rule); err != nil {
			t.Fatal(err)
		}
		if err := seo.UpdateSEORuleWeight(ctx, rule.ID, 9); err != nil {
			t.Fatal(err)
		}
		if list, err := seo.SEORules(ctx); err != nil || len(list) != 1 || list[0].FieldStorageID != nil || list[0].Weight != 9 {
			t.Errorf("rules: %+v %v", list, err)
		}
		_ = seo.DeleteSEORule(ctx, rule.ID)
		if _, err := seo.SEORuleByID(ctx, rule.ID); !errors.Is(err, entity.ErrEntityNotFound) {
			t.Errorf("deleted rule: %v", err)
		}
	})
}
