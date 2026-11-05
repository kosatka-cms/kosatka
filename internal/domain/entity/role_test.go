package entity

import "testing"

// TestRoleLabel - the readable name, or the machine name without one.
func TestRoleLabel(t *testing.T) {
	if got := (Role{Name: "editor", Description: "Editors"}).Label(); got != "Editors" {
		t.Errorf("with a readable name: %q", got)
	}
	if got := (Role{Name: "editor"}).Label(); got != "editor" {
		t.Errorf("without a readable name: %q", got)
	}
}
