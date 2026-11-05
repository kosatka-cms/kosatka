package admin

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestViewGroupFieldOptions - the display's own fields as grouping
// choices: field storages by machine name, builtins by their key, the
// View's label or the builtin's.
func TestViewGroupFieldOptions(t *testing.T) {
	id := 7
	got := viewGroupFieldOptions(context.Background(), []entity.ViewFieldDef{
		{FieldStorageID: &id, Label: "World premiere"},
		{BuiltinField: entity.ViewFieldBuiltinCreatedAt},
	}, map[int]string{7: "premiere_us"})
	if len(got) != 2 || got[0] != (viewGroupFieldOption{Key: "premiere_us", Label: "World premiere"}) || got[1].Key != "created_at" || got[1].Label == "" {
		t.Fatalf("options = %+v", got)
	}
}
