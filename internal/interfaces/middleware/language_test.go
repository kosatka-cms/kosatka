package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestLanguageMiddleware - an address with a language's prefix is served
// without it in that language; /en is the front page; another address -
// as is, in the default language.
func TestLanguageMiddleware(t *testing.T) {
	entity.SetLanguagesSwitch(func() bool { return true })
	entity.SetSiteLanguages("ru", []entity.SiteLanguage{{Code: "en", Prefix: "en"}})
	t.Cleanup(func() { entity.SetSiteLanguages("", nil); entity.SetLanguagesSwitch(nil) })
	var path, lang string
	h := LanguageMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		path, lang = r.URL.Path, i18n.FromContext(r.Context())
	}))
	for in, want := range map[string][2]string{
		"/en/news/x":   {"/news/x", "en"},
		"/en":          {"/", "en"},
		"/en/":         {"/", "en"},
		"/news/x":      {"/news/x", i18n.Default()},
		"/english/x":   {"/english/x", i18n.Default()},
		"/static/a.js": {"/static/a.js", i18n.Default()},
	} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, in, nil))
		if path != want[0] || lang != want[1] {
			t.Errorf("%s: %q in %q, want %q in %q", in, path, lang, want[0], want[1])
		}
	}
}
