package views

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestResolveViewFieldTargetLinkTo - the builtin branch only (no
// entity.ContentTypeRepository needed): "link this field to the content"
// and the image formatter's "link image to content/file".
func TestResolveViewFieldTargetLinkTo(t *testing.T) {
	s := &DefaultService{}
	ctx := t.Context()

	t.Run("rejects an unknown LinkTo value", func(t *testing.T) {
		f := entity.ViewFieldDef{BuiltinField: entity.ViewFieldBuiltinTitle, LinkTo: "nonsense"}
		if err := s.resolveViewFieldTarget(ctx, &f); err == nil {
			t.Fatal("expected an error for an unrecognized LinkTo value, got nil")
		}
	})

	t.Run("clears LinkTo=file on the title builtin (not an image)", func(t *testing.T) {
		f := entity.ViewFieldDef{BuiltinField: entity.ViewFieldBuiltinTitle, LinkTo: entity.ViewFieldLinkFile}
		if err := s.resolveViewFieldTarget(ctx, &f); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.LinkTo != "" {
			t.Fatalf("got LinkTo %q, want cleared to \"\" - title isn't an image, \"file\" makes no sense there", f.LinkTo)
		}
	})

	t.Run("keeps LinkTo=content on any builtin", func(t *testing.T) {
		f := entity.ViewFieldDef{BuiltinField: entity.ViewFieldBuiltinTitle, LinkTo: entity.ViewFieldLinkContent}
		if err := s.resolveViewFieldTarget(ctx, &f); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.LinkTo != entity.ViewFieldLinkContent {
			t.Fatalf("got LinkTo %q, want %q kept - \"link to content\" is valid for any field type", f.LinkTo, entity.ViewFieldLinkContent)
		}
	})
}

// TestResolveSortTargetBuiltin - the builtin branch only (no
// entity.ContentTypeRepository needed) of sort criteria, which share
// ViewFilterDef/ViewFieldDef's either/or target shape.
func TestResolveSortTargetBuiltin(t *testing.T) {
	s := &DefaultService{}
	ctx := t.Context()

	for _, builtin := range []entity.ViewSortBuiltin{
		entity.ViewSortBuiltinCreatedAt,
		entity.ViewSortBuiltinViews,
		entity.ViewSortBuiltinTitle,
	} {
		t.Run(string(builtin), func(t *testing.T) {
			sd := entity.ViewSortDef{BuiltinField: builtin}
			if err := s.resolveSortTarget(ctx, &sd); err != nil {
				t.Fatalf("unexpected error for a known builtin %q: %v", builtin, err)
			}
		})
	}

	t.Run("rejects an unknown builtin", func(t *testing.T) {
		sd := entity.ViewSortDef{BuiltinField: "nonsense"}
		if err := s.resolveSortTarget(ctx, &sd); err == nil {
			t.Fatal("expected an error for an unrecognized BuiltinField, got nil")
		}
	})

	t.Run("comment_subject (base_entity==comment)", func(t *testing.T) {
		sd := entity.ViewSortDef{BuiltinField: entity.ViewSortBuiltinCommentSubject}
		if err := s.resolveSortTarget(ctx, &sd); err != nil {
			t.Fatalf("unexpected error for comment_subject: %v", err)
		}
	})
}

// TestResolveViewFieldTargetCommentBuiltin - same "builtin branch
// only" shape as TestResolveViewFieldTargetLinkTo, for the
// base_entity=="comment" field builtins.
func TestResolveViewFieldTargetCommentBuiltin(t *testing.T) {
	s := &DefaultService{}
	ctx := t.Context()

	for _, builtin := range []entity.ViewFieldBuiltinField{
		entity.ViewFieldBuiltinCommentSubject,
		entity.ViewFieldBuiltinCommentContent,
		entity.ViewFieldBuiltinCommentAuthor,
		entity.ViewFieldBuiltinCommentCreatedAt,
		entity.ViewFieldBuiltinCommentNodeTitle,
	} {
		t.Run(string(builtin), func(t *testing.T) {
			f := entity.ViewFieldDef{BuiltinField: builtin}
			if err := s.resolveViewFieldTarget(ctx, &f); err != nil {
				t.Fatalf("unexpected error for a known comment builtin %q: %v", builtin, err)
			}
			if f.ImageStyle != "" {
				t.Fatalf("expected ImageStyle cleared for a non-image comment builtin, got %q", f.ImageStyle)
			}
		})
	}
}

// TestTermComparingBuiltinsAreContextualOnly - shares_terms_with_current
// and similar_to_current_node only work as Contextual filters (their Value
// is a vocabulary scope: empty or a vocabulary ID), with eq as their one
// operator.
func TestTermComparingBuiltinsAreContextualOnly(t *testing.T) {
	for _, builtin := range []entity.ViewFilterBuiltinField{entity.ViewFilterBuiltinSharesTermsWithCurrentNode, entity.ViewFilterBuiltinSimilarToCurrentNode} {
		t.Run(string(builtin), func(t *testing.T) {
			if got := validOperatorsForBuiltin(builtin); !slices.Equal(got, []entity.ViewFilterOperator{entity.ViewFilterOperatorEq}) {
				t.Fatalf("operators = %v, want [eq]", got)
			}
			s := newTestViewService(newFakeViewRepo())
			base := entity.ViewFilterDef{BuiltinField: builtin, Operator: entity.ViewFilterOperatorEq, Value: "3"}
			if err := s.validateFilter(t.Context(), base); !errors.Is(err, ErrFilterContextualUnsupported) {
				t.Fatalf("non-contextual filter: %v, want ErrFilterContextualUnsupported", err)
			}
			base.Contextual = true
			if err := s.validateFilter(t.Context(), base); err != nil {
				t.Fatalf("contextual filter with a vocabulary ID: %v", err)
			}
			base.Value = ""
			if err := s.validateFilter(t.Context(), base); err != nil {
				t.Fatalf("contextual filter of every vocabulary: %v", err)
			}
			base.Value = "tags"
			if err := s.validateFilter(t.Context(), base); !errors.Is(err, ErrFilterOperatorUnsupported) {
				t.Fatalf("a non-numeric vocabulary ID: %v, want ErrFilterOperatorUnsupported", err)
			}
		})
	}
}

// TestHasCurrentTermIsContextualOnly - has_current_term reads the term
// from the page being rendered, so it is accepted only as Contextual.
func TestHasCurrentTermIsContextualOnly(t *testing.T) {
	if got := validOperatorsForBuiltin(entity.ViewFilterBuiltinHasCurrentTerm); !slices.Equal(got, []entity.ViewFilterOperator{entity.ViewFilterOperatorEq}) {
		t.Fatalf("operators = %v, want [eq]", got)
	}
	s := newTestViewService(newFakeViewRepo())
	f := entity.ViewFilterDef{BuiltinField: entity.ViewFilterBuiltinHasCurrentTerm, Operator: entity.ViewFilterOperatorEq}
	if err := s.validateFilter(t.Context(), f); err == nil {
		t.Fatal("non-contextual has_current_term must be rejected")
	}
	f.Contextual = true
	if err := s.validateFilter(t.Context(), f); err != nil {
		t.Fatalf("contextual has_current_term: %v", err)
	}
}

// blockTypes - fakeListTypes with the field sets and their fields.
type blockTypes struct {
	fakeListTypes
	sets   map[string]entity.ContentTypeDef
	fields map[int]entity.FieldInstanceDefs
}

func (f *blockTypes) ContentTypeByMachineName(_ context.Context, name string) (*entity.ContentTypeDef, error) {
	ct, ok := f.sets[name]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	return &ct, nil
}

func (f *blockTypes) FieldsByContentTypeID(_ context.Context, id int) (entity.FieldInstanceDefs, error) {
	return f.fields[id], nil
}

// TestValidateFilterVia - a filter in a blocks field's blocks: through a
// blocks field, of a field of one of its sets.
func TestValidateFilterVia(t *testing.T) {
	repo := &blockTypes{
		fakeListTypes: fakeListTypes{storages: map[int]entity.FieldStorageDef{
			1: {ID: 1, FieldType: entity.FieldTypeBlocks, Config: map[string]any{"block_types": []any{"gone", "role"}}},
			2: {ID: 2, FieldType: entity.FieldTypeText},
			3: {ID: 3, FieldType: entity.FieldTypeText},
		}},
		sets:   map[string]entity.ContentTypeDef{"role": {ID: 7, MachineName: "role", Embedded: true}},
		fields: map[int]entity.FieldInstanceDefs{7: {{FieldStorageID: 2}}},
	}
	s := &DefaultService{contentTypeRepo: repo}
	id := func(v int) *int { return &v }
	for name, tc := range map[string]struct {
		field, via int
		ok         bool
	}{
		"a field of the set":       {2, 1, true},
		"a field of no set":        {3, 1, false},
		"through a non-blocks one": {2, 3, false},
	} {
		t.Run(name, func(t *testing.T) {
			err := s.validateFilter(t.Context(), entity.ViewFilterDef{FieldStorageID: id(tc.field), ViaFieldStorageID: id(tc.via), Operator: entity.ViewFilterOperatorEq, Value: "x"})
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v, want ok %v", err, tc.ok)
			}
			if err != nil && !errors.Is(err, ErrFilterFieldTypeUnsupported) {
				t.Fatalf("err = %v, want ErrFilterFieldTypeUnsupported", err)
			}
		})
	}
}

// TestValidateSortExposure - an offered criterion needs a key of the
// right shape, unused in its display, and a label; not offered - its
// label is dropped.
func TestValidateSortExposure(t *testing.T) {
	repo := newFakeViewRepo()
	repo.sorts[1] = entity.ViewSortDef{ID: 1, ViewDisplayID: 7, ExposedKey: "new", ExposedLabel: "Newest"}
	repo.sorts[2] = entity.ViewSortDef{ID: 2, ViewDisplayID: 8, ExposedKey: "cheap", ExposedLabel: "Cheapest"}
	s := &DefaultService{repository: repo}
	ctx := t.Context()

	for _, tc := range []struct {
		name string
		sd   entity.ViewSortDef
		want error
	}{
		{"not offered", entity.ViewSortDef{ViewDisplayID: 7, ExposedLabel: "stale"}, nil},
		{"offered", entity.ViewSortDef{ViewDisplayID: 7, ExposedKey: " cheap ", ExposedLabel: " Cheapest "}, nil},
		{"the criterion's own key", entity.ViewSortDef{ID: 1, ViewDisplayID: 7, ExposedKey: "new", ExposedLabel: "Newest"}, nil},
		{"a bad key", entity.ViewSortDef{ViewDisplayID: 7, ExposedKey: "price asc", ExposedLabel: "Price"}, ErrSortExposedKeyInvalid},
		{"no label", entity.ViewSortDef{ViewDisplayID: 7, ExposedKey: "price"}, ErrSortExposedLabelRequired},
		{"a key taken in the display", entity.ViewSortDef{ViewDisplayID: 7, ExposedKey: "new", ExposedLabel: "Newer"}, ErrSortExposedKeyTaken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sd := tc.sd
			err := s.validateSortExposure(ctx, &sd)
			if !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if err == nil && sd.ExposedKey == "" && sd.ExposedLabel != "" {
				t.Errorf("a criterion not offered kept its label %q", sd.ExposedLabel)
			}
			if sd.ExposedKey != strings.TrimSpace(sd.ExposedKey) || sd.ExposedLabel != strings.TrimSpace(sd.ExposedLabel) {
				t.Errorf("key and label not trimmed: %q, %q", sd.ExposedKey, sd.ExposedLabel)
			}
		})
	}
}

// TestCheckExposedParam - an exposed filter's parameter: given, of its
// shape, not the page's own, not another exposed filter's of the display.
func TestCheckExposedParam(t *testing.T) {
	repo := newFakeViewRepo()
	repo.filters[5] = entity.ViewFilterDef{ID: 5, ViewDisplayID: 1, Exposed: true, ExposedParam: "genre"}
	s := newTestViewService(repo)
	for param, want := range map[string]error{
		"":         ErrFilterExposedParamRequired,
		"Genre":    ErrFilterExposedParamInvalid,
		"2genre":   ErrFilterExposedParamInvalid,
		"a b":      ErrFilterExposedParamInvalid,
		"page":     ErrFilterExposedParamInvalid,
		"sort":     ErrFilterExposedParamInvalid,
		"letter":   ErrFilterExposedParamInvalid,
		"genre":    ErrFilterExposedParamTaken,
		"platform": nil,
	} {
		err := s.checkExposedParam(t.Context(), entity.ViewFilterDef{ViewDisplayID: 1, Exposed: true, ExposedParam: param})
		if !errors.Is(err, want) && (want != nil || err != nil) {
			t.Errorf("%q: %v, want %v", param, err, want)
		}
	}
	// The filter itself keeps its parameter.
	if err := s.checkExposedParam(t.Context(), entity.ViewFilterDef{ID: 5, ViewDisplayID: 1, Exposed: true, ExposedParam: "genre"}); err != nil {
		t.Errorf("its own parameter: %v", err)
	}
}
