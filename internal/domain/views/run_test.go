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
