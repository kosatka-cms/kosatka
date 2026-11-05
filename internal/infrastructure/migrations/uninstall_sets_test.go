package migrations_test

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
)

// TestModuleSetsDropOrder - every module's tables have a drop order.
func TestModuleSetsDropOrder(t *testing.T) {
	for _, s := range migrations.Sets() {
		if s.Name == migrations.CoreSet {
			continue
		}
		if got := migrations.DropOrder(s.Tables); len(got) != len(s.Tables) {
			t.Errorf("%s: %d of %d tables", s.Name, len(got), len(s.Tables))
		}
	}
}

// TestFileColumns - a module's columns referencing files come from its
// schema; the core has none to give.
func TestFileColumns(t *testing.T) {
	got := migrations.FileColumns("galleries")
	if len(got) != 1 || got[0] != (migrations.FileColumn{Table: "gallery_items", Column: "fid"}) {
		t.Errorf("galleries: %v", got)
	}
	if got := migrations.FileColumns(migrations.CoreSet); len(got) != 0 {
		t.Errorf("core: %v", got)
	}
	if got := migrations.FileColumns("complaints"); len(got) != 0 {
		t.Errorf("complaints: %v", got)
	}
}
