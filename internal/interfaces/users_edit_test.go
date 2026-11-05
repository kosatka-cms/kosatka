package interfaces

import (
	"testing"

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
