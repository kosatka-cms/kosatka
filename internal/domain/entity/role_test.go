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

// TestRoleGrants - a permission granted (true) or not; full access isn't
// every permission here.
func TestRoleGrants(t *testing.T) {
	r := Role{Permissions: map[string]any{PermFullAccess: true, "edit": false, "odd": "yes"}}
	if !r.Grants(PermFullAccess) {
		t.Error("full access granted")
	}
	for _, perm := range []string{"edit", "odd", "view"} {
		if r.Grants(perm) {
			t.Errorf("%s isn't granted", perm)
		}
	}
	if (Role{}).Grants(PermFullAccess) {
		t.Error("a role without permissions grants nothing")
	}
}
