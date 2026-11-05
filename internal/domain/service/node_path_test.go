package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakePathTerms map[int]entity.TaxonomyTerm

func (f fakePathTerms) TermsByIDs(_ context.Context, ids []int) (map[int]entity.TaxonomyTerm, error) {
	out := map[int]entity.TaxonomyTerm{}
	for _, id := range ids {
		if t, ok := f[id]; ok {
			out[id] = t
		}
	}
	return out, nil
}

// fakePathSlugs - the slugs taken, by the node that has each.
type fakePathSlugs map[string]int

func (f fakePathSlugs) SlugExistsExcludingNode(_ context.Context, slug string, excludeNodeID int) (bool, error) {
	owner, ok := f[slug]
	return ok && owner != excludeNodeID, nil
}

// TestNodePath - the default pattern (the type and the title, as before
// patterns); a pattern with the creation date and a term with its parent;
// a taken slug gets -1 unless the node itself has it; a title with nothing
// to transliterate falls back to the type and a timestamp.
func TestNodePath(t *testing.T) {
	sections := entity.TaxonomyVocabulary{Name: "Sections", TermPathPrefix: "sections"}
	terms := fakePathTerms{5: {ID: 5, Name: "Dinghies", Slug: "sections/sailing/dinghies", Vocabulary: sections}}
	fields := entity.FieldInstanceDefs{{ID: 1, MachineName: "field_section", FieldType: entity.FieldTypeTaxonomy}}
	created := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	dc := func(id int, title string) entity.DynamicContent {
		return entity.DynamicContent{Node: entity.Node{ID: id, Title: title, CreatedAt: created}, Fields: fields, Values: map[int]any{1: 5}}
	}
	s := NewNodePathService(terms, fakePathSlugs{"news/zvyozdnyy-chas": 7})
	ctx := context.Background()
	for _, tc := range []struct {
		pattern string
		dc      entity.DynamicContent
		want    string
	}{
		{"", dc(0, "Звёздный час"), "news/zvyozdnyy-chas-1"},
		{"", dc(7, "Звёздный час"), "news/zvyozdnyy-chas"},
		{"[node:field:field_section]/[node:created:Y]/[node:title]", dc(0, "Новый номер"), "sailing/dinghies/2026/novyy-nomer"},
		{"blog/[node:created:Y]-[node:created:m]/[node:title]", dc(0, "Hello"), "blog/2026-10/hello"},
		// A token without a value leaves no empty segment.
		{"[node:field:field_missing]/[node:title]", dc(0, "Hello"), "hello"},
	} {
		got, err := s.Path(ctx, entity.ContentTypeDef{MachineName: "news", PathPattern: tc.pattern}, tc.dc)
		if err != nil || got != tc.want {
			t.Errorf("%q: %q %v, want %q", tc.pattern, got, err, tc.want)
		}
	}
	got, err := s.Path(ctx, entity.ContentTypeDef{MachineName: "news"}, dc(0, "!!!"))
	if err != nil || !strings.HasPrefix(got, "news/") || len(got) < len("news/1") {
		t.Errorf("no title: %q %v", got, err)
	}
}

func TestValidateNodePathPattern(t *testing.T) {
	fields := entity.FieldInstanceDefs{
		{MachineName: "field_section", FieldType: entity.FieldTypeTaxonomy},
		{MachineName: "field_cover", FieldType: entity.FieldTypeImage},
		{MachineName: "field_content", FieldType: entity.FieldTypeText},
		{MachineName: "field_height", FieldType: entity.FieldTypeNumber},
	}
	for _, ok := range []string{"", "[node:type]/[node:title]", "news/[node:created:Y]/[node:field:field_section]/[node:title]", "{[node:field:field_section]/}[node:title]", "a_b.c-d/[node:title]"} {
		if err := ValidateNodePathPattern(ok, fields); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for pattern, want := range map[string]error{
		"[node:author]/[node:title]":     ErrNodePathPatternToken,
		"[node:field:field_cover]":       ErrNodePathPatternToken,
		"[node:field:field_content]":     ErrNodePathPatternToken,
		"[node:field:field_height]":      ErrNodePathPatternToken,
		"[node:field:field_nope]":        ErrNodePathPatternToken,
		"News/[node:title]":              ErrNodePathPatternChars,
		"новости/[node:title]":           ErrNodePathPatternChars,
		"news?x=[node:title]":            ErrNodePathPatternChars,
		"[node:title|node:nope]/archive": ErrNodePathPatternToken,
	} {
		if err := ValidateNodePathPattern(pattern, fields); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", pattern, err, want)
		}
	}
}
