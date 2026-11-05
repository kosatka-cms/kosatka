package interfaces

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestViewBlockPageOpenedAsPage - a block's page link opened without the
// script (a new tab, a crawler) goes back to the page it was on, never
// the JSON; another site's referrer - the front page.
func TestViewBlockPageOpenedAsPage(t *testing.T) {
	c := &IndexController{}
	for referer, want := range map[string]string{
		"http://example.com/articles?sort=new": "/articles?sort=new",
		"https://elsewhere.example/x":          "/",
		"":                                     "/",
	} {
		r := httptest.NewRequest(http.MethodGet, "http://example.com/api/view-block/9?page=2", nil)
		if referer != "" {
			r.Header.Set("Referer", referer)
		}
		w := httptest.NewRecorder()
		c.ViewBlockPage(w, r)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != want {
			t.Errorf("referer %q: %d %q, want %q", referer, w.Code, w.Header().Get("Location"), want)
		}
	}
}
