package middleware

import (
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// adminSaveAuditPaths - the admin pages whose saves are "settings
// changed" (audit.SettingsSave): the configuration and the structure,
// the users' fields, the performance page's actions. Pages with actions
// of their own in the log (the Modules page, the roles and permissions,
// content, users) aren't here.
var adminSaveAuditPaths = []string{"/admin/config/", "/admin/people/fields", "/admin/performance/"}

// adminSaveAuditSkip - under adminSaveAuditPaths, but recorded otherwise.
var adminSaveAuditSkip = []string{"/admin/config/modules"}

// AdminSaveAuditMiddleware records a saved admin settings page for the
// action log (audit.SettingsSave): a POST answered with a redirect that
// tells of no error (no "err" in its query: ?error=, ?perf=err…). The
// page is named by its admin menu item.
func AdminSaveAuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !adminSaveAudited(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		rec := &redirectRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status < 300 || rec.status >= 400 || strings.Contains(locationQuery(rec.Header().Get("Location")), "err") {
			return
		}
		obj := audit.Object{Kind: audit.KindSettings, ID: r.URL.Path, Label: r.URL.Path}
		if _, item, ok := entity.AdminMenuCurrent(entity.AdminMenu(), r.URL.Path); ok {
			obj.Label, obj.URL = i18n.T(r.Context(), item.Label), item.Path
		}
		audit.Record(r.Context(), audit.Action{Type: audit.SettingsSave, Object: obj})
	})
}

func adminSaveAudited(path string) bool {
	for _, p := range adminSaveAuditSkip {
		if path == p || strings.HasPrefix(path, p+"/") {
			return false
		}
	}
	for _, p := range adminSaveAuditPaths {
		if strings.HasPrefix(path, p) || path == strings.TrimSuffix(p, "/") {
			return true
		}
	}
	return false
}

// locationQuery - the query of a redirect's Location.
func locationQuery(location string) string {
	if i := strings.IndexByte(location, '?'); i >= 0 {
		return location[i+1:]
	}
	return ""
}

// redirectRecorder keeps the status a handler answers with (and passes
// Flush on, for streamed answers).
type redirectRecorder struct {
	http.ResponseWriter
	status int
}

func (r *redirectRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *redirectRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// Flush passes a flush on when the writer can.
func (r *redirectRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
