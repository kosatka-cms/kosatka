package forms

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// RegisterWebformRoutes registers a form's submission (the forms module),
// open to anonymous visitors and rate limited.
func RegisterWebformRoutes(r modulekit.Mux, store *sessions.CookieStore, wc *WebformController, submitRL *ratelimit.RateLimiter) {
	optionalAuth := modulekit.OptionalAuthMiddleware(store)
	rl := ratelimit.RateLimitMiddleware(submitRL, httputil.ClientIP)
	r.Handle("POST /node/{id}/form", rl(optionalAuth(http.HandlerFunc(wc.Submit))))
	r.Handle("GET /node/{id}/form/submissions", optionalAuth(http.HandlerFunc(wc.Submissions)))
	r.Handle("GET /node/{id}/form/embed", optionalAuth(http.HandlerFunc(wc.Embed)))
	r.Handle("POST /node/{id}/form/submissions/delete", optionalAuth(http.HandlerFunc(wc.DeleteSubmissions)))
	r.Handle("POST /node/{id}/form/submissions/purge", optionalAuth(http.HandlerFunc(wc.PurgeSubmissions)))
	r.Handle("GET /node/{id}/form/analysis", optionalAuth(http.HandlerFunc(wc.Analysis)))
	r.Handle("GET /node/{id}/form/submissions/{sid}", optionalAuth(http.HandlerFunc(wc.Submission)))
	r.Handle("GET /admin/forms", optionalAuth(http.HandlerFunc(wc.FormsList)))
	r.Handle("GET /admin/forms/{id}/delete", optionalAuth(http.HandlerFunc(wc.DeleteFormConfirm)))
	r.Handle("POST /admin/forms/{id}/delete", optionalAuth(http.HandlerFunc(wc.DeleteForm)))
	r.Handle("GET /node/{id}/form/file", optionalAuth(http.HandlerFunc(wc.File)))
	r.Handle("GET /node/{id}/form/submissions.csv", optionalAuth(http.HandlerFunc(wc.SubmissionsCSV)))
}
