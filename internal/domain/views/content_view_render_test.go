package views

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func storageID(v int) *int { return &v }

func TestBuildContentFieldInstances(t *testing.T) {
	fieldStorages := map[int]entity.FieldStorageDef{
		1: {ID: 1, MachineName: "publisher", FieldType: entity.FieldTypeText, Config: map[string]any{"multiple": false}},
		2: {ID: 2, MachineName: "tags", FieldType: entity.FieldTypeTaxonomy},
	}

	t.Run("one field instance per View field, in order", func(t *testing.T) {
		viewFields := entity.ViewFieldDefs{
			{ID: 10, FieldStorageID: storageID(1), Label: "Publisher", ShowLabel: true, CSSClass: "custom"},
			{ID: 11, FieldStorageID: storageID(2), Label: "Tags", TagFormat: entity.ViewTagFormatBadges},
		}
		got := BuildContentFieldInstances(t.Context(), viewFields, fieldStorages, nil, nil)
		if len(got) != 2 {
			t.Fatalf("got %d instances, want 2", len(got))
		}
		first := got[0]
		if first.ID != 1 || first.FieldStorageID != 1 {
			t.Fatalf("first ID/FieldStorageID = %d/%d, want 1/1 (FieldValuesForNodes keys by ID)", first.ID, first.FieldStorageID)
		}
		if first.MachineName != "publisher" || first.Label != "Publisher" || !first.ShowLabelInList {
			t.Fatalf("first = %+v", first)
		}
		if first.Config["css_class"] != "custom" || first.Config["multiple"] != false {
			t.Fatalf("first Config = %+v, want the View's css_class and the storage's own keys", first.Config)
		}
		if got[1].Config["tag_format"] != string(entity.ViewTagFormatBadges) || got[1].ShowLabelInList {
			t.Fatalf("second = %+v", got[1])
		}
	})

	t.Run("a field whose storage is missing is left out", func(t *testing.T) {
		viewFields := entity.ViewFieldDefs{{ID: 10, FieldStorageID: storageID(1)}, {ID: 12, FieldStorageID: storageID(999)}}
		got := BuildContentFieldInstances(t.Context(), viewFields, fieldStorages, nil, nil)
		if len(got) != 1 || got[0].FieldStorageID != 1 {
			t.Fatalf("got %+v, want only storage 1", got)
		}
	})

	t.Run("the cached storage's Config is not changed", func(t *testing.T) {
		BuildContentFieldInstances(t.Context(), entity.ViewFieldDefs{{ID: 10, FieldStorageID: storageID(1), CSSClass: "x"}}, fieldStorages, nil, nil)
		if _, ok := fieldStorages[1].Config["css_class"]; ok {
			t.Fatal("css_class leaked into the storage's own Config")
		}
	})

	t.Run("label: the View's, else the field's own name, else the machine name", func(t *testing.T) {
		viewFields := entity.ViewFieldDefs{{ID: 10, FieldStorageID: storageID(1)}, {ID: 11, FieldStorageID: storageID(2)}}
		got := BuildContentFieldInstances(t.Context(), viewFields, fieldStorages, map[int]string{1: "Publishing house"}, nil)
		if got[0].Label != "Publishing house" || got[1].Label != "tags" {
			t.Fatalf("labels = %q, %q", got[0].Label, got[1].Label)
		}
	})

	t.Run("builtins take sentinel IDs without a storage", func(t *testing.T) {
		got := BuildContentFieldInstances(t.Context(), entity.ViewFieldDefs{{ID: 20, BuiltinField: entity.ViewFieldBuiltinTitle, ShowLabel: true}}, nil, nil, nil)
		if len(got) != 1 {
			t.Fatalf("got %d instances, want 1", len(got))
		}
		title := got[0]
		if title.ID != contentViewBuiltinTitleID || title.FieldStorageID != contentViewBuiltinTitleID {
			t.Fatalf("title ID/FieldStorageID = %d/%d", title.ID, title.FieldStorageID)
		}
		if title.Label != "Title" || title.FieldType != entity.FieldTypeText || !title.ShowLabelInList {
			t.Fatalf("title = %+v", title)
		}
	})

	t.Run("link_to for generic and builtin fields", func(t *testing.T) {
		viewFields := entity.ViewFieldDefs{
			{ID: 10, FieldStorageID: storageID(1), LinkTo: entity.ViewFieldLinkContent},
			{ID: 20, BuiltinField: entity.ViewFieldBuiltinTitle, LinkTo: entity.ViewFieldLinkFile},
		}
		got := BuildContentFieldInstances(t.Context(), viewFields, fieldStorages, nil, nil)
		if got[0].Config["link_to"] != string(entity.ViewFieldLinkContent) || got[1].Config["link_to"] != string(entity.ViewFieldLinkFile) {
			t.Fatalf("link_to = %v, %v", got[0].Config["link_to"], got[1].Config["link_to"])
		}
	})

	t.Run("text is HTML only for storages holding it; the title is plain", func(t *testing.T) {
		storages := map[int]entity.FieldStorageDef{
			1: {ID: 1, MachineName: "plain", FieldType: entity.FieldTypeText},
			3: {ID: 3, MachineName: "body", FieldType: entity.FieldTypeText},
		}
		viewFields := entity.ViewFieldDefs{{ID: 10, FieldStorageID: storageID(1)}, {ID: 11, FieldStorageID: storageID(3)}, {ID: 20, BuiltinField: entity.ViewFieldBuiltinTitle}}
		got := BuildContentFieldInstances(t.Context(), viewFields, storages, nil, map[int]bool{3: true})
		if got[0].RichText() || !got[1].RichText() || got[2].RichText() {
			t.Fatalf("RichText plain/body/title = %v/%v/%v, want false/true/false", got[0].RichText(), got[1].RichText(), got[2].RichText())
		}
	})
}

// fakeListTypes - the ContentTypeRepository BuildContentList reads; any
// other method panics (the embedded nil interface).
type fakeListTypes struct {
	entity.ContentTypeRepository
	storages map[int]entity.FieldStorageDef
	labels   map[int]string
	rich     map[int]bool
	types    entity.ContentTypeDefs
	values   map[int]map[int]any // node ID → storage ID → value
	valueErr error
	// fieldsByType - content type ID → its fields.
	fieldsByType map[int]entity.FieldInstanceDefs
}

func (f *fakeListTypes) FieldStorageByID(_ context.Context, id int) (*entity.FieldStorageDef, error) {
	fs, ok := f.storages[id]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	return &fs, nil
}

func (f *fakeListTypes) FieldInstanceLabelsByFieldStorageIDs(context.Context, []int) (map[int]string, error) {
	return f.labels, nil
}

func (f *fakeListTypes) RichTextFieldStorageIDs(context.Context, []int) (map[int]bool, error) {
	return f.rich, nil
}

func (f *fakeListTypes) ContentTypes(context.Context) (entity.ContentTypeDefs, error) {
	return f.types, nil
}

func (f *fakeListTypes) FieldValuesForNodes(_ context.Context, nodeIDs []int, fields entity.FieldInstanceDefs) (map[int]map[int]any, error) {
	if f.valueErr != nil {
		return nil, f.valueErr
	}
	out := map[int]map[int]any{}
	for _, nid := range nodeIDs {
		out[nid] = map[int]any{}
		for _, fd := range fields {
			if v, ok := f.values[nid][fd.FieldStorageID]; ok {
				out[nid][fd.ID] = v
			}
		}
	}
	return out, nil
}

// FieldsByContentTypeID - the type's fields (fieldsByType).
func (f *fakeListTypes) FieldsByContentTypeID(_ context.Context, id int) (entity.FieldInstanceDefs, error) {
	return f.fieldsByType[id], nil
}

type fakeListNodes struct {
	entity.NodesRepository
	alt   map[int]string
	calls int
}

func (f *fakeListNodes) AltTitles(context.Context, entity.Nodes) (map[int]string, error) {
	f.calls++
	return f.alt, nil
}

type fakeListComments map[int]int

func (f fakeListComments) CountByNodeIDs(context.Context, []int, int8, bool) (map[int]int, error) {
	return f, nil
}

// TestBuildContentList - the one list of a content View: fields with
// their labels and HTML-ness from the storages, values batched across the
// nodes, builtins from the node; a View field whose storage is gone is
// left out; alternative titles only when asked (View blocks).
func TestBuildContentList(t *testing.T) {
	premiere := time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)
	types := &fakeListTypes{
		storages: map[int]entity.FieldStorageDef{
			10: {ID: 10, MachineName: "premiere", FieldType: entity.FieldTypeDate},
			11: {ID: 11, MachineName: "body", FieldType: entity.FieldTypeText},
		},
		labels: map[int]string{10: "Premiere"},
		rich:   map[int]bool{11: true},
		types:  entity.ContentTypeDefs{{MachineName: "movie", Label: "Movie"}},
		values: map[int]map[int]any{1: {10: premiere, 11: "<p>Plot</p>"}},
	}
	viewFields := entity.ViewFieldDefs{
		{ID: 4, BuiltinField: entity.ViewFieldBuiltinCommentCount, SortOrder: 40},
		{ID: 1, BuiltinField: entity.ViewFieldBuiltinTitle, SortOrder: 10},
		{ID: 2, FieldStorageID: storageID(10), SortOrder: 20},
		{ID: 3, FieldStorageID: storageID(11), SortOrder: 30},
		{ID: 5, FieldStorageID: storageID(404), SortOrder: 50},
		{ID: 6, BuiltinField: entity.ViewFieldBuiltinContentType, SortOrder: 60},
	}
	nodes := entity.Nodes{{ID: 1, Type: "movie", Title: "Moby"}, {ID: 2, Type: "movie", Title: "Ahab"}}

	for _, tc := range []struct {
		name      string
		altTitles bool
		want      [2]string
	}{
		{"page", false, [2]string{"Moby", "Ahab"}},
		{"block", true, [2]string{"The Whale", "Ahab"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodeRepo := &fakeListNodes{alt: map[int]string{1: "The Whale"}}
			src := ContentListSources{Nodes: nodeRepo, Types: types, Comments: fakeListComments{2: 7}}
			fields, items, err := BuildContentList(t.Context(), src, 1, viewFields, nodes, ContentListOptions{AltTitles: tc.altTitles})
			if err != nil {
				t.Fatal(err)
			}
			if tc.altTitles != (nodeRepo.calls == 1) {
				t.Fatalf("AltTitles calls = %d with AltTitles %v", nodeRepo.calls, tc.altTitles)
			}
			var names []string
			for _, f := range fields {
				names = append(names, f.MachineName+"/"+f.Label)
			}
			if want := "[title/Title premiere/Premiere body/body comment_count/Comments content_type/Content type]"; fmt.Sprint(names) != want {
				t.Fatalf("fields = %v, want %s", names, want)
			}
			if !fields[2].RichText() {
				t.Fatal("the HTML text storage renders as plain text")
			}
			if len(items) != 2 {
				t.Fatalf("got %d items", len(items))
			}
			for i, item := range items {
				if item.Values[contentViewBuiltinTitleID] != tc.want[i] {
					t.Errorf("item %d title = %v, want %s", i, item.Values[contentViewBuiltinTitleID], tc.want[i])
				}
				if item.Values[contentViewBuiltinContentTypeID] != "Movie" {
					t.Errorf("item %d content type = %v", i, item.Values[contentViewBuiltinContentTypeID])
				}
			}
			if got, _ := items[0].Values[10].(time.Time); !got.Equal(premiere) || items[0].Values[11] != "<p>Plot</p>" {
				t.Errorf("item 1 values = %v", items[0].Values)
			}
			if items[0].Values[contentViewBuiltinCommentCountID] != 0 || items[1].Values[contentViewBuiltinCommentCountID] != 7 {
				t.Errorf("comment counts = %v, %v", items[0].Values[contentViewBuiltinCommentCountID], items[1].Values[contentViewBuiltinCommentCountID])
			}
		})
	}

	t.Run("a failure to load the values is returned", func(t *testing.T) {
		boom := errors.New("boom")
		failing := *types
		failing.valueErr = boom
		_, _, err := BuildContentList(t.Context(), ContentListSources{Types: &failing}, 1, viewFields, nodes, ContentListOptions{})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want boom", err)
		}
	})

	t.Run("without sources only builtins are listed", func(t *testing.T) {
		fields, items, err := BuildContentList(t.Context(), ContentListSources{}, 1, viewFields, nodes, ContentListOptions{AltTitles: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(fields) != 3 || items[0].Values[contentViewBuiltinTitleID] != "Moby" {
			t.Fatalf("fields = %+v, items = %+v", fields, items)
		}
	})
}

// An image field without a style shows nothing: there is no derivative
// to point at.
func TestBuildContentListItemsImageWithoutStyle(t *testing.T) {
	types := &fakeListTypes{values: map[int]map[int]any{1: {17: 90}}}
	listFields := BuildContentFieldInstances(t.Context(), entity.ViewFieldDefs{{ID: 1, FieldStorageID: storageID(17)}},
		map[int]entity.FieldStorageDef{17: {ID: 17, MachineName: "cover", FieldType: entity.FieldTypeImage}}, nil, nil)
	files := fakeListFiles{90: {ID: 90, URI: "public://cover.png"}}
	items, err := BuildContentListItems(t.Context(), types, entity.Nodes{{ID: 1, Type: "movie"}}, listFields, nil, nil, nil, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items[0].ImageFieldURLs) != 0 {
		t.Fatalf("image URLs = %v, want none", items[0].ImageFieldURLs)
	}

	listFields[0].ListImageStyle = "thumbnail"
	items, err = BuildContentListItems(t.Context(), types, entity.Nodes{{ID: 1, Type: "movie"}}, listFields, nil, nil, nil, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := items[0].ImageFieldURLs[17]; got != "/uploads/styles/thumbnail/cover.webp" {
		t.Errorf("styled URL = %q", got)
	}
	if got := items[0].ImageFieldOriginalURLs[17]; got != "/uploads/cover.png" {
		t.Errorf("original URL = %q", got)
	}
}

// A field with several images: the first stands for it (the URL maps);
// all of them, in order, are in ImageFieldImages for "image_display" "all".
func TestBuildContentListItemsSeveralImages(t *testing.T) {
	types := &fakeListTypes{values: map[int]map[int]any{1: {17: []int{92, 90}}}}
	listFields := BuildContentFieldInstances(t.Context(), entity.ViewFieldDefs{{ID: 1, FieldStorageID: storageID(17), ImageStyle: "thumbnail", ImageDisplay: entity.ViewImageAll}},
		map[int]entity.FieldStorageDef{17: {ID: 17, MachineName: "photos", FieldType: entity.FieldTypeImage, Config: map[string]any{"multiple": true}}}, nil, nil)
	listFields[0].ListImageStyle = "thumbnail"
	files := fakeListFiles{90: {ID: 90, URI: "public://a.png", Alt: "A"}, 92: {ID: 92, URI: "public://b.png", Alt: "B"}}
	items, err := BuildContentListItems(t.Context(), types, entity.Nodes{{ID: 1, Type: "movie"}}, listFields, nil, nil, nil, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].ImageFieldAlts[17] != "B" || items[0].ImageFieldURLs[17] != "/uploads/styles/thumbnail/b.webp" {
		t.Errorf("the first image: %q %q", items[0].ImageFieldURLs[17], items[0].ImageFieldAlts[17])
	}
	all := items[0].ImageFieldImages[17]
	if len(all) != 2 || all[0].Alt != "B" || all[1].Alt != "A" {
		t.Errorf("all images: %+v", all)
	}
	if listFields[0].Config["image_display"] != "all" {
		t.Errorf("image_display config: %v", listFields[0].Config["image_display"])
	}
}

type fakeListFiles map[int]entity.File

func (f fakeListFiles) ByIDs(context.Context, []int) (map[int]entity.File, error) { return f, nil }

// TestContentViewBuiltinsHandled - every builtin field a content View may
// show (contentBuiltins) has its instance and value in viewBuiltins,
// under an ID of its own.
func TestContentViewBuiltinsHandled(t *testing.T) {
	for _, b := range contentBuiltins.fields {
		bi, ok := viewBuiltins[b]
		if !ok || bi.value == nil {
			t.Errorf("%s: not in viewBuiltins (its instance and value)", b)
		}
	}
	if len(viewBuiltinByID) != len(viewBuiltins) {
		t.Errorf("viewBuiltins: %d builtins under %d IDs", len(viewBuiltins), len(viewBuiltinByID))
	}
}

// TestBuildContentListReadingTime - the reading time field: of the long
// text fields a node's type shows on its page, as text; none for a node
// with nothing to read or a type without such fields.
func TestBuildContentListReadingTime(t *testing.T) {
	body := entity.FieldInstanceDef{ID: 31, FieldStorageID: 11, FieldType: entity.FieldTypeText, ShowOnDetail: true, WidgetConfig: map[string]any{"widget": "wysiwyg"}}
	subtitle := entity.FieldInstanceDef{ID: 32, FieldStorageID: 12, FieldType: entity.FieldTypeText, ShowOnDetail: true}
	types := &fakeListTypes{
		types: entity.ContentTypeDefs{{ID: 1, MachineName: "article"}, {ID: 2, MachineName: "link"}},
		fieldsByType: map[int]entity.FieldInstanceDefs{
			1: {body, subtitle},
			2: {subtitle},
		},
		values: map[int]map[int]any{
			1: {11: "<p>" + strings.Repeat("word ", 450) + "</p>", 12: strings.Repeat("word ", 450)},
			3: {12: "A subtitle"},
		},
	}
	viewFields := entity.ViewFieldDefs{{ID: 1, BuiltinField: entity.ViewFieldBuiltinReadingTime}}
	nodes := entity.Nodes{{ID: 1, Type: "article"}, {ID: 2, Type: "article"}, {ID: 3, Type: "link"}}
	fields, items, err := BuildContentList(t.Context(), ContentListSources{Types: types}, 1, viewFields, nodes, ContentListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].MachineName != "reading_time" || fields[0].Label != "Reading time" {
		t.Fatalf("fields = %+v", fields)
	}
	for i, want := range []string{"3 min read", "", ""} {
		if got, _ := items[i].Values[contentViewBuiltinReadingTimeID].(string); got != want {
			t.Errorf("node %d: %v, want %v", nodes[i].ID, got, want)
		}
	}
}
