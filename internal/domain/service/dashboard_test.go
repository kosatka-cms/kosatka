package service

import (
	"testing"
)

// TestDashboardBlocks - the modules' blocks lightest first, as registered
// among equals; a reset forgets them.
func TestDashboardBlocks(t *testing.T) {
	t.Cleanup(ResetDashboardBlocks)
	ResetDashboardBlocks()
	RegisterDashboardBlock(DashboardBlock{Name: "late", Weight: 10})
	RegisterDashboardBlock(DashboardBlock{Name: "first"})
	RegisterDashboardBlock(DashboardBlock{Name: "second"})
	var got []string
	for _, b := range DashboardBlocks() {
		got = append(got, b.Name)
	}
	if len(got) != 3 || got[0] != "first" || got[1] != "second" || got[2] != "late" {
		t.Errorf("order: %v", got)
	}
	ResetDashboardBlocks()
	if len(DashboardBlocks()) != 0 {
		t.Error("not reset")
	}
}
