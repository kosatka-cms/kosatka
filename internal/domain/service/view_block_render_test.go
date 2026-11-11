package service

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

// fakeListViewService - RunView and the View's fields.
type fakeListViewService struct {
	views.Service
	nodes  entity.Nodes
	fields entity.ViewFieldDefs
}

func (f fakeListViewService) RunConfig(context.Context, views.Config, views.RunContext) (entity.Nodes, int, error) {
	return f.nodes, len(f.nodes), nil
}

func (f fakeListViewService) PublishedConfig(_ context.Context, d entity.ViewDisplayDef) (views.Config, error) {
	return views.Config{View: d, Fields: f.fields}, nil
}

// publishedOf - display's configuration as s's Views service gives it.
func publishedOf(t *testing.T, s *CacheService, d entity.ViewDisplayDef) views.Config {
	t.Helper()
	cfg, err := s.viewService.PublishedConfig(t.Context(), d)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// fakeListContentTypes - fakeContentTypeRepoForBlocks plus which text
// storages hold HTML.
type fakeListContentTypes struct {
	*fakeContentTypeRepoForBlocks
	rich map[int]bool
}

func (f fakeListContentTypes) RichTextFieldStorageIDs(context.Context, []int) (map[int]bool, error) {
	return f.rich, nil
}

type fakeListAltTitles struct {
	entity.NodesRepository
	titles map[int]string
}

func (f fakeListAltTitles) AltTitles(context.Context, entity.Nodes) (map[int]string, error) {
	return f.titles, nil
}

// TestCacheViewBlockList - a View block in the block cache builds its
// list as its page does (views.BuildContentList): the nodes' alternative
// titles linking them, a rich text field as HTML, a plain one escaped, a
// container id of its own.
func TestCacheViewBlockList(t *testing.T) {
	zzChdirToProjectRoot(t)
	body, note := 21, 22
	types := fakeListContentTypes{
		fakeContentTypeRepoForBlocks: &fakeContentTypeRepoForBlocks{
			fieldStorages: map[int]entity.FieldStorageDef{
				body: {ID: body, MachineName: "body", FieldType: entity.FieldTypeText},
				note: {ID: note, MachineName: "note", FieldType: entity.FieldTypeText},
			},
			contentTypes: entity.ContentTypeDefs{{MachineName: "article", Label: "Article"}},
			valuesByNode: map[int]map[int]any{
				1: {body: "<p>Call me <em>Ishmael</em></p>", note: "<b>bold</b>"},
			},
		},
		rich: map[int]bool{body: true},
	}
	s := &CacheService{
		viewService: fakeListViewService{
			nodes: entity.Nodes{{ID: 1, Type: "article", Title: "Moby", Slug: "article/moby"}, {ID: 2, Type: "article", Title: "Pequod", Slug: "article/pequod"}},
			fields: entity.ViewFieldDefs{
				{ID: 1, BuiltinField: entity.ViewFieldBuiltinTitle, LinkTo: entity.ViewFieldLinkContent, SortOrder: 1},
				{ID: 2, FieldStorageID: &body, SortOrder: 2},
				{ID: 3, FieldStorageID: &note, SortOrder: 3},
			},
		},
		templateService: NewTemplateService(false),
		contentTypeRepo: types,
		nodeRepo:        fakeListAltTitles{titles: map[int]string{1: "The Whale"}},
		logger:          slog.New(slog.DiscardHandler),
	}
	html, err := s.renderContentViewBlock(t.Context(), publishedOf(t, s, entity.ViewDisplayDef{ID: 7, DisplayStyle: entity.ViewDisplayStyleList}), viewBlockPart{page: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(string(html)), " ")
	for _, want := range []string{
		`id="view-list-container-block-7"`,
		`<a href="/article/moby" class="item-field-link"> <span class="item-field-value">The Whale</span>`,
		`<a href="/article/pequod" class="item-field-link"> <span class="item-field-value">Pequod</span>`,
		`<p>Call me <em>Ishmael</em></p>`,
		`&lt;b&gt;bold&lt;/b&gt;`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the block has no %s:\n%s", want, out)
		}
	}
}

// fakePagedViewService - fakeListViewService with total results, the
// display's own settings as its published ones and its filters.
type fakePagedViewService struct {
	fakeListViewService
	total   int
	filters entity.ViewFilterDefs
}

func (f fakePagedViewService) RunConfig(context.Context, views.Config, views.RunContext) (entity.Nodes, int, error) {
	return f.nodes, f.total, nil
}

func (f fakePagedViewService) PublishedConfig(_ context.Context, d entity.ViewDisplayDef) (views.Config, error) {
	return views.Config{View: d, Fields: f.fields, Filters: f.filters}, nil
}

// TestCacheViewBlockPager - a View block whose display pages has a
// compact pager (‹ the numbers ›) whose links load its pages in place
// (/api/view-block/{placement}, data-ajax-pagination); a page's body is
// the list and pager without the container. A display that doesn't page,
// or one with contextual filters, has no pages to serve.
func TestCacheViewBlockPager(t *testing.T) {
	zzChdirToProjectRoot(t)
	fake := fakePagedViewService{
		fakeListViewService: fakeListViewService{
			nodes:  entity.Nodes{{ID: 1, Type: "article", Title: "Moby", Slug: "article/moby"}},
			fields: entity.ViewFieldDefs{{ID: 1, BuiltinField: entity.ViewFieldBuiltinTitle, LinkTo: entity.ViewFieldLinkContent, SortOrder: 1}},
		},
		total: 12,
	}
	s := &CacheService{
		viewService:     fake,
		templateService: NewTemplateService(false),
		contentTypeRepo: fakeListContentTypes{fakeContentTypeRepoForBlocks: &fakeContentTypeRepoForBlocks{contentTypes: entity.ContentTypeDefs{{MachineName: "article", Label: "Article"}}}},
		nodeRepo:        fakeListAltTitles{},
		logger:          slog.New(slog.DiscardHandler),
	}
	paged := entity.ViewDisplayDef{ID: 7, DisplayStyle: entity.ViewDisplayStyleList, ShowPagination: true, PageSize: 5}
	html, err := s.renderContentViewBlock(t.Context(), publishedOf(t, s, paged), viewBlockPart{placementID: 9, page: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	for _, want := range []string{`id="view-list-container-block-7" data-ajax-pagination="1"`, `pagination-compact`, `href="/api/view-block/9?page=2"`, `href="/api/view-block/9?page=3"`} {
		if !strings.Contains(out, want) {
			t.Errorf("the paged block has no %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "pager-last") || strings.Contains(out, "pager-first") {
		t.Error("the block's pager has the first and last links")
	}

	page, ok, err := s.ViewBlockPage(t.Context(), paged, 9, 2, nil)
	if err != nil || !ok {
		t.Fatalf("page 2: %v %v", ok, err)
	}
	if strings.Contains(string(page), "view-list-container-block") || !strings.Contains(string(page), `<li class="pager-current">2</li>`) || !strings.Contains(string(page), "Moby") {
		t.Errorf("page 2's body:\n%s", page)
	}

	unpaged := paged
	unpaged.ShowPagination = false
	if html, _ := s.renderContentViewBlock(t.Context(), publishedOf(t, s, unpaged), viewBlockPart{placementID: 9, page: 1}, nil); strings.Contains(string(html), "pagination") || strings.Contains(string(html), "data-ajax-pagination") {
		t.Errorf("a block that doesn't page has a pager:\n%s", html)
	}
	if _, ok, err := s.ViewBlockPage(t.Context(), unpaged, 9, 2, nil); ok || err != nil {
		t.Errorf("a page of a block that doesn't page: %v %v", ok, err)
	}
	fake.filters = entity.ViewFilterDefs{{Contextual: true}}
	s.viewService = fake
	if _, ok, err := s.ViewBlockPage(t.Context(), paged, 9, 2, nil); ok || err != nil {
		t.Errorf("a page of a contextual block: %v %v", ok, err)
	}
}
