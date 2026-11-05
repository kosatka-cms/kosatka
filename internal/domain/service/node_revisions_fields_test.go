package service

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestFieldTypesInRevisions - every field type is decided for the
// history: how its values compare and whether reverting drops what they
// name (revisionFieldTypes).
func TestFieldTypesInRevisions(t *testing.T) {
	for _, ft := range entity.FieldTypes {
		if _, ok := revisionFieldTypes[ft]; !ok {
			t.Errorf("%s: not in revisionFieldTypes - decide how the history shows it", ft)
		}
	}
}
