package viewquery

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func TestResolveViewFilterValue(t *testing.T) {
	t.Run("fixed filter with a value", func(t *testing.T) {
		f := entity.ViewFilterDef{Exposed: false, Value: "5"}
		v, ok := resolveViewFilterValue(f, nil)
		if !ok || v != "5" {
			t.Fatalf("got (%q, %v), want (%q, true)", v, ok, "5")
		}
	})

	t.Run("fixed filter with an empty value has no constraint", func(t *testing.T) {
		f := entity.ViewFilterDef{Exposed: false, Value: ""}
		_, ok := resolveViewFilterValue(f, nil)
		if ok {
			t.Fatalf("expected no constraint for an empty fixed value")
		}
	})

	t.Run("exposed filter reads its query param", func(t *testing.T) {
		f := entity.ViewFilterDef{Exposed: true, ExposedParam: "publisher"}
		v, ok := resolveViewFilterValue(f, map[string]string{"publisher": "alpha"})
		if !ok || v != "alpha" {
			t.Fatalf("got (%q, %v), want (%q, true)", v, ok, "alpha")
		}
	})

	t.Run("exposed filter with a missing param has no constraint", func(t *testing.T) {
		f := entity.ViewFilterDef{Exposed: true, ExposedParam: "publisher"}
		_, ok := resolveViewFilterValue(f, map[string]string{})
		if ok {
			t.Fatalf("expected no constraint when the exposed param is absent")
		}
	})

	t.Run("exposed filter with an empty param has no constraint", func(t *testing.T) {
		f := entity.ViewFilterDef{Exposed: true, ExposedParam: "publisher"}
		_, ok := resolveViewFilterValue(f, map[string]string{"publisher": ""})
		if ok {
			t.Fatalf("expected no constraint when the exposed param is empty")
		}
	})
}

func TestBuiltinFilterPredicate(t *testing.T) {
	t.Run("invalid status value errors", func(t *testing.T) {
		if _, err := builtinFilterPredicate(entity.ViewFilterBuiltinStatus, entity.ViewFilterOperatorEq, "not-a-number"); err == nil {
			t.Fatalf("expected an error for a non-numeric status value")
		}
	})

	t.Run("valid status value builds a predicate", func(t *testing.T) {
		if _, err := builtinFilterPredicate(entity.ViewFilterBuiltinStatus, entity.ViewFilterOperatorEq, "1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unknown builtin field errors", func(t *testing.T) {
		if _, err := builtinFilterPredicate("nonsense", entity.ViewFilterOperatorEq, "1"); err == nil {
			t.Fatalf("expected an error for an unknown builtin field")
		}
	})

	t.Run("content_type eq builds a predicate", func(t *testing.T) {
		if _, err := builtinFilterPredicate(entity.ViewFilterBuiltinContentType, entity.ViewFilterOperatorEq, "video"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("content_type in builds a predicate", func(t *testing.T) {
		if _, err := builtinFilterPredicate(entity.ViewFilterBuiltinContentType, entity.ViewFilterOperatorIn, "video, page"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("title starts_with builds a predicate", func(t *testing.T) {
		if _, err := builtinFilterPredicate(entity.ViewFilterBuiltinTitle, entity.ViewFilterOperatorStartsWith, "B"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// TestBuildFilterPredicatesContextual: a contextual filter on the
// created_by builtin resolves straight from contextCreatedByID without a
// field-storage lookup, so a nil client is safe here.
func TestBuildFilterPredicatesContextual(t *testing.T) {
	filters := entity.ViewFilterDefs{{
		Contextual:   true,
		BuiltinField: entity.ViewFilterBuiltinCreatedBy,
		Operator:     entity.ViewFilterOperatorEq,
	}}

	t.Run("no context makes the whole query impossible, not unfiltered", func(t *testing.T) {
		predicates, impossible, _, err := buildFilterPredicates(context.Background(), nil, filters, nil, nil, 0, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !impossible {
			t.Fatalf("expected impossible=true when contextCreatedByID<=0 (must not silently show unfiltered content)")
		}
		if predicates != nil {
			t.Fatalf("expected no predicates when impossible, got %d", len(predicates))
		}
	})

	t.Run("a context resolves to a created_by predicate", func(t *testing.T) {
		predicates, impossible, _, err := buildFilterPredicates(context.Background(), nil, filters, nil, nil, 42, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if impossible {
			t.Fatalf("expected impossible=false when a context is present")
		}
		if len(predicates) != 1 {
			t.Fatalf("got %d predicates, want 1", len(predicates))
		}
	})
}

func TestParseStringList(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "single value", raw: "video", want: []string{"video"}},
		{name: "comma-separated with spaces", raw: "video, page ,product", want: []string{"video", "page", "product"}},
		{name: "trailing comma ignored", raw: "video,page,", want: []string{"video", "page"}},
		{name: "empty string", raw: "", want: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseStringList(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestParseIntList(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    []int
		wantErr bool
	}{
		{name: "single value", raw: "5", want: []int{5}},
		{name: "comma-separated with spaces", raw: "5, 6 ,7", want: []int{5, 6, 7}},
		{name: "trailing comma ignored", raw: "5,6,", want: []int{5, 6}},
		{name: "non-numeric errors", raw: "5,x", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseIntList(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestDedupeInts(t *testing.T) {
	got := dedupeInts([]int{1, 2, 2, 3, 1})
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestIntersectInts(t *testing.T) {
	got := intersectInts([]int{1, 2, 3, 4}, []int{2, 4, 6})
	want := []int{2, 4}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// TestBuildFilterPredicatesCurrentTerm - has_current_term without a
// term in context matches nothing; with one it adds one predicate.
func TestBuildFilterPredicatesCurrentTerm(t *testing.T) {
	filters := entity.ViewFilterDefs{{Contextual: true, BuiltinField: entity.ViewFilterBuiltinHasCurrentTerm, Operator: entity.ViewFilterOperatorEq}}
	if _, impossible, _, err := buildFilterPredicates(context.Background(), nil, filters, nil, nil, 42, 7, 0); err != nil || !impossible {
		t.Fatalf("no term context: impossible=%v err=%v, want impossible", impossible, err)
	}
	predicates, impossible, _, err := buildFilterPredicates(context.Background(), nil, filters, nil, nil, 0, 0, 5)
	if err != nil || impossible || len(predicates) != 1 {
		t.Fatalf("term context: predicates=%d impossible=%v err=%v, want 1 predicate", len(predicates), impossible, err)
	}
}
