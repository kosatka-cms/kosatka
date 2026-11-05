package http

import (
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// Mux - what route registration needs from a router.
type Mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// FeatureGate answers 404 while enabled reports false: API paths with a
// JSON error, pages with notFound.
func FeatureGate(enabled func() bool, notFound http.HandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if enabled() {
				next.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/api/") {
				httputil.JSONError(w, "Not found", http.StatusNotFound)
				return
			}
			notFound(w, r)
		})
	}
}

// gatedMux registers every route behind a gate.
type gatedMux struct {
	mux  Mux
	gate func(http.Handler) http.Handler
}

func (g gatedMux) Handle(pattern string, handler http.Handler) {
	g.mux.Handle(pattern, g.gate(handler))
}

func (g gatedMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	g.Handle(pattern, http.HandlerFunc(handler))
}

// ModuleMux - mux whose routes answer 404 while the built-in module or
// feature switch is off.
func ModuleMux(mux Mux, module string, notFound http.HandlerFunc) Mux {
	return gatedMux{mux: mux, gate: FeatureGate(func() bool { return service.ModuleEnabled(module) }, notFound)}
}
