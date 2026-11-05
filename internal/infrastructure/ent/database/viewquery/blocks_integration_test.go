//go:build dbintegration

package viewquery_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/viewquery"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestRunViewBlocksFilters - filters in a blocks field's blocks: the
// films with a block naming the current actor (a reference, contextual),
// with a block whose character contains a text, with no block of a
// character at all; the blocks themselves never listed.
func TestRunViewBlocksFilters(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()
		storage := func(name string, ft entity.ContentTypeFieldType, config map[string]any) entity.FieldStorageDef {
			st, err := client.FieldStorageDef.Create().SetMachineName(name).SetFieldType(string(ft)).SetConfig(config).
				SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return entity.FieldStorageDef{ID: st.ID, MachineName: name, FieldType: ft, Config: config}
		}
		cast := storage("field_cast", entity.FieldTypeBlocks, map[string]any{"block_types": []any{"role"}})
		actor := storage("field_actor", entity.FieldTypeReference, map[string]any{})
		character := storage("field_character", entity.FieldTypeText, map[string]any{})

		// Actors 1, 2; films 10 (actor 1 as Stalker, actor 2 as Writer), 11 (actor 2 as Professor), 12 (no cast).
		for id, title := range map[int]string{1: "Actor One", 2: "Actor Two", 10: "Stalker", 11: "Solaris", 12: "Mirror"} {
			seedNode(t, db, id, "film", title, "x/"+title)
		}
		block := func(id, film, delta, actorID int, name string) {
			t.Helper()
			if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO nodes (id, type, title, status, parent_id) VALUES (?, 'role', ?, 1, ?)`), id, name, film); err != nil {
				t.Fatal(err)
			}
			if err := client.FieldValueBlock.Create().SetNodeID(film).SetFieldID(cast.ID).SetItemID(id).SetDelta(delta).Exec(ctx); err != nil {
				t.Fatal(err)
			}
			if err := client.FieldValueReference.Create().SetNodeID(id).SetFieldID(actor.ID).SetTargetNodeID(actorID).Exec(ctx); err != nil {
				t.Fatal(err)
			}
			if err := client.FieldValueText.Create().SetNodeID(id).SetFieldID(character.ID).SetValue(name).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		block(100, 10, 0, 1, "Stalker")
		block(101, 10, 1, 2, "Writer")
		block(102, 11, 0, 2, "Professor")

		repo := viewquery.New(client)
		storages := map[int]entity.FieldStorageDef{actor.ID: actor, character.ID: character, cast.ID: cast}
		run := func(f entity.ViewFilterDef, contextNode int) []int {
			t.Helper()
			got, _, err := repo.RunView(ctx, entity.ViewRunParams{Filters: entity.ViewFilterDefs{f}, FieldStorages: storages, ContextNodeID: contextNode, PageSize: 20})
			if err != nil {
				t.Fatal(err)
			}
			var ids []int
			for _, n := range got {
				ids = append(ids, n.ID)
			}
			slices.Sort(ids)
			return ids
		}
		via, actorID, characterID := cast.ID, actor.ID, character.ID
		withActor := entity.ViewFilterDef{FieldStorageID: &actorID, ViaFieldStorageID: &via, Operator: entity.ViewFilterOperatorRelatedToCurrent, Contextual: true}
		if got := run(withActor, 2); !slices.Equal(got, []int{10, 11}) {
			t.Errorf("films with actor 2: %v, want [10 11]", got)
		}
		if got := run(withActor, 1); !slices.Equal(got, []int{10}) {
			t.Errorf("films with actor 1: %v, want [10]", got)
		}
		contains := entity.ViewFilterDef{FieldStorageID: &characterID, ViaFieldStorageID: &via, Operator: entity.ViewFilterOperatorContains, Value: "ess"}
		if got := run(contains, 0); !slices.Equal(got, []int{11}) {
			t.Errorf("a character with \"ess\": %v, want [11]", got)
		}
		empty := entity.ViewFilterDef{FieldStorageID: &characterID, ViaFieldStorageID: &via, Operator: entity.ViewFilterOperatorEmpty}
		if got := run(empty, 0); !slices.Equal(got, []int{1, 2, 12}) {
			t.Errorf("no character: %v, want [1 2 12] (the blocks never)", got)
		}
	})
}
