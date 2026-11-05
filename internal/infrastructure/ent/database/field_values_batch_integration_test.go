//go:build dbintegration

package database_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestFieldValuesForNodes - the values of every field type for many
// nodes at once (one query per value table): text, number, date, bool,
// list and reference (one value and several, in their order), image and
// file, link (one and several, in their order); a node without values has
// none; a node's own (FieldValues) the same.
func TestFieldValuesForNodes(t *testing.T) {
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
		specs := []struct {
			name   string
			typ    entity.ContentTypeFieldType
			config map[string]any
		}{
			{"field_subtitle", entity.FieldTypeText, map[string]any{}},
			{"field_rating", entity.FieldTypeNumber, map[string]any{}},
			{"field_premiere", entity.FieldTypeDate, map[string]any{}},
			{"field_featured", entity.FieldTypeBool, map[string]any{}},
			{"field_kind", entity.FieldTypeList, map[string]any{"allowed_values": []any{"sloop", "brig"}}},
			{"field_crew", entity.FieldTypeList, map[string]any{"multiple": true, "allowed_values": []any{"a", "b", "c"}}},
			{"field_sequel", entity.FieldTypeReference, map[string]any{}},
			{"field_related", entity.FieldTypeReference, map[string]any{"multiple": true}},
			{"field_cover", entity.FieldTypeImage, map[string]any{}},
			{"field_script", entity.FieldTypeFile, map[string]any{}},
			{"field_site", entity.FieldTypeLink, map[string]any{}},
			{"field_show", entity.FieldTypeDate, map[string]any{"date_time": "optional", "date_end": "optional"}},
			{"field_shows", entity.FieldTypeDate, map[string]any{"multiple": true}},
			{"field_lead", entity.FieldTypeUser, map[string]any{}},
			{"field_mail", entity.FieldTypeEmail, map[string]any{}},
			{"field_phone", entity.FieldTypePhone, map[string]any{}},
			{"field_trailers", entity.FieldTypeLink, map[string]any{"multiple": true}},
		}
		// Every field type is written and read back here - but a group (no
		// value), taxonomy (TestMultiTermOrder) and blocks
		// (TestDynamicContentBlocks).
		for _, ft := range entity.FieldTypes {
			switch ft {
			case entity.FieldTypeGroup, entity.FieldTypeTaxonomy, entity.FieldTypeBlocks:
				continue
			}
			if !slices.ContainsFunc(specs, func(s struct {
				name   string
				typ    entity.ContentTypeFieldType
				config map[string]any
			}) bool {
				return s.typ == ft
			}) {
				t.Errorf("%s: not written and read back here - add a field of it", ft)
			}
		}
		for _, s := range specs {
			st, err := client.FieldStorageDef.Create().SetMachineName(s.name).SetFieldType(string(s.typ)).SetConfig(s.config).
				SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.FieldInstanceDef.Create().SetContentTypeID(ct.ID).SetFieldStorageID(st.ID).SetLabel(s.name).
				SetConfig(map[string]any{}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx); err != nil {
				t.Fatal(err)
			}
		}
		cover, err := client.File.Create().SetURI("public://cover.jpg").SetMimeType("image/jpeg").Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		script, err := client.File.Create().SetURI("public://script.pdf").SetMimeType("application/pdf").Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for id, title := range map[int]string{1: "One", 2: "Two", 3: "Three", 4: "Four"} {
			seedNode(t, db, id, "article", title, "article/"+title)
		}
		fields, err := repo.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			t.Fatal(err)
		}
		premiere := time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)
		full := map[string]any{
			"field_subtitle": "The whale", "field_rating": 4.5, "field_premiere": premiere, "field_featured": true,
			"field_kind": "brig", "field_crew": []string{"c", "a"}, "field_sequel": 2, "field_related": []int{4, 2, 3},
			"field_cover": cover.ID, "field_script": script.ID,
			"field_site":  []entity.Link{{URL: "https://whale.example", Title: "Whale"}},
			"field_lead":  []int{1},
			"field_shows": []entity.DateValue{{Start: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)}, {Start: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)}},
			"field_show": entity.DateValue{Start: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC),
				End: time.Date(2026, 11, 12, 18, 30, 0, 0, time.UTC), WithTime: true},
			"field_trailers": []entity.Link{{URL: "https://video.example/2"}, {URL: "https://video.example/1", Title: "One"}},
		}
		byName := func(set map[string]any) map[int]any {
			out := map[int]any{}
			for _, f := range fields {
				if v, ok := set[f.MachineName]; ok {
					out[f.ID] = v
				}
			}
			return out
		}
		// Node 1 has every field, node 2 a few, nodes 3 and 4 none.
		if err := repo.SetFieldValues(ctx, 1, fields, byName(full), nil); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetFieldValues(ctx, 2, fields, byName(map[string]any{"field_kind": "sloop", "field_related": []int{1}}), nil); err != nil {
			t.Fatal(err)
		}

		// As a View list asks: field instance ID equal to its storage ID.
		listFields := make(entity.FieldInstanceDefs, len(fields))
		for i, f := range fields {
			f.ID = f.FieldStorageID
			listFields[i] = f
		}
		nodeIDs := []int{1, 2, 3}
		batched, err := repo.FieldValuesForNodes(ctx, nodeIDs, listFields)
		if err != nil {
			t.Fatal(err)
		}
		want := map[int]map[string]any{
			1: full,
			2: {"field_kind": "sloop", "field_related": []int{1}},
			3: {},
		}
		for _, nid := range nodeIDs {
			got := map[string]any{}
			for _, f := range listFields {
				if v, ok := batched[nid][f.ID]; ok {
					got[f.MachineName] = v
				}
			}
			if g, ok := got["field_premiere"].(time.Time); ok && g.Equal(premiere) {
				got["field_premiere"] = premiere
			}
			// The instants as the full value's (the driver's location aside).
			if g, ok := got["field_shows"].([]entity.DateValue); ok {
				w := full["field_shows"].([]entity.DateValue)
				if len(g) == len(w) && g[0].Start.Equal(w[0].Start) && g[1].Start.Equal(w[1].Start) {
					got["field_shows"] = w
				}
			}
			if g, ok := got["field_show"].(entity.DateValue); ok {
				w := full["field_show"].(entity.DateValue)
				if g.Start.Equal(w.Start) && g.End.Equal(w.End) && g.WithTime == w.WithTime {
					got["field_show"] = w
				}
			}
			if !reflect.DeepEqual(got, want[nid]) {
				t.Errorf("node %d: %#v, want %#v", nid, got, want[nid])
			}
			// A node's own (the node page's FieldValues) are the same.
			one, err := repo.FieldValues(ctx, nid, listFields)
			if err != nil {
				t.Fatal(err)
			}
			if len(one) != len(batched[nid]) {
				t.Errorf("node %d: %d values by FieldValues, %d for many nodes", nid, len(one), len(batched[nid]))
			}
		}
	})
}

// TestMultiTermOrder - a multi-value taxonomy field's terms come in the
// order saved, not the index's (by term ID): node by node, for many
// nodes, and in the node's terms index; a repeated term is saved once.
func TestMultiTermOrder(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		repo := database.NewContentTypeRepository(client)
		taxonomy := database.NewTaxonomyRepository(client)
		now := time.Now()
		vocab, err := client.TaxonomyVocabulary.Create().SetName("Tags").SetSlug("tags").SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var terms []int
		for _, name := range []string{"alpha", "beta", "gamma"} {
			term, err := client.TaxonomyTermData.Create().SetName(name).SetSlug("tags/" + name).SetVid(vocab.ID).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			terms = append(terms, term.ID)
		}
		ct, err := client.ContentTypeDef.Create().SetMachineName("article").SetLabel("Article").
			SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_tags").SetFieldType(string(entity.FieldTypeTaxonomy)).
			SetConfig(map[string]any{"multiple": true, "vocabulary_id": vocab.ID}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.FieldInstanceDef.Create().SetContentTypeID(ct.ID).SetFieldStorageID(st.ID).SetLabel("Tags").
			SetConfig(map[string]any{}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx); err != nil {
			t.Fatal(err)
		}
		seedNode(t, db, 1, "article", "One", "article/one")
		fields, err := repo.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			t.Fatal(err)
		}
		saved := []int{terms[2], terms[0], terms[1]}
		if err := repo.SetFieldValues(ctx, 1, fields, map[int]any{fields[0].ID: saved}, nil); err != nil {
			t.Fatal(err)
		}
		if err := taxonomy.UpdateNodeTerms(ctx, 1, saved); err != nil {
			t.Fatal(err)
		}

		one, err := repo.FieldValues(ctx, 1, fields)
		if err != nil {
			t.Fatal(err)
		}
		many, err := repo.FieldTaxonomyMultiValuesForNodes(ctx, st.ID, []int{1})
		if err != nil {
			t.Fatal(err)
		}
		index, err := taxonomy.NodeTermsByID(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		var indexed []int
		for _, term := range index {
			indexed = append(indexed, term.ID)
		}
		for name, got := range map[string]any{"one by one": one[fields[0].ID], "for many nodes": many[1], "the terms index": indexed} {
			if !reflect.DeepEqual(got, saved) {
				t.Errorf("%s: %v, want %v", name, got, saved)
			}
		}

		// A term given twice ("5,new:Foo" where Foo is 5) is saved once.
		if err := repo.SetFieldValues(ctx, 1, fields, map[int]any{fields[0].ID: []int{terms[1], terms[0], terms[1]}}, nil); err != nil {
			t.Fatalf("a repeated term: %v", err)
		}
		if again, err := repo.FieldValues(ctx, 1, fields); err != nil || !reflect.DeepEqual(again[fields[0].ID], []int{terms[1], terms[0]}) {
			t.Errorf("a repeated term: %v %v", again[fields[0].ID], err)
		}
	})
}
