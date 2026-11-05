package admin

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func TestFilterValueDisplays(t *testing.T) {
	fieldStorageTypes := map[int]entity.ContentTypeFieldType{
		1: entity.FieldTypeTaxonomy,
		2: entity.FieldTypeText,
	}
	termNames := map[int]string{10: "Alpha", 20: "Beta"}

	t.Run("taxonomy filter resolves term IDs to names", func(t *testing.T) {
		fieldID := 1
		filters := entity.ViewFilterDefs{{ID: 1, FieldStorageID: &fieldID, Value: "10,20"}}
		got := filterValueDisplays(context.Background(), filters, fieldStorageTypes, termNames)
		if got[1] != "Alpha, Beta" {
			t.Fatalf("got %q, want %q", got[1], "Alpha, Beta")
		}
	})

	t.Run("unknown term ID falls back to the raw ID", func(t *testing.T) {
		fieldID := 1
		filters := entity.ViewFilterDefs{{ID: 1, FieldStorageID: &fieldID, Value: "10,999"}}
		got := filterValueDisplays(context.Background(), filters, fieldStorageTypes, termNames)
		if got[1] != "Alpha, 999" {
			t.Fatalf("got %q, want %q", got[1], "Alpha, 999")
		}
	})

	t.Run("non-taxonomy filter keeps its raw value", func(t *testing.T) {
		fieldID := 2
		filters := entity.ViewFilterDefs{{ID: 1, FieldStorageID: &fieldID, Value: "hello"}}
		got := filterValueDisplays(context.Background(), filters, fieldStorageTypes, termNames)
		if got[1] != "hello" {
			t.Fatalf("got %q, want %q", got[1], "hello")
		}
	})

	t.Run("builtin filter (no FieldStorageID) keeps its raw value", func(t *testing.T) {
		filters := entity.ViewFilterDefs{{ID: 1, Value: "video"}}
		got := filterValueDisplays(context.Background(), filters, fieldStorageTypes, termNames)
		if got[1] != "video" {
			t.Fatalf("got %q, want %q", got[1], "video")
		}
	})
}
