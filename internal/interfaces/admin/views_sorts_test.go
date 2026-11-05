package admin

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

// TestSortFromFormExposure - the offer to visitors is read only when it
// is ticked: unticked, a key and a label left in the form are dropped.
func TestSortFromFormExposure(t *testing.T) {
	form := func(exposed string) url.Values {
		return url.Values{"target": {"builtin:title"}, "direction": {"asc"}, "exposed": {exposed}, "exposed_key": {"title"}, "exposed_label": {"By title"}}
	}
	for exposed, want := range map[string][2]string{"1": {"title", "By title"}, "": {"", ""}} {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form(exposed).Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		sd, code := sortFromForm(r, 7, nil)
		if code != "" || sd.ExposedKey != want[0] || sd.ExposedLabel != want[1] {
			t.Errorf("exposed=%q: got %q, %q (code %q), want %q, %q", exposed, sd.ExposedKey, sd.ExposedLabel, code, want[0], want[1])
		}
	}
}

// TestSortErrorCode - the service's refusals of an offer as the form's
// error codes.
func TestSortErrorCode(t *testing.T) {
	for err, want := range map[error]string{
		views.ErrSortFieldTypeUnsupported: "field-type",
		views.ErrSortExposedKeyInvalid:    "exposed-key",
		views.ErrSortExposedLabelRequired: "exposed-label",
		views.ErrSortExposedKeyTaken:      "exposed-key-taken",
	} {
		if got := sortErrorCode(fmt.Errorf("saving: %w", err)); got != want {
			t.Errorf("%v: got %q, want %q", err, got, want)
		}
	}
}
