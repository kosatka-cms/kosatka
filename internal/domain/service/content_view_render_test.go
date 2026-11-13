package service

import (
	"bytes"
	"context"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

// fakeContentTypeRepoForBlocks - a minimal entity.ContentTypeRepository
// for the generic-field batching path. The embedded nil interface makes
// any other method panic if called.
type fakeContentTypeRepoForBlocks struct {
	entity.ContentTypeRepository
	fieldStorages      map[int]entity.FieldStorageDef
	fieldStorageLabels map[int]string
	contentTypes       entity.ContentTypeDefs
	valuesByNode       map[int]map[int]any // nodeID -> fieldStorageID -> value
}

func (r *fakeContentTypeRepoForBlocks) FieldStorageByID(_ context.Context, id int) (*entity.FieldStorageDef, error) {
	fs, ok := r.fieldStorages[id]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	return &fs, nil
}

func (r *fakeContentTypeRepoForBlocks) FieldInstanceLabelsByFieldStorageIDs(_ context.Context, ids []int) (map[int]string, error) {
	out := make(map[int]string, len(ids))
	for _, id := range ids {
		out[id] = r.fieldStorageLabels[id]
	}
	return out, nil
}

func (r *fakeContentTypeRepoForBlocks) ContentTypes(context.Context) (entity.ContentTypeDefs, error) {
	return r.contentTypes, nil
}

func (r *fakeContentTypeRepoForBlocks) FieldValuesForNodes(_ context.Context, nodeIDs []int, fields entity.FieldInstanceDefs) (map[int]map[int]any, error) {
	out := make(map[int]map[int]any, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		out[nodeID] = make(map[int]any)
		for _, f := range fields {
			if v, ok := r.valuesByNode[nodeID][f.FieldStorageID]; ok {
				out[nodeID][f.ID] = v
			}
		}
	}
	return out, nil
}

// TestBuildContentListItems_GenericDateFieldAndContentTypeBuiltin - a
// generic date field resolved via the batched FieldValuesForNodes path
// plus the content_type builtin from a label map, with a nil nodeRepo.
func TestBuildContentListItems_GenericDateFieldAndContentTypeBuiltin(t *testing.T) {
	premiereDate := time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)
	repo := &fakeContentTypeRepoForBlocks{
		fieldStorages:      map[int]entity.FieldStorageDef{10: {ID: 10, MachineName: "premiere_us", FieldType: entity.FieldTypeDate}, 11: {ID: 11, MachineName: "cover_image", FieldType: entity.FieldTypeImage}},
		fieldStorageLabels: map[int]string{10: "World premiere"},
		valuesByNode: map[int]map[int]any{
			1: {10: premiereDate},
		},
	}

	viewFields := entity.ViewFieldDefs{
		{ID: 1, FieldStorageID: intPtrContentViewTest(11)},
		{ID: 2, FieldStorageID: intPtrContentViewTest(10), ExcludeFromDisplay: true},
		{ID: 3, BuiltinField: entity.ViewFieldBuiltinContentType, ExcludeFromDisplay: true},
	}
	listFields := views.BuildContentFieldInstances(context.Background(), viewFields, repo.fieldStorages, repo.fieldStorageLabels, nil)
	if len(listFields) != 3 {
		t.Fatalf("expected 3 synthetic fields (image + generic date + content_type), got %d: %+v", len(listFields), listFields)
	}

	nodes := entity.Nodes{{ID: 1, Type: "video", Title: "Test video"}}
	contentTypeLabels := map[string]string{"video": "Video"}

	items, err := views.BuildContentListItems(context.Background(), repo, nodes, listFields, contentTypeLabels, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]

	var dateFieldID, contentTypeFieldID int
	for _, f := range item.Fields {
		if f.FieldType == entity.FieldTypeDate {
			dateFieldID = f.ID
		}
		if f.MachineName == string(entity.ViewFieldBuiltinContentType) {
			contentTypeFieldID = f.ID
			if !f.ExcludeFromDisplay {
				t.Fatalf("expected content_type field's ExcludeFromDisplay to survive synthesis, got false")
			}
		}
	}
	if dateFieldID == 0 {
		t.Fatal("expected a synthetic date field in item.Fields")
	}
	gotDate, ok := item.Values[dateFieldID].(time.Time)
	if !ok || !gotDate.Equal(premiereDate) {
		t.Fatalf("got date value %v, want %v (batched, not per-node, resolution)", item.Values[dateFieldID], premiereDate)
	}
	if contentTypeFieldID == 0 {
		t.Fatal("expected a synthetic content_type field in item.Fields")
	}
	if item.Values[contentTypeFieldID] != "Video" {
		t.Fatalf("got content_type value %v, want %q", item.Values[contentTypeFieldID], "Video")
	}
}

func intPtrContentViewTest(v int) *int { return &v }

// TestBuildContentFieldInstances_ImageRewriteHTMLSurvives - an image
// field's Config keeps "rewrite_html" (e.g. a badge over the picture),
// which dynamic/list.html reads via `index .Field.Config "rewrite_html"`.
func TestBuildContentFieldInstances_ImageRewriteHTMLSurvives(t *testing.T) {
	const rewrite = `{{value}}<span class="premiere-badge-date">{{field:premiere_us}}</span>`
	viewFields := entity.ViewFieldDefs{
		{ID: 1, FieldStorageID: intPtrContentViewTest(11), RewriteHTML: rewrite},
	}
	listFields := views.BuildContentFieldInstances(context.Background(), viewFields, map[int]entity.FieldStorageDef{11: {ID: 11, MachineName: "cover_image", FieldType: entity.FieldTypeImage}}, nil, nil)
	if len(listFields) != 1 {
		t.Fatalf("expected 1 synthetic field, got %d: %+v", len(listFields), listFields)
	}
	got, _ := listFields[0].Config["rewrite_html"].(string)
	if got != rewrite {
		t.Fatalf("image field's Config[%q] = %q, want %q", "rewrite_html", got, rewrite)
	}
}

// TestDynamicListFieldValue_CrossFieldRewriteToken - a RewriteHTML on
// MainImage uses a {{field:...}} token from a different, ExcludeFromDisplay
// field of the same item (a badge over the image).
func TestDynamicListFieldValue_CrossFieldRewriteToken(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	imageField := entity.FieldInstanceDef{
		ID: 1, MachineName: "main_image", FieldType: entity.FieldTypeImage,
		Config: map[string]any{"rewrite_html": `{{value}}<span class="premiere-badge-date">{{field:premiere_us}}</span>`},
	}
	dateField := entity.FieldInstanceDef{
		ID: 2, MachineName: "premiere_us", FieldType: entity.FieldTypeDate, ExcludeFromDisplay: true,
	}
	listFields := entity.FieldInstanceDefs{imageField, dateField}

	item := views.ContentListItem{
		Node:               entity.Node{ID: 1, Slug: "harbour"},
		Fields:             listFields,
		Values:             map[int]any{2: time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)},
		ImageFieldURLs:     map[int]string{1: "/files/harbour.jpg"},
		ImageFieldPictures: map[int]imageurl.Picture{1: {Src: "/files/harbour.jpg"}},
		ImageFieldAlts:     map[int]string{1: "Cover"},
	}

	data := map[string]any{
		"items":          []views.ContentListItem{item},
		"listFieldsHead": listFields,
		"displayStyle":   "grid",
		"showPagination": false,
		"isViewList":     true,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte(`src="/files/harbour.jpg"`)) {
		t.Fatalf("expected the image itself to still render via {{value}}, got: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`<span class="premiere-badge-date">`)) {
		t.Fatalf("expected the RewriteHTML wrapper markup, got: %s", out)
	}
	// The ExcludeFromDisplay date field must never render its OWN slot.
	if bytes.Contains(buf.Bytes(), []byte(`item-field-date`)) {
		t.Fatalf("expected the ExcludeFromDisplay date field to render no slot of its own, got: %s", out)
	}
}

// TestDynamicListContainerID_ViewBlockGetsUniqueID - a page-level View
// keeps #view-list-container, while View blocks (isViewBlock+viewID) get
// an id unique per View, so a page's CustomCSS scoped under
// #view-list-container doesn't leak into blocks on the same page.
func TestDynamicListContainerID_ViewBlockGetsUniqueID(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	render := func(data map[string]any) string {
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
			t.Fatalf("render error: %v", err)
		}
		return buf.String()
	}

	pageOut := render(map[string]any{
		"items": []views.ContentListItem{}, "listFieldsHead": entity.FieldInstanceDefs{}, "displayStyle": "grid", "showPagination": false, "isViewList": true,
	})
	if !strings.Contains(pageOut, `id="view-list-container"`) {
		t.Fatalf("page-level render: expected unchanged id=\"view-list-container\", got: %s", pageOut)
	}

	blockOut := render(map[string]any{
		"items": []views.ContentListItem{}, "listFieldsHead": entity.FieldInstanceDefs{}, "displayStyle": "grid", "showPagination": false, "isViewList": true,
		"isViewBlock": true, "viewID": 116,
	})
	if !strings.Contains(blockOut, `id="view-list-container-block-116"`) {
		t.Fatalf("block render: expected a viewID-scoped id, got: %s", blockOut)
	}
	if strings.Contains(blockOut, `id="view-list-container"`) {
		t.Fatalf("block render: must NOT also carry the shared page-level id, got: %s", blockOut)
	}
}

// An image storage field in a cached/contextual View block renders its
// own file in the View field's style (not silently dropped).
func TestBuildContentImageFieldInstance(t *testing.T) {
	cover := 17
	viewFields := entity.ViewFieldDefs{{ID: 1, FieldStorageID: &cover, Label: "Image", ImageStyle: "news_list", LinkTo: entity.ViewFieldLinkContent}}
	storages := map[int]entity.FieldStorageDef{17: {ID: 17, MachineName: "cover_image", FieldType: entity.FieldTypeImage}}
	listFields := views.BuildContentFieldInstances(context.Background(), viewFields, storages, nil, nil)
	if len(listFields) != 1 || listFields[0].FieldType != entity.FieldTypeImage || listFields[0].ListImageStyle != "news_list" {
		t.Fatalf("listFields = %+v", listFields)
	}
}

type fakeViewTaxonomyRepo struct {
	entity.ContentTypeRepository
	single map[int]map[int]int
}

func (f fakeViewTaxonomyRepo) FieldTaxonomyValuesForNodes(_ context.Context, storageID int, _ []int) (map[int]int, error) {
	return f.single[storageID], nil
}

type fakeViewTerms map[int]entity.TaxonomyTerm

func (f fakeViewTerms) TermsByIDs(context.Context, []int) (map[int]entity.TaxonomyTerm, error) {
	return f, nil
}

// TestBuildContentListItemsTaxonomyWithFallback - a taxonomy View field
// in a block shows its term; a node without the term falls back to the
// content type label (FallbackFieldKey "content_type"), e.g. as a type
// badge.
func TestBuildContentListItemsTaxonomyWithFallback(t *testing.T) {
	storageID := 107
	fallback := string(entity.ViewFieldBuiltinContentType)
	viewFields := entity.ViewFieldDefs{
		{FieldStorageID: &storageID, ExcludeFromDisplay: true, FallbackFieldKey: fallback, SortOrder: 10},
		{BuiltinField: entity.ViewFieldBuiltinContentType, ExcludeFromDisplay: true, SortOrder: 20},
	}
	storages := map[int]entity.FieldStorageDef{storageID: {ID: storageID, MachineName: "field_kind", FieldType: entity.FieldTypeTaxonomy, Config: map[string]any{"vocabulary_id": float64(30)}}}
	listFields := views.BuildContentFieldInstances(context.Background(), viewFields, storages, nil, nil)
	if len(listFields) != 2 || listFields[0].FieldType != entity.FieldTypeTaxonomy {
		t.Fatalf("taxonomy field not built: %+v", listFields)
	}

	nodes := entity.Nodes{{ID: 1, Type: "video"}, {ID: 2, Type: "product"}}
	repo := fakeViewTaxonomyRepo{single: map[int]map[int]int{storageID: {1: 2848}}}
	terms := fakeViewTerms{2848: {ID: 2848, Name: "Kind A", Slug: "vidy/vid-a"}}
	labels := map[string]string{"video": "Video", "product": "Products"}
	items, err := views.BuildContentListItems(context.Background(), repo, nodes, listFields, labels, terms, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := items[0].TaxonomyFieldTerms[storageID]; len(got) != 1 || got[0].Name != "Kind A" {
		t.Fatalf("video terms = %v", got)
	}
	if got := items[1].Values[storageID]; got != "Products" {
		t.Fatalf("product without term: value = %v, want fallback Products", got)
	}

	// The list template prints the term, or the fallback string.
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("dynamic/list.html", []string{"dynamic", "list.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("list-taxonomy-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"Kind A", "Products"} {
		var buf strings.Builder
		data := map[string]any{"Field": listFields[0], "Value": items[i].Values[storageID], "Item": items[i]}
		if err := set.ExecuteTemplate(&buf, "dynamic_list_field_value_raw", data); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), want) {
			t.Errorf("item %d renders %q, want %q", i, strings.TrimSpace(buf.String()), want)
		}
	}
}

// TestContentListTrimmedText - a text field with a trim length
// (ViewFieldDef.TrimLength) lists as plain text cut at a word, blocks
// apart; without one it lists as before.
func TestContentListTrimmedText(t *testing.T) {
	storageID := 120
	storages := map[int]entity.FieldStorageDef{storageID: {ID: storageID, MachineName: "field_body", FieldType: entity.FieldTypeText}}
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("dynamic/list.html", []string{"dynamic", "list.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("list-trim-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	body := "<p>First paragraph here.</p><p>Second paragraph text.</p>"
	for trim, want := range map[int]string{
		16: `<span class="item-field-value">First paragraph…</span>`,
		0:  `&lt;p&gt;First paragraph here.&lt;/p&gt;`,
	} {
		listFields := views.BuildContentFieldInstances(context.Background(), entity.ViewFieldDefs{{FieldStorageID: &storageID, TrimLength: trim}}, storages, nil, nil)
		var buf strings.Builder
		data := map[string]any{"Field": listFields[0], "Value": body, "Item": views.ContentListItem{}}
		if err := set.ExecuteTemplate(&buf, "dynamic_list_field_value_raw", data); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), want) {
			t.Errorf("trim %d renders %q, want %q", trim, strings.TrimSpace(buf.String()), want)
		}
	}
}

// TestContentListDefaultImage - a content list (views.BuildContentListItems)
// shows the default image of the field on the node's own type; a type
// without one shows nothing.
func TestContentListDefaultImage(t *testing.T) {
	storageID, defaultID := 110, 90
	ct := &fakeTeaserContentTypes{
		defs: entity.ContentTypeDefs{{ID: 1, MachineName: "article"}, {ID: 2, MachineName: "page"}},
		fields: map[int]entity.FieldInstanceDefs{
			1: {{ID: 8, FieldStorageID: storageID, MachineName: "cover_image", FieldType: entity.FieldTypeImage, DefaultImageID: &defaultID}},
			2: {{ID: 9, FieldStorageID: storageID, MachineName: "cover_image", FieldType: entity.FieldTypeImage}},
		},
		values: map[int]map[int]any{},
	}
	viewFields := entity.ViewFieldDefs{{ID: 1, FieldStorageID: &storageID, ImageStyle: "thumbnail"}}
	listFields := views.BuildContentFieldInstances(t.Context(), viewFields, map[int]entity.FieldStorageDef{storageID: {ID: storageID, MachineName: "cover_image", FieldType: entity.FieldTypeImage}}, nil, nil)
	files := fakeTeaserFiles{90: {ID: 90, URI: "public://default.png"}}
	items, err := views.BuildContentListItems(t.Context(), ct, entity.Nodes{{ID: 1, Type: "article"}, {ID: 2, Type: "page"}}, listFields, nil, nil, nil, files)
	if err != nil {
		t.Fatal(err)
	}
	if got := items[0].ImageFieldURLs[listFields[0].ID]; got != "/uploads/styles/thumbnail/default.webp" {
		t.Errorf("article: %q", got)
	}
	if got := items[1].ImageFieldURLs[listFields[0].ID]; got != "" {
		t.Errorf("page: %q", got)
	}
}

// TestContentListAuthorAndLink - the builtin author renders the author's
// name linking their profile (no link without a profile address), the
// builtin link the field's label linking the node, without a label
// before it.
func TestContentListAuthorAndLink(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)

	viewFields := entity.ViewFieldDefs{
		{ID: 1, BuiltinField: entity.ViewFieldBuiltinAuthor, Label: "Posted by", ShowLabel: true},
		{ID: 2, BuiltinField: entity.ViewFieldBuiltinLink, Label: "Read on", ShowLabel: true},
	}
	listFields := views.BuildContentFieldInstances(t.Context(), viewFields, nil, nil, nil)
	nodes := entity.Nodes{
		{ID: 1, Type: "article", Slug: "article/whale", CreatedBy: entity.User{Username: "ishmael", Slug: "ishmael"}},
		{ID: 2, Type: "article", Slug: "article/sea", CreatedBy: entity.User{Username: "starbuck"}},
	}
	items, err := views.BuildContentListItems(t.Context(), &fakeContentTypeRepoForBlocks{}, nodes, listFields, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("BuildContentListItems: %v", err)
	}
	var buf bytes.Buffer
	data := map[string]any{"items": items, "listFieldsHead": listFields, "displayStyle": "list", "isViewList": true}
	if err := svc.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		`<span class="item-field-label">Posted by:</span> <a href="/users/ishmael" class="item-field-link"> <span class="item-field-value">ishmael</span>`,
		`<span class="item-field-label">Posted by:</span> <span class="item-field-value">starbuck</span>`,
		`item-field-name-link"> <a href="/article/whale" class="item-field-link"> <span class="item-field-value">Read on</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %s in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Read on:") {
		t.Errorf("the link shows a label before it:\n%s", out)
	}
}
