package interfaces

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

func TestWithPreloadImage(t *testing.T) {
	p := entity.PageSEO{Title: "Test"}
	got := withPreloadImage(p, imageurl.Picture{Src: "/uploads/hero.jpg", Srcset: "/a.webp 1x, /b.webp 2x"})
	if got.PreloadImage != "/uploads/hero.jpg" || got.PreloadImageSrcset != "/a.webp 1x, /b.webp 2x" {
		t.Fatalf("PreloadImage = %q, srcset %q", got.PreloadImage, got.PreloadImageSrcset)
	}
	got = withPreloadImage(p, imageurl.Picture{Src: "  "})
	if got.PreloadImage != "" {
		t.Fatalf("expected empty PreloadImage for blank URI, got %q", got.PreloadImage)
	}
}

type fakeSEORepo struct {
	tpl   entity.SEOTemplates
	rules entity.SEORuleDefs
}

func (f *fakeSEORepo) SEOTemplates(context.Context) (entity.SEOTemplates, error) { return f.tpl, nil }

func (f *fakeSEORepo) SaveSEOTemplate(context.Context, entity.SEOTemplate) error { return nil }

func (f *fakeSEORepo) SEORules(context.Context) (entity.SEORuleDefs, error) { return f.rules, nil }

func (f *fakeSEORepo) SEORuleByID(context.Context, int) (*entity.SEORuleDef, error) {
	return nil, nil
}

func (f *fakeSEORepo) CreateSEORule(context.Context, entity.SEORuleDef) (*entity.SEORuleDef, error) {
	return nil, nil
}

func (f *fakeSEORepo) UpdateSEORule(context.Context, entity.SEORuleDef) error { return nil }

func (f *fakeSEORepo) UpdateSEORuleWeight(context.Context, int, int) error { return nil }

func (f *fakeSEORepo) DeleteSEORule(context.Context, int) error { return nil }

type fakeSEOTaxonomy map[int]entity.TaxonomyTerm

func (f fakeSEOTaxonomy) TermsByVocabularyID(context.Context, int) (entity.TaxonomyTerms, error) {
	var out entity.TaxonomyTerms
	for _, t := range f {
		out = append(out, t)
	}
	return out, nil
}

func (f fakeSEOTaxonomy) TermsByIDs(_ context.Context, ids []int) (map[int]entity.TaxonomyTerm, error) {
	out := map[int]entity.TaxonomyTerm{}
	for _, id := range ids {
		out[id] = f[id]
	}
	return out, nil
}

func TestSEOMigrationTemplates(t *testing.T) {
	const year = `{ ([node:field:date_local:year|node:field:date_global:year])}`
	const dual = `{[node:field:title_alt] ([node:title])|[node:title]}`
	storyKind := 106
	repo := &fakeSEORepo{
		tpl: entity.SEOTemplates{
			"node__article": {TitleTemplate: "[node:title]{ ([node:field:title_alt])} - Article"},
			"node__video":   {TitleTemplate: dual + year},
			"node__show":    {TitleTemplate: dual + year},
			"node__product": {TitleTemplate: dual + year + "{ - [node:field:platforms]}"},
			"node__post":    {DescriptionTemplate: "[node:field:content_short]"},
		},
	}
	for i, tc := range []struct {
		id    int
		label string
	}{{1285, "book"}, {20, "video"}, {5008, "show"}, {1284, "game"}, {21, "music"}, {2926, "mobile game"}} {
		repo.rules = append(repo.rules, entity.SEORuleDef{ID: i + 1, ContentType: "story", FieldStorageID: &storyKind, TermIDs: strconv.Itoa(tc.id), TitleTemplate: "[node:title] - " + tc.label + " review", Weight: i})
	}
	taxonomy := fakeSEOTaxonomy{1: {ID: 1, Name: "PC"}, 2: {ID: 2, Name: "PS5"}, 1285: {ID: 1285}, 20: {ID: 20}, 5008: {ID: 5008}, 1284: {ID: 1284}, 21: {ID: 21}, 2926: {ID: 2926}}
	seo := service.NewSEOService(repo, repo, taxonomy, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	fields := entity.FieldInstanceDefs{
		{ID: 1, MachineName: "title_alt", FieldType: entity.FieldTypeText},
		{ID: 2, MachineName: "date_local", FieldType: entity.FieldTypeDate},
		{ID: 3, MachineName: "date_global", FieldType: entity.FieldTypeDate},
		{ID: 4, MachineName: "platforms", FieldType: entity.FieldTypeTaxonomy, FieldStorageID: 108, Config: map[string]any{"vocabulary_id": float64(9), "multiple": true}},
		{ID: 5, MachineName: "story_kind", FieldType: entity.FieldTypeTaxonomy, FieldStorageID: storyKind, Config: map[string]any{"vocabulary_id": float64(4)}},
		{ID: 6, MachineName: "content_short", FieldType: entity.FieldTypeText},
		{ID: 7, MachineName: "content", FieldType: entity.FieldTypeText},
	}
	node := func(typ, title string, values map[int]any) *entity.DynamicContent {
		return &entity.DynamicContent{Node: entity.Node{Type: entity.ContentType(typ), Title: title, Slug: typ + "/x"}, Fields: fields, Values: values}
	}
	d2008 := time.Date(2008, 5, 1, 0, 0, 0, 0, time.UTC)
	d2010 := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name, title, desc string
		dc                *entity.DynamicContent
	}{
		{"article", "Foo (Fu) - Article", "", node("article", "Foo", map[int]any{1: "Fu", 7: "<p>Article text</p>"})},
		{"article no alt", "Foo - Article", "", node("article", "Foo", map[int]any{})},
		{"video local year", "Barre (Bar) (2008)", "", node("video", "Bar", map[int]any{1: "Barre", 2: d2008, 3: d2010})},
		{"video global year", "Barre (Bar) (2010)", "", node("video", "Bar", map[int]any{1: "Barre", 3: d2010})},
		{"video no translation no year", "Baz", "", node("video", "Baz", map[int]any{})},
		// No special case for a translation equal to the title.
		{"video same titles", "Baz (Baz)", "", node("video", "Baz", map[int]any{1: "Baz"})},
		{"show", "Kwax (Qux) (2008)", "", node("show", "Qux", map[int]any{1: "Kwax", 2: d2008})},
		{"product", "Kuux (Quux) (2010) - PC, PS5", "", node("product", "Quux", map[int]any{1: "Kuux", 3: d2010, 4: []int{1, 2}})},
		{"product platforms only", "X - PS5", "", node("product", "X", map[int]any{4: []int{2}})},
		{"story book", "Alpha - book review", "", node("story", "Alpha", map[int]any{1: "Alfa", 5: 1285})},
		{"story show", "Beta - show review", "", node("story", "Beta", map[int]any{5: 5008})},
		{"story mobile", "App - mobile game review", "", node("story", "App", map[int]any{5: 2926})},
		{"post", "Note", "Note teaser", node("post", "Note", map[int]any{6: "<p>Note teaser</p>", 7: "<p>Text</p>"})},
		// The node's own title wins over every template.
		{"override", "Custom title", "", func() *entity.DynamicContent {
			dc := node("story", "Alpha", map[int]any{5: 1285})
			dc.Node.PageSEO.Title = "Custom title"
			return dc
		}()},
	}
	for _, tc := range cases {
		got := seo.Apply(context.Background(), *tc.dc, PageSEO(tc.dc))
		if got.Title != tc.title {
			t.Errorf("%s: title %q, want %q", tc.name, got.Title, tc.title)
		}
		if got.Description != tc.desc {
			t.Errorf("%s: description %q, want %q", tc.name, got.Description, tc.desc)
		}
	}
}

// TestSEOImageTemplate - og:image comes from the image template
// ([node:field:<image>] → the file's original, absolute): the type's,
// else the default one; an empty field - no og:image.
func TestSEOImageTemplate(t *testing.T) {
	repo := &fakeSEORepo{tpl: entity.SEOTemplates{
		entity.SEOTemplateDefaultScope: {ImageTemplate: "[node:field:cover_image]"},
		"node__page":                   {ImageTemplate: "[node:field:poster]"},
	}}
	seo := service.NewSEOService(repo, repo, fakeSEOTaxonomy{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	seo.SetFiles(func(_ context.Context, id int) (*entity.File, error) {
		return &entity.File{ID: id, URI: "public://img" + strconv.Itoa(id) + ".jpg"}, nil
	})
	fields := entity.FieldInstanceDefs{
		{ID: 1, MachineName: "cover_image", FieldType: entity.FieldTypeImage},
		{ID: 2, MachineName: "poster", FieldType: entity.FieldTypeImage},
	}
	domain := entity.NewSystemSettings().Domain
	for _, tc := range []struct {
		typ    string
		values map[int]any
		want   string
	}{
		{"article", map[int]any{1: 7}, domain + "/uploads/img7.jpg"},
		{"article", map[int]any{}, ""},
		{"page", map[int]any{1: 7, 2: 9}, domain + "/uploads/img9.jpg"},
	} {
		dc := entity.DynamicContent{Node: entity.Node{Type: entity.ContentType(tc.typ)}, Fields: fields, Values: tc.values}
		if got := seo.Apply(context.Background(), dc, entity.PageSEO{}).OGImage; got != tc.want {
			t.Errorf("%s %v: og:image %q, want %q", tc.typ, tc.values, got, tc.want)
		}
	}
}

// Term page SEO templates build the titles and descriptions of term pages;
// a term's own page title takes precedence.
func TestTermPageSEOTemplates(t *testing.T) {
	repo := &fakeSEORepo{tpl: entity.SEOTemplates{
		entity.SEOTemplateVocabularyScope(3):  {TitleTemplate: "[term:title] | [site:name]", DescriptionTemplate: "[term:title] - news, articles and other information on the topic. All materials on \"[term:title]\" at example.com."},
		entity.SEOTemplateVocabularyScope(16): {TitleTemplate: "[term:title] | [site:name]", DescriptionTemplate: "[term:title] - news, articles and other information on the topic"},
	}}
	site := func(context.Context) string { return "Example Site" }
	seo := service.NewSEOService(repo, repo, nil, site, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	tag := entity.TaxonomyTerm{Name: "Foo", Vocabulary: entity.TaxonomyVocabulary{ID: 3}}
	got := seo.ApplyTerm(ctx, tag, PageSEOForList("Foo", "/tags/foo"))
	if got.Title != "Foo | Example Site" || got.Description != "Foo - news, articles and other information on the topic. All materials on \"Foo\" at example.com." {
		t.Errorf("tag: %q / %q", got.Title, got.Description)
	}
	news := entity.TaxonomyTerm{Name: "Video", PageTitle: "Video news", Vocabulary: entity.TaxonomyVocabulary{ID: 16}}
	got = seo.ApplyTerm(ctx, news, PageSEOForList("Video news", "/news/video-news"))
	if got.Title != "Video news | Example Site" || got.Description != "Video news - news, articles and other information on the topic" {
		t.Errorf("news: %q / %q", got.Title, got.Description)
	}
	// No template for the vocabulary, none by default - the list SEO stays.
	other := entity.TaxonomyTerm{Name: "Alpha", Vocabulary: entity.TaxonomyVocabulary{ID: 31}}
	if got = seo.ApplyTerm(ctx, other, PageSEOForList("Alpha", "/razdely/alpha")); got.Title != PageSEOForList("Alpha", "/razdely/alpha").Title {
		t.Errorf("other: %q", got.Title)
	}
}
