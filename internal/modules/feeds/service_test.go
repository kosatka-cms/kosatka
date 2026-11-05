package feeds

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
)

type fakeFeedRepo struct {
	feeds  map[int]Feed
	nextID int
	// listed - how many times Feeds was asked.
	listed int
}

func (f *fakeFeedRepo) Feeds(context.Context) ([]Feed, error) {
	f.listed++
	var out []Feed
	for _, x := range f.feeds {
		out = append(out, x)
	}
	return out, nil
}

func (f *fakeFeedRepo) FeedByID(_ context.Context, id int) (*Feed, error) {
	if x, ok := f.feeds[id]; ok {
		return &x, nil
	}
	return nil, modulekit.ErrEntityNotFound
}

func (f *fakeFeedRepo) FeedByPath(_ context.Context, path string) (*Feed, error) {
	for _, x := range f.feeds {
		if x.Path == path {
			return &x, nil
		}
	}
	return nil, modulekit.ErrEntityNotFound
}

func (f *fakeFeedRepo) CreateFeed(_ context.Context, x Feed) (int, error) {
	f.nextID++
	x.ID = f.nextID
	if f.feeds == nil {
		f.feeds = map[int]Feed{}
	}
	f.feeds[x.ID] = x
	return x.ID, nil
}

func (f *fakeFeedRepo) UpdateFeed(_ context.Context, x Feed) error {
	f.feeds[x.ID] = x
	return nil
}

func (f *fakeFeedRepo) DeleteFeed(_ context.Context, id int) error {
	delete(f.feeds, id)
	return nil
}

type fakeFeedContentTypes struct{}

func (fakeFeedContentTypes) ContentTypes(context.Context) (modulekit.ContentTypeDefs, error) {
	return modulekit.ContentTypeDefs{{ID: 1, MachineName: "news"}, {ID: 2, MachineName: "article"}}, nil
}

func (fakeFeedContentTypes) FieldsByContentTypeID(_ context.Context, id int) (modulekit.FieldInstanceDefs, error) {
	if id == 1 {
		return modulekit.FieldInstanceDefs{
			{MachineName: "content", FieldType: modulekit.FieldTypeText},
			{MachineName: "cover", FieldType: modulekit.FieldTypeImage},
			{MachineName: "rubric", FieldType: modulekit.FieldTypeTaxonomy},
		}, nil
	}
	return nil, nil
}

// pathOwners - modulekit.PathOwner with the addresses others answer.
func pathOwners(owners map[string]string) func(context.Context, string) (string, error) {
	return func(_ context.Context, p string) (string, error) { return owners[p], nil }
}

type fakeFeedNodes struct {
	modulekit.NodesRepository
	got modulekit.FeedNodeFilter
}

func (f *fakeFeedNodes) ListForFeed(_ context.Context, filter modulekit.FeedNodeFilter) (modulekit.Nodes, error) {
	f.got = filter
	return modulekit.Nodes{{ID: 1}}, nil
}

func TestFeedSaveValidation(t *testing.T) {
	ctx := context.Background()
	repo := &fakeFeedRepo{}
	s := NewFeedService(repo, &fakeFeedNodes{}, fakeFeedContentTypes{}, pathOwners(map[string]string{"news": "views", "rss.xml": "routes", "about": "content"}), slog.New(slog.DiscardHandler))
	ok := Feed{Label: "Dzen", Path: "/Feeds/Dzen.xml/", ContentTypes: []string{"news", "missing"}, Status: 1,
		FieldMaps: FeedFieldMaps{{ContentType: "news", ImageField: "cover", CategoryField: "rubric"}}}

	cases := []struct {
		name string
		mod  func(f *Feed)
		want error
	}{
		{"no label", func(f *Feed) { f.Label = " " }, ErrFeedLabel},
		{"bad path", func(f *Feed) { f.Path = "feeds/../x" }, ErrFeedPath},
		{"path of a view", func(f *Feed) { f.Path = "news" }, ErrFeedPathTaken},
		{"path of the engine's", func(f *Feed) { f.Path = "rss.xml" }, ErrFeedPathTaken},
		{"path of content", func(f *Feed) { f.Path = "about" }, ErrFeedPathTaken},
		{"no known type", func(f *Feed) { f.ContentTypes = []string{"missing"} }, ErrFeedContentTypes},
		{"image field of wrong type", func(f *Feed) {
			f.FieldMaps = FeedFieldMaps{{ContentType: "news", ImageField: "rubric"}}
		}, ErrFeedFieldMap},
	}
	for _, tc := range cases {
		f := ok
		tc.mod(&f)
		if _, err := s.Save(ctx, f); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
		}
	}

	id, err := s.Save(ctx, ok)
	if err != nil {
		t.Fatal(err)
	}
	saved := repo.feeds[id]
	if saved.Path != "feeds/dzen.xml" || len(saved.ContentTypes) != 1 || saved.ItemLimit != feedDefaultItemLimit || saved.Format != FeedFormatRSS {
		t.Fatalf("normalized: %+v", saved)
	}
	if m := saved.FieldMaps.For("news"); m.ImageField != "cover" || m.CategoryField != "rubric" {
		t.Fatalf("field map: %+v", m)
	}

	dup := ok
	dup.Label = "Another"
	if _, err := s.Save(ctx, dup); !errors.Is(err, ErrFeedPathTaken) {
		t.Fatalf("second feed at the same path: %v", err)
	}
	if s.FeedByPath(ctx, "/feeds/dzen.xml") == nil {
		t.Fatal("enabled feed not found by path")
	}
}

func TestFeedNodesFilter(t *testing.T) {
	nodes := &fakeFeedNodes{}
	s := NewFeedService(&fakeFeedRepo{}, nodes, fakeFeedContentTypes{}, nil, slog.New(slog.DiscardHandler))
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

	f := Feed{Format: FeedFormatDzen, ContentTypes: []string{"news"}, TermIDs: []int{5}, ItemLimit: 20}
	if _, err := s.Nodes(context.Background(), f, now); err != nil {
		t.Fatal(err)
	}
	if !nodes.got.Since.Equal(now.Add(-7*24*time.Hour)) || nodes.got.Limit != 20 || len(nodes.got.TermIDs) != 1 || nodes.got.Types[0] != "news" {
		t.Fatalf("dzen default window: %+v", nodes.got)
	}

	f.Format, f.Days = FeedFormatRSS, 0
	_, _ = s.Nodes(context.Background(), f, now)
	if !nodes.got.Since.IsZero() {
		t.Fatalf("rss has no window: %+v", nodes.got)
	}

	f.Days = 3
	_, _ = s.Nodes(context.Background(), f, now)
	if !nodes.got.Since.Equal(now.Add(-3 * 24 * time.Hour)) {
		t.Fatalf("explicit days: %+v", nodes.got)
	}

	// A disabled module's type is left out; with no type left, nothing.
	s.typeEnabled = func(t string) bool { return t != "poll" }
	f.ContentTypes = []string{"poll", "news"}
	_, _ = s.Nodes(context.Background(), f, now)
	if !slices.Equal(nodes.got.Types, []string{"news"}) {
		t.Fatalf("a disabled module's type: %+v", nodes.got.Types)
	}
	nodes.got = modulekit.FeedNodeFilter{}
	f.ContentTypes = []string{"poll"}
	if got, err := s.Nodes(context.Background(), f, now); err != nil || got != nil || nodes.got.Types != nil {
		t.Fatalf("only a disabled module's type: %v %v %+v", got, err, nodes.got)
	}
}

// TestFeedByPathKept - the enabled feeds' addresses are read once: asking
// again doesn't list the feeds; a save or a delete drops them; a disabled
// feed answers no address.
func TestFeedByPathKept(t *testing.T) {
	ctx := context.Background()
	repo := &fakeFeedRepo{feeds: map[int]Feed{
		1: {ID: 1, Path: "rss.xml", Status: 1},
		2: {ID: 2, Path: "off.xml", Status: 0},
	}, nextID: 2}
	s := NewFeedService(repo, &fakeFeedNodes{}, fakeFeedContentTypes{}, nil, slog.New(slog.DiscardHandler))
	if f := s.FeedByPath(ctx, "/rss.xml"); f == nil || f.ID != 1 {
		t.Fatalf("rss.xml: %+v", f)
	}
	if f := s.FeedByPath(ctx, "off.xml"); f != nil {
		t.Errorf("a disabled feed answers: %+v", f)
	}
	if s.FeedByPath(ctx, "none") != nil || repo.listed != 1 {
		t.Errorf("listed %d times, want once", repo.listed)
	}
	if err := s.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if f := s.FeedByPath(ctx, "rss.xml"); f != nil || repo.listed != 2 {
		t.Errorf("after a delete: %+v, listed %d", f, repo.listed)
	}
}
