package middleware

import (
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// LanguageMiddleware - the request's language from its address's prefix
// (entity.LanguageOfPrefix): /en/news/x is served as /news/x in English
// (i18n.WithLang: the interface, a new node's language), /en and /en/ as
// the front page; any other address - the site's default language. The
// site has no other languages - nothing to do.
func LanguageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
		code, ok := entity.LanguageOfPrefix(prefix)
		if !ok || prefix == "" {
			next.ServeHTTP(w, r)
			return
		}
		r = r.Clone(i18n.WithLang(r.Context(), code))
		r.URL.Path = "/" + rest
		r.URL.RawPath = ""
		next.ServeHTTP(w, r)
	})
}
