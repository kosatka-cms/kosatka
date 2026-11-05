package entity

import "testing"

func TestUserSlug(t *testing.T) {
	if got := UserSlug(" Peter Parker "); got != "users/Peter-Parker" {
		t.Errorf("UserSlug = %q", got)
	}
	if got := UserAlias("users/ishmael"); got != "ishmael" {
		t.Errorf("UserAlias = %q", got)
	}
	for _, slug := range []string{"users/ishmael", "ishmael"} {
		if got := UserURL(slug); got != "/users/ishmael" {
			t.Errorf("UserURL(%q) = %q", slug, got)
		}
	}
}
