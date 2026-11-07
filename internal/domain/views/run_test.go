package views

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestGlossaryLetterFilter - the synthetic (never persisted) filter RunView
// appends when ViewDisplayDef.ShowGlossary is on, built from the
// request's ?letter=X, bypassing CreateFilter/validateFilter; covers the
// title and the generic-field targets.
func TestGlossaryLetterFilter(t *testing.T) {
	t.Run("uppercases and trims the letter", func(t *testing.T) {
		// Cyrillic on purpose: checks uppercasing of a non-ASCII letter.
		got := glossaryLetterFilter(entity.ViewDisplayDef{}, "  б  ")
		if got.Value != "Б" {
			t.Fatalf("got Value %q, want %q", got.Value, "Б")
		}
	})

	t.Run("targets the title builtin with starts_with when GlossaryFieldStorageID is nil", func(t *testing.T) {
		got := glossaryLetterFilter(entity.ViewDisplayDef{}, "A")
		if got.BuiltinField != entity.ViewFilterBuiltinTitle {
			t.Fatalf("got BuiltinField %q, want %q", got.BuiltinField, entity.ViewFilterBuiltinTitle)
		}
		if got.Operator != entity.ViewFilterOperatorStartsWith {
			t.Fatalf("got Operator %q, want %q", got.Operator, entity.ViewFilterOperatorStartsWith)
		}
		if got.FieldStorageID != nil {
			t.Fatalf("expected a builtin target (nil FieldStorageID), got %v", *got.FieldStorageID)
		}
	})

	t.Run("targets the generic field when GlossaryFieldStorageID is set", func(t *testing.T) {
		id := 42
		got := glossaryLetterFilter(entity.ViewDisplayDef{GlossaryFieldStorageID: &id}, "A")
		if got.FieldStorageID == nil || *got.FieldStorageID != id {
			t.Fatalf("got FieldStorageID %v, want %d", got.FieldStorageID, id)
		}
		if got.BuiltinField != "" {
			t.Fatalf("expected BuiltinField unset when targeting a generic field, got %q", got.BuiltinField)
		}
	})
}

func TestViewByURLArgument(t *testing.T) {
	repo := &fakeViewRepo{displays: map[int]entity.ViewDisplayDef{
		1: {ID: 1, DisplayType: entity.ViewDisplayTypePage, URLPath: "posts/%"},
		2: {ID: 2, DisplayType: entity.ViewDisplayTypePage, URLPath: "post"},
	}}
	s := newTestViewService(repo)
	v, arg, err := s.ViewByURLArgument(context.Background(), "posts/ishmael")
	if err != nil || v.ID != 1 || arg != "ishmael" {
		t.Fatalf("posts/ishmael: %+v %q %v", v, arg, err)
	}
	if _, _, err := s.ViewByURLArgument(context.Background(), "news/ishmael"); err == nil {
		t.Fatal("news/ishmael must not match")
	}
}

// TestRunViewSkipItems - a run skips the display's effective SkipItems
// (a display inheriting the settings - its Default's), for content and
// for comments; a preview - the draft value.
func TestRunViewSkipItems(t *testing.T) {
	viewID := 3
	repo := &fakeViewRepo{displays: map[int]entity.ViewDisplayDef{
		1: {ID: 1, ViewID: &viewID, IsDefault: true, PageSize: 5, SkipItems: 2},
		2: {ID: 2, ViewID: &viewID, PageSize: 9, SkipItems: 7},
	}}
	s := newTestViewService(repo)
	ctx := context.Background()
	if _, _, err := s.RunView(ctx, repo.displays[2], nil, 1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.RunCommentView(ctx, repo.displays[2], nil, 1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.PreviewView(ctx, repo.displays[2], PreviewSettings{PageSize: 4, SkipItems: 1}); err != nil {
		t.Fatal(err)
	}
	if len(repo.runs) != 3 {
		t.Fatalf("%d runs, want 3", len(repo.runs))
	}
	for i, want := range []struct{ skip, pageSize int }{{2, 5}, {2, 5}, {1, 4}} {
		if got := repo.runs[i]; got.Skip != want.skip || got.PageSize != want.pageSize {
			t.Errorf("run %d: skip %d, page size %d, want %d, %d", i, got.Skip, got.PageSize, want.skip, want.pageSize)
		}
	}
}
