package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// TestTextFiltersFromForm - the ticked offered filters are saved, an
// unticked one dropped, a saved one not offered (its module is off) kept;
// a name not offered can't be ticked in.
func TestTextFiltersFromForm(t *testing.T) {
	service.RegisterTextFilter(service.TextFilter{Name: "zz_offered", Module: "zz_on"})
	service.RegisterTextFilter(service.TextFilter{Name: "zz_ticked", Module: "zz_on"})
	service.SetModules(map[string]bool{"zz_off": false})
	t.Cleanup(func() { service.SetModules(map[string]bool{"zz_off": true}) })
	form := url.Values{"filters": {"zz_ticked", "zz_unknown"}}
	r := httptest.NewRequest(http.MethodPost, "/admin/config/text-formats/1", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if got := textFiltersFromForm(r, []string{"zz_offered", "zz_disabled"}); got != "zz_disabled zz_ticked" {
		t.Errorf("filters: %q", got)
	}
}
