package service

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeTagCloudRepo struct {
	entity.TaxonomyRepository
	counts map[int]map[int]int // vocabulary → term → materials
	terms  map[int]entity.TaxonomyTerm
	asked  []int
}

func (f *fakeTagCloudRepo) TermNodeCounts(_ context.Context, vid int) (map[int]int, error) {
	f.asked = append(f.asked, vid)
	return f.counts[vid], nil
}

func (f *fakeTagCloudRepo) TermsByIDs(_ context.Context, ids []int) (map[int]entity.TaxonomyTerm, error) {
	out := map[int]entity.TaxonomyTerm{}
	for _, id := range ids {
		if t, ok := f.terms[id]; ok {
			out[id] = t
		}
	}
	return out, nil
}

func (f *fakeTagCloudRepo) Vocabularies(context.Context) (entity.TaxonomyVocabularies, error) {
	return entity.TaxonomyVocabularies{{ID: 2, Name: "Tags"}}, nil
}

func tagCloudFixture() *fakeTagCloudRepo {
	public := entity.TaxonomyVocabulary{ID: 2, PublicTermPages: true}
	return &fakeTagCloudRepo{
		counts: map[int]map[int]int{
			0: {1: 40, 2: 1, 3: 7, 4: 7, 5: 3},
			2: {1: 40, 3: 7},
		},
		terms: map[int]entity.TaxonomyTerm{
			1: {ID: 1, Name: "zebra", Slug: "tags/zebra", Vocabulary: public},
			2: {ID: 2, Name: "rare", Slug: "tags/rare", Vocabulary: public},
			3: {ID: 3, Name: "Apple", Slug: "tags/apple", Vocabulary: public},
			4: {ID: 4, Name: "hidden", Slug: "kinds/hidden"},
			5: {ID: 5, Name: "mid", Slug: "tags/mid", Vocabulary: public},
		},
	}
}

func tagCloudNames(terms []entity.TagCloudTerm) string {
	var out []string
	for _, t := range terms {
		out = append(out, t.Term.Name)
	}
	return strings.Join(out, ",")
}

// TestTagCloud - the most used terms (equal counts - the lower ID), by
// name; a term of a vocabulary without public pages isn't linked.
func TestTagCloud(t *testing.T) {
	repo := tagCloudFixture()
	svc := NewTaxonomyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := svc.TagCloud(context.Background(), 0, 3, 6)
	if err != nil {
		t.Fatal(err)
	}
	if names := tagCloudNames(got); names != "Apple,hidden,zebra" {
		t.Errorf("terms %q, want the three most used by name", names)
	}
	for _, term := range got {
		if term.Linked != (term.Term.ID != 4) {
			t.Errorf("%s: linked %v", term.Term.Name, term.Linked)
		}
	}
	if got[2].Count != 40 || got[2].Level != 6 || got[0].Level != 1 {
		t.Errorf("zebra %d at level %d, Apple at %d: want 40, 6, 1", got[2].Count, got[2].Level, got[0].Level)
	}
}

func TestTagCloudLevels(t *testing.T) {
	terms := []entity.TagCloudTerm{{Count: 1}, {Count: 10}, {Count: 100}}
	tagCloudLevels(terms, 5)
	if terms[0].Level != 1 || terms[1].Level != 3 || terms[2].Level != 5 {
		t.Errorf("levels %d %d %d, want 1 3 5 (logarithmic)", terms[0].Level, terms[1].Level, terms[2].Level)
	}
	same := []entity.TagCloudTerm{{Count: 4}, {Count: 4}}
	tagCloudLevels(same, 6)
	if same[0].Level != 3 || same[1].Level != 3 {
		t.Errorf("all on one count: %d %d, want the middle level", same[0].Level, same[1].Level)
	}
	one := []entity.TagCloudTerm{{Count: 2}, {Count: 9}}
	tagCloudLevels(one, 1)
	if one[0].Level != 1 || one[1].Level != 1 {
		t.Errorf("one level: %d %d", one[0].Level, one[1].Level)
	}
}

// TestTagCloudBlockData - the block's data follows its placement's
// settings; one no longer valid (a vocabulary gone, a number out of
// range) falls back to its default.
func TestTagCloudBlockData(t *testing.T) {
	repo := tagCloudFixture()
	RegisterTagCloudBlock(NewTaxonomyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil))))
	load, ok := PluginBlockDataLoader(entity.PluginBlockTagCloud)
	if !ok {
		t.Fatal("the tag cloud has no data loader")
	}
	data, err := load(context.Background(), entity.BlockPlacementDef{PluginConfig: map[string]string{
		entity.TagCloudVocabularyKey: "2", entity.TagCloudCountKey: "1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if names := tagCloudNames(data.([]entity.TagCloudTerm)); names != "zebra" {
		t.Errorf("vocabulary 2, one term: %q", names)
	}
	data, _ = load(context.Background(), entity.BlockPlacementDef{PluginConfig: map[string]string{
		entity.TagCloudVocabularyKey: "9", entity.TagCloudCountKey: "1000",
	}})
	if n := len(data.([]entity.TagCloudTerm)); n != 5 || repo.asked[len(repo.asked)-1] != 0 {
		t.Errorf("invalid settings: %d terms of vocabulary %d, want every vocabulary's (defaults)", n, repo.asked[len(repo.asked)-1])
	}
	settings, err := PluginBlockSettings(context.Background(), entity.PluginBlockTagCloud)
	if err != nil || len(settings) != 3 || len(settings[0].Options) != 2 {
		t.Errorf("settings %+v %v: want vocabulary (every one, Tags), count, levels", settings, err)
	}
}

// TestResolvePluginBlockData - the cache loads a plugin placement's data
// into new slices, the given ones (the previous cache's) untouched.
func TestResolvePluginBlockData(t *testing.T) {
	id := entity.PluginBlockID("test_data_block")
	SetPluginBlockData(id, func(_ context.Context, p entity.BlockPlacementDef) (any, error) {
		return "data of " + p.PluginSetting("x"), nil
	})
	other := entity.PluginBlockLogo
	given := map[entity.Region][]entity.BlockWithPlacement{"sidebar": {
		{Placement: entity.BlockPlacementDef{ID: 1, PlacementType: entity.BlockPlacementPlugin, PluginID: &id, PluginConfig: map[string]string{"x": "one"}}},
		{Placement: entity.BlockPlacementDef{ID: 2, PlacementType: entity.BlockPlacementPlugin, PluginID: &other}},
	}}
	s := &CacheService{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	got := s.resolvePluginBlockData(context.Background(), given)
	if got["sidebar"][0].PluginData != "data of one" || got["sidebar"][1].PluginData != nil {
		t.Errorf("data: %v, %v", got["sidebar"][0].PluginData, got["sidebar"][1].PluginData)
	}
	if given["sidebar"][0].PluginData != nil {
		t.Error("the given placements were changed")
	}
}

// TestTagCloudRender - the block lists its terms with their level
// classes, a linked one to its page; no terms - nothing.
func TestTagCloudRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)
	id := entity.PluginBlockTagCloud
	render := func(data any) string {
		t.Helper()
		bp := map[entity.Region][]entity.BlockWithPlacement{"sidebar": {{
			Placement:  entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &id, VisibilityScope: entity.BlockVisibilityAll, Title: "Tags", ShowTitle: true},
			PluginData: data,
		}}}
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "region_blocks", map[string]any{"Region": entity.Region("sidebar"), "Ctx": map[string]any{"blockPlacements": bp}}, zzBlockTemplateFiles...); err != nil {
			t.Fatalf("render error: %v", err)
		}
		return buf.String()
	}
	out := render([]entity.TagCloudTerm{
		{Term: entity.TaxonomyTerm{Name: "Go", Slug: "tags/go"}, Count: 12, Level: 6, Linked: true},
		{Term: entity.TaxonomyTerm{Name: "Kind", Slug: "kinds/kind"}, Count: 1, Level: 1},
	})
	for _, want := range []string{`<h2>Tags</h2>`, `class="tag-cloud-item tag-cloud-level-6"><a href="/tags/go" title="12 materials">Go</a>`, `class="tag-cloud-item tag-cloud-level-1"><span>Kind</span>`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in:\n%s", want, out)
		}
	}
	if out := render(nil); strings.Contains(out, "tag-cloud") {
		t.Errorf("no terms, yet: %s", out)
	}
}
