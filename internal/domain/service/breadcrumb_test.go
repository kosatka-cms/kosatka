package service

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeBreadcrumbRules struct {
	entity.BreadcrumbRuleRepository
	rules entity.BreadcrumbRuleDefs
}

func (f fakeBreadcrumbRules) BreadcrumbRules(context.Context) (entity.BreadcrumbRuleDefs, error) {
	return f.rules, nil
}

type fakeBreadcrumbTerms map[int]entity.TaxonomyTerms

func (f fakeBreadcrumbTerms) TermsByVocabularyID(_ context.Context, vid int) (entity.TaxonomyTerms, error) {
	return f[vid], nil
}

func testMenuTrees() map[string][]MenuTreeItem {
	return map[string][]MenuTreeItem{
		"footer": {{ID: 90, Title: "Alpha (footer)", URL: "/alpha-articles"}},
		"main": {
			{ID: 30, Title: "Forum", URL: "/forum", Children: []MenuTreeItem{{ID: 31, Title: "Who will win", URL: "/forum/kto"}}},
			{ID: 1, Title: "News", URL: "/", Children: []MenuTreeItem{{ID: 2, Title: "Video news", URL: "/novosti/novosti-video"}}},
			{ID: 10, Title: "Alpha section", URL: "/alpha-articles", Children: []MenuTreeItem{
				{ID: 11, Title: "Main articles", URL: "/alpha-articles"},
				{ID: 12, Title: "Beta articles", URL: "/beta-articles"},
			}},
			{ID: 20, Title: "Media", URL: "/videos", Children: []MenuTreeItem{{ID: 21, Title: "Video", URL: "/videos"}}},
		},
	}
}

func items(pairs ...string) []entity.BreadcrumbItem {
	var out []entity.BreadcrumbItem
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, entity.BreadcrumbItem{URL: pairs[i], Label: pairs[i+1]})
	}
	return out
}

func TestBreadcrumbTrailForPath(t *testing.T) {
	s := NewBreadcrumbService(nil, nil, testMenuTrees, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for path, want := range map[string][]entity.BreadcrumbItem{
		// The news menu's home link "/" collapses into the home crumb.
		"/novosti/novosti-video": items("/", "Home", "", "Video news"),
		// A section and its first child share a URL - the section stays,
		// the bigger menu wins over the footer at the same depth.
		"/alpha-articles": items("/", "Home", "", "Alpha section"),
		"/beta-articles":  items("/", "Home", "/alpha-articles", "Alpha section", "", "Beta articles"),
	} {
		if got := s.TrailForPath(context.Background(), path); !reflect.DeepEqual(got, want) {
			t.Errorf("TrailForPath(%s) = %v, want %v", path, got, want)
		}
	}
	if got := s.TrailForPath(context.Background(), "/tags/foo"); got != nil {
		t.Errorf("path in no menu: %v, want nil", got)
	}
	if got := s.TrailOrTitle(context.Background(), "/tags/foo", "Foo Bar"); !reflect.DeepEqual(got, items("/", "Home", "", "Foo Bar")) {
		t.Errorf("TrailOrTitle = %v", got)
	}
}

func TestBreadcrumbTrailForNode(t *testing.T) {
	sectionField := 31
	rules := fakeBreadcrumbRules{rules: entity.BreadcrumbRuleDefs{
		{ContentType: "articles", FieldStorageID: &sectionField, TermIDs: "2852", MenuLinkID: 12, Weight: 0},  // Beta
		{ContentType: "articles", FieldStorageID: &sectionField, TermIDs: "2856", MenuLinkID: 11, Weight: 10}, // Alpha
		{ContentType: "videos", MenuLinkID: 21},
	}}
	// Term 2851 is a child of Alpha (2856); 2852 too.
	terms := fakeBreadcrumbTerms{31: {{ID: 2856}, {ID: 2851, ParentID: 2856}, {ID: 2852, ParentID: 2856}, {ID: 2857}}}
	s := NewBreadcrumbService(nil, rules, testMenuTrees, terms, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	article := func(termIDs ...int) entity.DynamicContent {
		return entity.DynamicContent{
			Node:   entity.Node{Type: "articles"},
			Fields: entity.FieldInstanceDefs{{ID: 5, FieldStorageID: sectionField, FieldType: entity.FieldTypeTaxonomy, Config: map[string]any{"vocabulary_id": float64(31), "multiple": true}}},
			Values: map[int]any{5: termIDs},
		}
	}
	ctx := context.Background()
	if got, want := s.TrailForNode(ctx, article(2851), "Title"), items("/", "Home", "/alpha-articles", "Alpha section", "", "Title"); !reflect.DeepEqual(got, want) {
		t.Errorf("descendant of Alpha: %v, want %v", got, want)
	}
	if got, want := s.TrailForNode(ctx, article(2852), "Beta Title"), items("/", "Home", "/alpha-articles", "Alpha section", "/beta-articles", "Beta articles", "", "Beta Title"); !reflect.DeepEqual(got, want) {
		t.Errorf("lighter rule wins: %v, want %v", got, want)
	}
	if got, want := s.TrailForNode(ctx, article(2857), "Foo Bar"), items("/", "Home", "", "Foo Bar"); !reflect.DeepEqual(got, want) {
		t.Errorf("no rule: %v, want %v", got, want)
	}
	video := entity.DynamicContent{Node: entity.Node{Type: "videos"}}
	if got, want := s.TrailForNode(ctx, video, "Video piece"), items("/", "Home", "/videos", "Media", "", "Video piece"); !reflect.DeepEqual(got, want) {
		t.Errorf("rule without condition: %v, want %v", got, want)
	}
}

func TestBreadcrumbJSONLD(t *testing.T) {
	s := NewBreadcrumbService(func() string { return "https://example.org/" }, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	got := string(s.JSONLD(items("/", "Home", "/videos", "Media", "", "Video piece <x>")))
	for _, want := range []string{`"@type":"BreadcrumbList"`, `"item":"https://example.org/videos"`, `"position":3`, `\u003cx\u003e`} {
		if !strings.Contains(got, want) {
			t.Errorf("JSON-LD %s lacks %s", got, want)
		}
	}
	if strings.Count(got, `"item"`) != 2 {
		t.Errorf("the current page must have no item: %s", got)
	}
	if NewBreadcrumbService(nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))).JSONLD(items("/", "Home")) != "" {
		t.Error("no base URL - no JSON-LD")
	}
}

type fakeBreadcrumbFields map[int][]int // node → section terms

func (f fakeBreadcrumbFields) FieldStorageByID(_ context.Context, id int) (*entity.FieldStorageDef, error) {
	return &entity.FieldStorageDef{ID: id, Config: map[string]any{"vocabulary_id": float64(31)}}, nil
}

func (f fakeBreadcrumbFields) FieldTaxonomyValuesForNodes(context.Context, int, []int) (map[int]int, error) {
	return map[int]int{}, nil
}

func (f fakeBreadcrumbFields) FieldTaxonomyMultiValuesForNodes(_ context.Context, _ int, nodeIDs []int) (map[int][]int, error) {
	out := map[int][]int{}
	for _, id := range nodeIDs {
		out[id] = f[id]
	}
	return out, nil
}

func TestBreadcrumbChildTrail(t *testing.T) {
	s := NewBreadcrumbService(nil, nil, testMenuTrees, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	forum := entity.BreadcrumbItem{URL: "/forum", Label: "Forum (code)"}
	for name, tc := range map[string]struct {
		parents []entity.BreadcrumbItem
		want    []entity.BreadcrumbItem
	}{
		// The deepest parent in a menu brings its chain (menu labels win).
		"section in menu":     {[]entity.BreadcrumbItem{forum, {URL: "/forum/kto", Label: "Who (code)"}}, items("/", "Home", "/forum", "Forum", "/forum/kto", "Who will win", "", "Topic")},
		"section not in menu": {[]entity.BreadcrumbItem{forum, {URL: "/forum/other", Label: "Other"}}, items("/", "Home", "/forum", "Forum", "/forum/other", "Other", "", "Topic")},
		"no parent in menu":   {[]entity.BreadcrumbItem{{URL: "/ishmael", Label: "ishmael"}}, items("/", "Home", "/ishmael", "ishmael", "", "Topic")},
		"no parents":          {nil, items("/", "Home", "", "Topic")},
	} {
		if got := s.ChildTrail(context.Background(), tc.parents, "Topic"); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}
}

func TestBreadcrumbNodeSubpageTrail(t *testing.T) {
	sectionField := 31
	rules := fakeBreadcrumbRules{rules: entity.BreadcrumbRuleDefs{{ContentType: "articles", FieldStorageID: &sectionField, TermIDs: "2856", MenuLinkID: 11}}}
	terms := fakeBreadcrumbTerms{31: {{ID: 2856}, {ID: 2851, ParentID: 2856}}}
	s := NewBreadcrumbService(nil, rules, testMenuTrees, terms, fakeBreadcrumbFields{7: {2851}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	node := entity.Node{ID: 7, Type: "articles", Title: "Title", Slug: "articles/foo"}
	got := s.NodeSubpageTrail(context.Background(), node, "Gallery", entity.BreadcrumbItem{URL: "/node/7/galleries", Label: "Galleries"})
	want := items("/", "Home", "/alpha-articles", "Alpha section", "/articles/foo", "Title", "/node/7/galleries", "Galleries", "", "Gallery")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NodeSubpageTrail = %v, want %v", got, want)
	}
}
