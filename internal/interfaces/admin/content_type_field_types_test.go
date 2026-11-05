package admin

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestFieldTypesHaveSettings - every field type is decided for the
// field's page: its settings (adminFieldTypes; none - an empty entry).
func TestFieldTypesHaveSettings(t *testing.T) {
	for _, ft := range entity.FieldTypes {
		if _, ok := adminFieldTypes[ft]; !ok {
			t.Errorf("%s: not in adminFieldTypes - decide its settings", ft)
		}
	}
}
