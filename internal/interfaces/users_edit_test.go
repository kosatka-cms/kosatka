package interfaces

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// TestUserTimezoneFromForm - a profile/registration time zone is kept
// as current while user time zones are off or the value is invalid;
// empty means the site's.
func TestUserTimezoneFromForm(t *testing.T) {
	t.Cleanup(func() { sitetime.SetUserZones(false) })
	if userTimezoneFromForm("Asia/Omsk", "Europe/Kyiv") != "Europe/Kyiv" {
		t.Fatal("off - unchanged")
	}
	sitetime.SetUserZones(true)
	for value, want := range map[string]string{"Asia/Omsk": "Asia/Omsk", "": "", "Nowhere/X": "Europe/Kyiv", " Asia/Omsk ": "Asia/Omsk"} {
		if got := userTimezoneFromForm(value, "Europe/Kyiv"); got != want {
			t.Errorf("%q: %q, want %q", value, got, want)
		}
	}
}

// TestChosenRole - the profile form's role: the chosen one of those
// offered; a user without a role keeps none when the base role is left
// chosen (the form shows it so) and gets another one chosen; a role not
// offered or no choice keeps the user's own.
func TestChosenRole(t *testing.T) {
	base := entity.Role{ID: 1, Name: entity.RoleUser}
	editor := entity.Role{ID: 3, Name: "editor"}
	choices := entity.Roles{base, editor}
	name := func(r *entity.Role) string {
		if r == nil {
			return "none"
		}
		return r.Name
	}
	for what, c := range map[string]struct {
		own    *entity.Role
		roleID string
		want   string
	}{
		"no role, the base one left chosen": {nil, "1", "none"},
		"no role, another one chosen":       {nil, "3", "editor"},
		"the base role, another one chosen": {&base, "3", "editor"},
		"a role, the base one chosen":       {&editor, "1", entity.RoleUser},
		"a role not offered":                {&editor, "7", "editor"},
		"no choice in the form":             {&editor, "", "editor"},
	} {
		if got := name(chosenRole(&entity.User{Role: c.own}, choices, c.roleID)); got != c.want {
			t.Errorf("%s: %s, want %s", what, got, c.want)
		}
	}
}
