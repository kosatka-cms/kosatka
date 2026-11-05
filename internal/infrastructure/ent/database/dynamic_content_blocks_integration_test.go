//go:build dbintegration

package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/schema"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// blocksFixture - a field set "role" (a text field) and a type "film"
// with a blocks field of it: the film's repository, the set's fields.
func blocksFixture(ctx context.Context, t *testing.T, client *generated.Client) (*database.DynamicContentRepository, entity.FieldInstanceDefs, entity.FieldInstanceDef) {
	t.Helper()
	types := database.NewContentTypeRepository(client)
	now := time.Now()
	typeOf := func(name string, embedded bool) int {
		ct, err := client.ContentTypeDef.Create().SetMachineName(name).SetLabel(name).SetStatus(1).SetEmbedded(embedded).
			SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return ct.ID
	}
	attach := func(typeID int, name string, ft entity.ContentTypeFieldType, config map[string]any) {
		st, err := client.FieldStorageDef.Create().SetMachineName(name).SetFieldType(string(ft)).SetConfig(config).
			SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.FieldInstanceDef.Create().SetContentTypeID(typeID).SetFieldStorageID(st.ID).SetLabel(name).
			SetConfig(map[string]any{}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx); err != nil {
			t.Fatal(err)
		}
	}
	role, film := typeOf("role", true), typeOf("film", false)
	attach(role, "field_character", entity.FieldTypeText, map[string]any{})
	attach(film, "field_cast", entity.FieldTypeBlocks, map[string]any{"block_types": []any{"role"}})
	roleFields, err := types.FieldsByContentTypeID(ctx, role)
	if err != nil {
		t.Fatal(err)
	}
	filmFields, err := types.FieldsByContentTypeID(ctx, film)
	if err != nil {
		t.Fatal(err)
	}
	repo := database.NewDynamicContentRepository(client, types, database.NewTaxonomyRepository(client),
		entity.ContentTypeDef{ID: film, MachineName: "film"}, filmFields)
	return repo, roleFields, filmFields[0]
}

// A blocks field's items: saved as nodes of their set with their parent
// and read back in order; an edit updates the kept ones in place, adds
// the new ones and deletes the left out; a revision's items come back as
// new ones; the items are no content of their own (a node query keeps
// them only WithBlocks: no page, search, sitemap) but the set's own
// (its count); deleting the node deletes them.
func TestDynamicContentBlocks(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		withBlocks := schema.WithBlocks(ctx)
		seedUser(t, db)
		client := entClient(db)
		repo, roleFields, cast := blocksFixture(ctx, t, client)
		character := roleFields[0].ID
		role := func(id int, name string) entity.BlockItem {
			return entity.BlockItem{ID: id, Type: "role", Fields: roleFields, Values: map[int]any{character: name}}
		}
		names := func(dc *entity.DynamicContent) []string {
			var out []string
			for _, item := range entity.BlockItems(dc.Values[cast.ID]) {
				out = append(out, item.Values[character].(string))
			}
			return out
		}

		created, err := repo.Create(ctx, &entity.DynamicContent{
			Node:   entity.Node{Title: "Stalker", Slug: "film/stalker", Status: 1, CreatedBy: entity.User{ID: 1}},
			Values: map[int]any{cast.ID: []entity.BlockItem{role(0, "Stalker"), role(0, "Writer"), {Type: "film"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := names(created); len(got) != 2 || got[0] != "Stalker" || got[1] != "Writer" {
			t.Fatalf("created blocks %v, want [Stalker Writer] (a type that isn't a set dropped)", got)
		}
		items := entity.BlockItems(created.Values[cast.ID])
		if _, err := client.Node.Get(ctx, items[0].ID); !generated.IsNotFound(err) {
			t.Errorf("a node query found a block (error %v)", err)
		}
		row, err := client.Node.Get(withBlocks, items[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if row.ParentID == nil || *row.ParentID != created.Node.ID || row.Slug != "" || row.Title != "Stalker" {
			t.Errorf("block node: parent %v, slug %q, title %q", row.ParentID, row.Slug, row.Title)
		}

		// Writer moved first and renamed, Stalker left out, a new one added.
		created.Values = map[int]any{cast.ID: []entity.BlockItem{role(items[1].ID, "Writer (2)"), role(0, "Professor"), role(items[0].ID+1000, "Forged")}}
		updated, err := repo.Update(ctx, created)
		if err != nil {
			t.Fatal(err)
		}
		after := entity.BlockItems(updated.Values[cast.ID])
		if got := names(updated); len(got) != 3 || got[0] != "Writer (2)" || got[1] != "Professor" || got[2] != "Forged" {
			t.Fatalf("updated blocks %v", got)
		}
		if after[0].ID != items[1].ID {
			t.Errorf("the kept block got a new node %d, want %d", after[0].ID, items[1].ID)
		}
		if after[2].ID == items[0].ID+1000 {
			t.Error("a block of another node's ID was taken as its own")
		}
		if exists, _ := client.Node.Query().Where(node.ID(items[0].ID)).Exist(withBlocks); exists {
			t.Error("the left out block is still there")
		}

		// A revision's blocks (their snapshot): new nodes.
		raw := entity.SnapshotOf(entity.DynamicContent{Fields: entity.FieldInstanceDefs{cast}, Values: map[int]any{cast.ID: []entity.BlockItem{role(0, "Old one")}}})
		values, _ := raw.Values(entity.FieldInstanceDefs{cast})
		updated.Values = values
		reverted, err := repo.Update(ctx, updated)
		if err != nil {
			t.Fatal(err)
		}
		if got := names(reverted); len(got) != 1 || got[0] != "Old one" {
			t.Fatalf("reverted blocks %v, want [Old one]", got)
		}

		nodes := database.NewNodeRepository(client)
		if _, err := nodes.ByID(ctx, after[0].ID); !errors.Is(err, entity.ErrEntityNotFound) {
			t.Errorf("ByID of a block: error %v, want not found", err)
		}
		if n, err := nodes.CountByType(ctx, "role"); err != nil || n != 1 {
			t.Errorf("CountByType(role) = %d, %v; want 1 (the block)", n, err)
		}
		hits, err := nodes.SearchByTitle(ctx, "Old", nil, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 0 {
			t.Errorf("search finds blocks: %v", hits)
		}
		sitemap, err := nodes.CountPublicSitemapNodes(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if sitemap != 1 {
			t.Errorf("%d nodes in the sitemap, want 1 (the film)", sitemap)
		}

		if err := database.NewNodeDeleteRepository(db.DB).DeleteNode(ctx, created.Node.ID); err != nil {
			t.Fatal(err)
		}
		if n, _ := client.Node.Query().Count(withBlocks); n != 0 {
			t.Errorf("%d nodes after deleting the film, want 0", n)
		}
		if n, _ := client.FieldValueBlock.Query().Count(ctx); n != 0 {
			t.Errorf("%d block rows after deleting the film, want 0", n)
		}
	})
}

// A set taken out of a blocks field: the node's blocks of it are kept on
// saving, a new one of it is dropped.
func TestDynamicContentBlocksOfSetTakenOut(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		repo, roleFields, cast := blocksFixture(ctx, t, client)
		character := roleFields[0].ID
		role := func(id int, name string) entity.BlockItem {
			return entity.BlockItem{ID: id, Type: "role", Fields: roleFields, Values: map[int]any{character: name}}
		}
		created, err := repo.Create(ctx, &entity.DynamicContent{
			Node:   entity.Node{Title: "Stalker", Slug: "film/stalker", Status: 1, CreatedBy: entity.User{ID: 1}},
			Values: map[int]any{cast.ID: []entity.BlockItem{role(0, "Stalker")}},
		})
		if err != nil {
			t.Fatal(err)
		}
		kept := entity.BlockItems(created.Values[cast.ID])[0]

		cast.Config = map[string]any{"block_types": []any{"scene"}}
		types := database.NewContentTypeRepository(client)
		without := database.NewDynamicContentRepository(client, types, database.NewTaxonomyRepository(client),
			entity.ContentTypeDef{ID: created.ContentType.ID, MachineName: "film"}, entity.FieldInstanceDefs{cast})
		created.Fields = entity.FieldInstanceDefs{cast}
		created.Values = map[int]any{cast.ID: []entity.BlockItem{role(kept.ID, "Stalker (2)"), role(0, "Writer")}}
		updated, err := without.Update(ctx, created)
		if err != nil {
			t.Fatal(err)
		}
		items := entity.BlockItems(updated.Values[cast.ID])
		if len(items) != 1 || items[0].ID != kept.ID || items[0].Values[character] != "Stalker (2)" {
			t.Errorf("blocks %+v, want the kept one updated alone", items)
		}
	})
}
