//go:build dbintegration

package database_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestSetFieldMultiple - a list field shared by two types and a mutual
// reference field go to several values and back: the values move between
// the tables and read the same, the widgets follow, a node with several
// values is found, "symmetric" goes with the single value.
func TestSetFieldMultiple(t *testing.T) {
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
		attach := func(typeID, storageID int, widget string) {
			if err := client.FieldInstanceDef.Create().SetContentTypeID(typeID).SetFieldStorageID(storageID).SetLabel("F").
				SetConfig(map[string]any{"widget": widget}).SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		fieldsOf := func(typeID int) entity.FieldInstanceDefs {
			fields, err := repo.FieldsByContentTypeID(ctx, typeID)
			if err != nil {
				t.Fatal(err)
			}
			return fields
		}
		widgets := func(storageID int) []string {
			ins, err := client.FieldInstanceDef.Query().Order(generated.Asc("id")).All(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var out []string
			for _, in := range ins {
				if in.FieldStorageID == storageID {
					w, _ := in.Config["widget"].(string)
					out = append(out, w)
				}
			}
			return out
		}
		storage := func(id int) *generated.FieldStorageDef {
			st, err := client.FieldStorageDef.Get(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			return st
		}

		articles, pages := newType("article"), newType("page")
		color := newStorage("field_color", entity.FieldTypeList, map[string]any{"allowed_values": []any{"red", "blue"}})
		attach(articles, color, "radio")
		attach(pages, color, "select")
		seedNode(t, db, 1, "article", "One", "article/one")
		seedNode(t, db, 2, "page", "Two", "page/two")
		for node, typ := range map[int]int{1: articles, 2: pages} {
			fields := fieldsOf(typ)
			if err := repo.SetFieldValues(ctx, node, fields, map[int]any{fields[0].ID: "red"}, nil); err != nil {
				t.Fatal(err)
			}
		}

		// One → several: the values move, read as lists; radio → checkboxes.
		if err := repo.SetFieldMultiple(ctx, color, true); err != nil {
			t.Fatal(err)
		}
		if n, _ := client.FieldValueList.Query().Count(ctx); n != 0 {
			t.Errorf("single-value rows left: %d", n)
		}
		fields := fieldsOf(articles)
		if !fields[0].Multiple() {
			t.Fatal("the field is still single-valued")
		}
		if vals, err := repo.FieldValues(ctx, 1, fields); err != nil || !reflect.DeepEqual(vals[fields[0].ID], []string{"red"}) {
			t.Errorf("node 1 after the move: %#v %v", vals, err)
		}
		if got := widgets(color); !reflect.DeepEqual(got, []string{"checkboxes", "select"}) {
			t.Errorf("widgets: %v", got)
		}

		// A node with two values blocks going back; with one again it moves back.
		if err := repo.SetFieldValues(ctx, 1, fields, map[int]any{fields[0].ID: []string{"red", "blue"}}, nil); err != nil {
			t.Fatal(err)
		}
		if ids, err := repo.NodesWithSeveralFieldValues(ctx, entity.FieldStorageDef{ID: color, FieldType: entity.FieldTypeList}); err != nil || !reflect.DeepEqual(ids, []int{1}) {
			t.Errorf("several values: %v %v", ids, err)
		}
		if err := repo.SetFieldValues(ctx, 1, fields, map[int]any{fields[0].ID: []string{"blue"}}, nil); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetFieldMultiple(ctx, color, false); err != nil {
			t.Fatal(err)
		}
		if n, _ := client.FieldValueListMulti.Query().Count(ctx); n != 0 {
			t.Errorf("multi-value rows left: %d", n)
		}
		fields = fieldsOf(articles)
		if vals, err := repo.FieldValues(ctx, 1, fields); err != nil || vals[fields[0].ID] != "blue" {
			t.Errorf("node 1 back: %#v %v", vals, err)
		}
		if got := widgets(color); !reflect.DeepEqual(got, []string{"radio", "select"}) {
			t.Errorf("widgets back: %v", got)
		}
		if _, ok := storage(color).Config["multiple"]; ok {
			t.Error("config still has multiple")
		}

		// A mutual reference (saving 1 → 2 adds 2 → 1): to one value it
		// loses "symmetric"; back to several, each target gets delta 0.
		related := newStorage("field_related", entity.FieldTypeReference, map[string]any{"multiple": true, "symmetric": true})
		attach(articles, related, "")
		fields = fieldsOf(articles)
		var ref entity.FieldInstanceDef
		for _, f := range fields {
			if f.FieldStorageID == related {
				ref = f
			}
		}
		if err := repo.SetFieldValues(ctx, 1, entity.FieldInstanceDefs{ref}, map[int]any{ref.ID: []int{2}}, nil); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetFieldMultiple(ctx, related, false); err != nil {
			t.Fatal(err)
		}
		if cfg := storage(related).Config; cfg["symmetric"] != nil || cfg["multiple"] != nil {
			t.Errorf("reference config: %v", cfg)
		}
		if rows, err := client.FieldValueReference.Query().Order(generated.Asc("id")).All(ctx); err != nil || len(rows) != 2 || rows[0].NodeID != 1 || rows[0].TargetNodeID != 2 || rows[1].TargetNodeID != 1 {
			t.Errorf("single reference rows: %+v %v", rows, err)
		}
		if err := repo.SetFieldMultiple(ctx, related, true); err != nil {
			t.Fatal(err)
		}
		if rows, err := client.FieldValueReferenceMulti.Query().Order(generated.Asc("id")).All(ctx); err != nil || len(rows) != 2 || rows[0].TargetNodeID != 2 || rows[0].Delta != 0 || rows[1].Delta != 0 {
			t.Errorf("multi reference rows: %+v %v", rows, err)
		}
	})
}

// TestLinkFieldSeveralValues - a link field keeps its links in the
// editor's order in its one table; one value saves the first link only; a
// node with several blocks going to one value, which leaves the rows where
// they are.
func TestLinkFieldSeveralValues(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		repo := database.NewContentTypeRepository(client)
		now := time.Now()
		ct, err := client.ContentTypeDef.Create().SetMachineName("film").SetLabel("Film").
			SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_trailer").SetFieldType(string(entity.FieldTypeLink)).
			SetConfig(map[string]any{"multiple": true}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.FieldInstanceDef.Create().SetContentTypeID(ct.ID).SetFieldStorageID(st.ID).SetLabel("Trailer").
			SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
			t.Fatal(err)
		}
		seedNode(t, db, 1, "film", "One", "film/one")
		fields, err := repo.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			t.Fatal(err)
		}
		id := fields[0].ID
		links := []entity.Link{{URL: "https://video.example/b", Title: "B"}, {URL: "https://video.example/a"}}
		if err := repo.SetFieldValues(ctx, 1, fields, map[int]any{id: links}, nil); err != nil {
			t.Fatal(err)
		}
		if vals, err := repo.FieldValues(ctx, 1, fields); err != nil || !reflect.DeepEqual(vals[id], links) {
			t.Errorf("links: %#v %v", vals[id], err)
		}
		if ids, err := repo.NodesWithSeveralFieldValues(ctx, entity.FieldStorageDef{ID: st.ID, FieldType: entity.FieldTypeLink}); err != nil || !reflect.DeepEqual(ids, []int{1}) {
			t.Errorf("several values: %v %v", ids, err)
		}

		if err := repo.SetFieldValues(ctx, 1, fields, map[int]any{id: links[1:]}, nil); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetFieldMultiple(ctx, st.ID, false); err != nil {
			t.Fatal(err)
		}
		fields, _ = repo.FieldsByContentTypeID(ctx, ct.ID)
		if fields[0].Multiple() {
			t.Fatal("the field still holds several values")
		}
		if vals, err := repo.FieldValues(ctx, 1, fields); err != nil || !reflect.DeepEqual(vals[id], links[1:]) {
			t.Errorf("one link after the switch: %#v %v", vals[id], err)
		}
		// One value: the first link only.
		if err := repo.SetFieldValues(ctx, 1, fields, map[int]any{id: links}, nil); err != nil {
			t.Fatal(err)
		}
		if n, _ := client.FieldValueLink.Query().Count(ctx); n != 1 {
			t.Errorf("rows of one value: %d", n)
		}
		// Cleared: no rows.
		if err := repo.SetFieldValues(ctx, 1, fields, map[int]any{}, nil); err != nil {
			t.Fatal(err)
		}
		if n, _ := client.FieldValueLink.Query().Count(ctx); n != 0 {
			t.Errorf("rows left: %d", n)
		}
	})
}

// TestImageFieldSeveralValues - an image field with several images keeps
// them in the editor's order, once each; it goes to one image and back,
// the files moving between the tables; the node's files and the files in
// use count them.
func TestImageFieldSeveralValues(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		repo := database.NewContentTypeRepository(client)
		now := time.Now()
		ct, err := client.ContentTypeDef.Create().SetMachineName("article").SetLabel("Article").
			SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_photos").SetFieldType(string(entity.FieldTypeImage)).
			SetConfig(map[string]any{"multiple": true}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.FieldInstanceDef.Create().SetContentTypeID(ct.ID).SetFieldStorageID(st.ID).SetLabel("Photos").
			SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
			t.Fatal(err)
		}
		var files []int
		for _, name := range []string{"a", "b", "c"} {
			f, err := client.File.Create().SetURI("public://" + name + ".jpg").SetMimeType("image/jpeg").Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, f.ID)
		}
		seedNode(t, db, 1, "article", "One", "article/one")
		fields, err := repo.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			t.Fatal(err)
		}
		id := fields[0].ID

		order := []int{files[2], files[0], files[2], files[1]} // c, a, (c again), b
		if err := repo.SetFieldValues(ctx, 1, fields, map[int]any{id: order}, nil); err != nil {
			t.Fatal(err)
		}
		want := []int{files[2], files[0], files[1]}
		if vals, err := repo.FieldValues(ctx, 1, fields); err != nil || !reflect.DeepEqual(vals[id], want) {
			t.Errorf("saved: %#v %v, want %v", vals[id], err, want)
		}
		uc := database.NewUserContentRepository(db.DB)
		if got, err := uc.NodeFileIDs(ctx, []int{1}); err != nil || len(got) != 3 {
			t.Errorf("the node's files: %v %v", got, err)
		}
		if uris, err := uc.DeleteUnreferencedFiles(ctx, files); err != nil || len(uris) != 0 {
			t.Errorf("files in use deleted: %v %v", uris, err)
		}

		// Several → one is refused while a node has several; with one it moves.
		if ids, err := repo.NodesWithSeveralFieldValues(ctx, entity.FieldStorageDef{ID: st.ID, FieldType: entity.FieldTypeImage}); err != nil || !reflect.DeepEqual(ids, []int{1}) {
			t.Errorf("several values: %v %v", ids, err)
		}
		if err := repo.SetFieldValues(ctx, 1, fields, map[int]any{id: []int{files[1]}}, nil); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetFieldMultiple(ctx, st.ID, false); err != nil {
			t.Fatal(err)
		}
		fields, _ = repo.FieldsByContentTypeID(ctx, ct.ID)
		if vals, err := repo.FieldValues(ctx, 1, fields); err != nil || vals[id] != files[1] {
			t.Errorf("one image: %#v %v", vals[id], err)
		}
		if err := repo.SetFieldMultiple(ctx, st.ID, true); err != nil {
			t.Fatal(err)
		}
		fields, _ = repo.FieldsByContentTypeID(ctx, ct.ID)
		if vals, err := repo.FieldValues(ctx, 1, fields); err != nil || !reflect.DeepEqual(vals[id], []int{files[1]}) {
			t.Errorf("several again: %#v %v", vals[id], err)
		}
	})
}

// TestTaxonomyFieldCardinality - a taxonomy field's terms move to the
// table of several values and back with SetFieldMultiple.
func TestTaxonomyFieldCardinality(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		repo := database.NewContentTypeRepository(client)
		now := time.Now()
		ct, err := client.ContentTypeDef.Create().SetMachineName("article").SetLabel("Article").
			SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		vocab, err := client.TaxonomyVocabulary.Create().SetName("Tags").SetSlug("tags").SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var terms []int
		for _, name := range []string{"red", "blue"} {
			term, err := client.TaxonomyTermData.Create().SetName(name).SetSlug("tags/" + name).SetVid(vocab.ID).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			terms = append(terms, term.ID)
		}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_tags").SetFieldType(string(entity.FieldTypeTaxonomy)).
			SetConfig(map[string]any{"vocabulary_id": vocab.ID}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.FieldInstanceDef.Create().SetContentTypeID(ct.ID).SetFieldStorageID(st.ID).SetLabel("Tags").
			SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
			t.Fatal(err)
		}
		seedNode(t, db, 1, "article", "One", "article/one")
		seedNode(t, db, 2, "article", "Two", "article/two")
		fields, err := repo.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			t.Fatal(err)
		}
		id := fields[0].ID
		for nid, term := range map[int]int{1: terms[0], 2: terms[1]} {
			if err := repo.SetFieldValues(ctx, nid, fields, map[int]any{id: term}, nil); err != nil {
				t.Fatal(err)
			}
		}

		if err := repo.SetFieldMultiple(ctx, st.ID, true); err != nil {
			t.Fatal(err)
		}
		multi, err := repo.FieldTaxonomyMultiValuesForNodes(ctx, st.ID, []int{1, 2})
		if err != nil || !reflect.DeepEqual(multi, map[int][]int{1: {terms[0]}, 2: {terms[1]}}) {
			t.Errorf("several values: %v %v", multi, err)
		}
		if single, err := repo.FieldTaxonomyValuesForNodes(ctx, st.ID, []int{1, 2}); err != nil || len(single) != 0 {
			t.Errorf("left in the single table: %v %v", single, err)
		}

		if err := repo.SetFieldMultiple(ctx, st.ID, false); err != nil {
			t.Fatal(err)
		}
		single, err := repo.FieldTaxonomyValuesForNodes(ctx, st.ID, []int{1, 2})
		if err != nil || !reflect.DeepEqual(single, map[int]int{1: terms[0], 2: terms[1]}) {
			t.Errorf("one value again: %v %v", single, err)
		}
	})
}
