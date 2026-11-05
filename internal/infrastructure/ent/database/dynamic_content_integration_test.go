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

// A content item's front page and sticky flags: saved with it, changed on
// update, read back.
func TestDynamicContentPromoteSticky(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		repo := database.NewDynamicContentRepository(client, database.NewContentTypeRepository(client), database.NewTaxonomyRepository(client),
			entity.ContentTypeDef{MachineName: "article"}, nil)
		created, err := repo.Create(ctx, &entity.DynamicContent{Node: entity.Node{
			Title: "Whale", Slug: "article/whale", Status: 1, Promote: true, Sticky: true, CreatedBy: entity.User{ID: 1},
		}})
		if err != nil {
			t.Fatal(err)
		}
		if !created.Node.Promote || !created.Node.Sticky {
			t.Errorf("created: promote %v, sticky %v", created.Node.Promote, created.Node.Sticky)
		}
		created.Node.Sticky = false
		updated, err := repo.Update(ctx, created)
		if err != nil {
			t.Fatal(err)
		}
		if !updated.Node.Promote || updated.Node.Sticky {
			t.Errorf("updated: promote %v, sticky %v, want true, false", updated.Node.Promote, updated.Node.Sticky)
		}
	})
}

// A node and its values are saved together: a value that can't be
// written (a file that isn't there) leaves no node on create, and the
// node's values as they were on update - not deleted halfway.
func TestDynamicContentSaveIsAtomic(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		types := database.NewContentTypeRepository(client)
		now := time.Now()
		ct, err := client.ContentTypeDef.Create().SetMachineName("article").SetLabel("Article").
			SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for name, typ := range map[string]entity.ContentTypeFieldType{"field_subtitle": entity.FieldTypeText, "field_cover": entity.FieldTypeImage} {
			st, err := client.FieldStorageDef.Create().SetMachineName(name).SetFieldType(string(typ)).SetConfig(map[string]any{}).
				SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.FieldInstanceDef.Create().SetContentTypeID(ct.ID).SetFieldStorageID(st.ID).SetLabel(name).
				SetConfig(map[string]any{}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx); err != nil {
				t.Fatal(err)
			}
		}
		fields, err := types.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			t.Fatal(err)
		}
		byName := map[string]int{}
		for _, f := range fields {
			byName[f.MachineName] = f.ID
		}
		repo := database.NewDynamicContentRepository(client, types, database.NewTaxonomyRepository(client), entity.ContentTypeDef{ID: ct.ID, MachineName: "article"}, fields)
		nodes := func() int {
			t.Helper()
			n, err := client.Node.Query().Count(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
		node := entity.Node{Title: "Whale", Slug: "article/whale", Status: 1, CreatedBy: entity.User{ID: 1}}

		if _, err := repo.Create(ctx, &entity.DynamicContent{Node: node, Values: map[int]any{
			byName["field_subtitle"]: "The white one", byName["field_cover"]: 987654}}); err == nil {
			t.Fatal("created with a file that isn't there")
		}
		if n := nodes(); n != 0 {
			t.Fatalf("%d nodes after the failed create, want 0", n)
		}

		created, err := repo.Create(ctx, &entity.DynamicContent{Node: node, Values: map[int]any{byName["field_subtitle"]: "The white one"}})
		if err != nil {
			t.Fatal(err)
		}
		created.Node.Title = "Moby"
		created.Values = map[int]any{byName["field_subtitle"]: "Changed", byName["field_cover"]: 987654}
		if _, err := repo.Update(ctx, created); err == nil {
			t.Fatal("updated with a file that isn't there")
		}
		got, err := repo.ByNodeID(ctx, created.Node.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Node.Title != "Whale" || got.Values[byName["field_subtitle"]] != "The white one" {
			t.Errorf("after the failed update: %q, %v - want the node as it was", got.Node.Title, got.Values)
		}
	})
}
