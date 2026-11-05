package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeSearchConfig struct {
	types    entity.SearchTypeDefs
	settings entity.SearchSettings
	saved    []entity.SearchTypeDef
	loads    int
}

func (f *fakeSearchConfig) SearchTypes(context.Context) (entity.SearchTypeDefs, error) {
	f.loads++
	return f.types, nil
}
func (f *fakeSearchConfig) SaveSearchType(_ context.Context, t entity.SearchTypeDef) error {
	f.saved = append(f.saved, t)
	return nil
}
func (f *fakeSearchConfig) UpdateSearchTypeWeight(context.Context, string, int) error { return nil }
func (f *fakeSearchConfig) SearchSettings(context.Context) (*entity.SearchSettings, error) {
	s := f.settings
	return &s, nil
}
func (f *fakeSearchConfig) SaveSearchSettings(context.Context, entity.SearchSettings) error {
	return nil
}

type fakeSearchRepo struct {
	gotTypes entity.SearchTypeDefs
	gotSet   entity.SearchSettings
}

func (f *fakeSearchRepo) Snippets(_ context.Context, _ string, results []entity.SearchResult, _ entity.SearchTypeDefs) (map[int]entity.SearchSnippet, error) {
	out := map[int]entity.SearchSnippet{}
	for _, r := range results {
		out[r.ID] = entity.SearchSnippet{Parts: []entity.SearchSnippetPart{{Text: r.Title, Match: true}}}
	}
	return out, nil
}
func (f *fakeSearchRepo) Search(_ context.Context, _ string, _ int, types entity.SearchTypeDefs, s entity.SearchSettings) ([]entity.SearchResult, error) {
	f.gotTypes, f.gotSet = types, s
	return nil, nil
}

func TestSearchServiceConfig(t *testing.T) {
	cfg := &fakeSearchConfig{
		types:    entity.SearchTypeDefs{{NodeType: "article", Enabled: true, Weight: 10, Limit: 15}, {NodeType: "forum", Enabled: true, Weight: 100, Limit: 5}},
		settings: entity.SearchSettings{PlaceholderImage: "/x.png", CategoriesMenu: "search"},
	}
	repo := &fakeSearchRepo{}
	menus := func() map[string][]MenuTreeItem {
		return map[string][]MenuTreeItem{
			"search": {{Title: "Articles", URL: "/alpha-articles"}, {Title: "Broken", Broken: true}, {Title: "Forum", URL: "/forum"}},
			"main":   {{Title: "Home", URL: "/"}},
		}
	}
	s := NewSearchService(repo, cfg, menus, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	// The empty search shows the configured menu's links.
	want := []entity.SearchCategory{{Label: "Articles", URL: "/alpha-articles"}, {Label: "Forum", URL: "/forum"}}
	if got := s.GetCategories(ctx); !reflect.DeepEqual(got, want) {
		t.Errorf("categories %v, want %v", got, want)
	}
	// The search gets the configuration; it's cached until a write.
	if _, err := s.Search(ctx, "bat", 50); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search(ctx, "bat", 50); err != nil {
		t.Fatal(err)
	}
	if len(repo.gotTypes) != 2 || repo.gotSet.PlaceholderImage != "/x.png" || cfg.loads != 1 {
		t.Errorf("types %v settings %v loads %d", repo.gotTypes, repo.gotSet, cfg.loads)
	}
	// The results page raises the types' limits to its own, keeping 0 off.
	cfg.types = append(cfg.types, entity.SearchTypeDef{NodeType: "page", Enabled: true, Weight: 200})
	s.reset()
	if _, err := s.SearchAll(ctx, "bat", 200); err != nil {
		t.Fatal(err)
	}
	if l := []int{repo.gotTypes[0].Limit, repo.gotTypes[1].Limit, repo.gotTypes[2].Limit}; l[0] != 200 || l[1] != 200 || l[2] != 0 {
		t.Errorf("results page limits %v", l)
	}
	if cfg.types[0].Limit != 15 {
		t.Error("the configured limits must stay")
	}
	cfg.types = cfg.types[:2]
	s.reset()
	loads := cfg.loads
	// A new type goes last.
	if err := s.SaveType(ctx, entity.SearchTypeDef{NodeType: "news", Limit: 5}, true); err != nil {
		t.Fatal(err)
	}
	if cfg.saved[0].Weight != 110 {
		t.Errorf("new type weight %d, want 110", cfg.saved[0].Weight)
	}
	if _, err := s.Types(ctx); err != nil || cfg.loads != loads+2 {
		t.Errorf("cache not reset after write: loads %d", cfg.loads)
	}
}

type failingWeights struct {
	fakeSearchConfig
	set []string
}

func (f *failingWeights) UpdateSearchTypeWeight(_ context.Context, t string, _ int) error {
	if t == "broken" {
		return errors.New("db down")
	}
	f.set = append(f.set, t)
	return nil
}

type countingUOW struct{ calls int }

func (u *countingUOW) Do(ctx context.Context, fn func(context.Context) error) error {
	u.calls++
	return fn(ctx)
}

// TestReorderTypesInOneTransaction - a reorder runs in one transaction
// (its unit of work), a failure halfway returned with what was reordered.
func TestReorderTypesInOneTransaction(t *testing.T) {
	cfg := &failingWeights{}
	uow := &countingUOW{}
	s := NewSearchService(&fakeSearchRepo{}, cfg, nil, slog.New(slog.DiscardHandler))
	s.SetUnitOfWork(uow)
	err := s.ReorderTypes(t.Context(), []string{"events", "broken", "places"})
	if err == nil || !strings.Contains(err.Error(), "broken") || uow.calls != 1 {
		t.Fatalf("err %v, transactions %d", err, uow.calls)
	}
	if fmt.Sprint(cfg.set) != "[events]" {
		t.Errorf("weights set before the failure: %v", cfg.set)
	}
}

// TestSearchTypesByDefault - a type with no row is searched with the
// defaults (all its text fields), after the rows, by label; one removed (its row switched off)
// isn't; removing keeps a row's settings and adding back restores them; a
// reorder gives a default type its row first.
func TestSearchTypesByDefault(t *testing.T) {
	cfg := &fakeSearchConfig{types: entity.SearchTypeDefs{
		{NodeType: "article", Enabled: true, Weight: 10, Limit: 15, MatchFields: []string{"field_body"}},
		{NodeType: "page", Enabled: false, Limit: 7},
	}}
	s := NewSearchService(&fakeSearchRepo{}, cfg, nil, slog.New(slog.DiscardHandler))
	s.SetDefaultTypes(func(context.Context) (map[string]string, error) {
		return map[string]string{"article": "Article", "page": "Page", "news": "News", "event": "Event"}, nil
	})
	ctx := t.Context()
	types, err := s.Types(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, ty := range types {
		got = append(got, fmt.Sprintf("%s:%d:%d:%v", ty.NodeType, ty.Weight, ty.Limit, ty.AllFields))
	}
	if want := "[article:10:15:false event:20:5:true news:30:5:true]"; fmt.Sprint(got) != want {
		t.Errorf("searched %v, want %s", got, want)
	}

	if err := s.RemoveType(ctx, "article"); err != nil {
		t.Fatal(err)
	}
	if r := cfg.saved[len(cfg.saved)-1]; r.NodeType != "article" || r.Enabled || r.Limit != 15 || len(r.MatchFields) != 1 {
		t.Errorf("removed article saved as %+v, want switched off with its settings", r)
	}
	if err := s.RemoveType(ctx, "news"); err != nil {
		t.Fatal(err)
	}
	if r := cfg.saved[len(cfg.saved)-1]; r.NodeType != "news" || r.Enabled || r.Limit != entity.DefaultSearchLimit || !r.AllFields {
		t.Errorf("removed news saved as %+v", r)
	}

	if err := s.SaveType(ctx, entity.SearchTypeDef{NodeType: "page", Limit: entity.DefaultSearchLimit}, true); err != nil {
		t.Fatal(err)
	}
	if r := cfg.saved[len(cfg.saved)-1]; !r.Enabled || r.Limit != 7 || r.Weight != 40 {
		t.Errorf("page added back saved as %+v, want enabled, limit 7, weight 40", r)
	}

	cfg.saved = nil
	if err := s.ReorderTypes(ctx, []string{"event", "article"}); err != nil {
		t.Fatal(err)
	}
	if len(cfg.saved) != 1 || cfg.saved[0].NodeType != "event" || !cfg.saved[0].Enabled {
		t.Errorf("rows made for the reorder: %+v, want event's", cfg.saved)
	}
}

// TestWithSnippets - the page's results get their snippets, in order;
// none to show - no query to the repository.
func TestWithSnippets(t *testing.T) {
	s := NewSearchService(&fakeSearchRepo{}, &fakeSearchConfig{}, nil, slog.New(slog.DiscardHandler))
	results := []entity.SearchResult{{ID: 2, Title: "Two"}, {ID: 1, Title: "One"}}
	got, err := s.WithSnippets(t.Context(), "one", results)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Snippet.Parts[0].Text != "Two" || got[1].Snippet.Parts[0].Text != "One" || results[0].Snippet.Parts != nil {
		t.Errorf("with snippets %+v (the given ones %+v)", got, results)
	}
	if got, err := s.WithSnippets(t.Context(), "one", nil); err != nil || len(got) != 0 {
		t.Errorf("none: %v %v", got, err)
	}
}
