package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kosatka-cms/kosatka/internal/lib/metrics"
	"github.com/kosatka-cms/kosatka/internal/lib/requestid"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// RequestIDMiddleware gives every request an id: a trusted proxy's
// X-Request-Id when it has a valid one (nginx's $request_id, so its log
// and the site's share it), else a new one. The id goes back in the
// X-Request-Id header and in the request's context (requestid.FromContext;
// the log lines written with the context carry it).
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ""
		if httputil.FromTrustedProxy(r) {
			if h := r.Header.Get(requestid.Header); requestid.Valid(h) {
				id = h
			}
		}
		if id == "" {
			id = requestid.New()
		}
		w.Header().Set(requestid.Header, id)
		next.ServeHTTP(w, r.WithContext(requestid.WithID(r.Context(), id)))
	})
}

// RouteNone - the route label of a request no route served (a redirect,
// a blocked visitor, the maintenance page, an address nothing matches).
const RouteNone = "none"

// routeInfo - what the routes learned about a request (RouteRecorder),
// for the layers outside them: the pattern that matched, the user,
// whether the path holds a secret.
type routeInfo struct {
	pattern string
	userID  int
	// secret - a {token} in the route (a password reset, a preview, an
	// unsubscribe link): the path isn't logged, its route is.
	secret bool
}

type routeInfoKey struct{}

// RouteRecorder wraps the routes (the ServeMux): after a request, it
// tells HTTPObserver the pattern that served it and the signed-in user,
// which the layers outside can't see in their copy of the request.
func RouteRecorder(routes http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routes.ServeHTTP(w, r)
		if ri, ok := r.Context().Value(routeInfoKey{}).(*routeInfo); ok {
			// ServeMux sets Pattern on the request it was given.
			ri.pattern = r.Pattern
			ri.userID, _ = r.Context().Value(userIDKey).(int)
			ri.secret = r.PathValue("token") != ""
		}
	})
}

// HTTPObserver - the requests' metrics (kosatka_http_…) and their log
// lines, each optional.
type HTTPObserver struct {
	requests *metrics.CounterVec
	duration *metrics.HistogramVec
	inFlight atomic.Int64
	log      *slog.Logger
}

// DurationBuckets - the upper bounds (seconds) of the response time
// histogram.
var DurationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

// NewHTTPObserver - the requests' metrics added to reg (nil - none) and a
// line a request written to requestLog (nil - none); nil when neither.
func NewHTTPObserver(reg *metrics.Registry, requestLog *slog.Logger) *HTTPObserver {
	if reg == nil && requestLog == nil {
		return nil
	}
	o := &HTTPObserver{log: requestLog}
	if reg != nil {
		o.requests = reg.Counter("kosatka_http_requests_total",
			"HTTP requests answered, by method, route (the pattern that served it) and status code.",
			"method", "route", "code")
		o.duration = reg.Histogram("kosatka_http_request_duration_seconds",
			"Time to answer an HTTP request, by route.", DurationBuckets, "route")
		reg.GaugeFunc("kosatka_http_requests_in_flight", "HTTP requests being answered.",
			func() float64 { return float64(o.inFlight.Load()) })
	}
	return o
}

// Middleware counts and times each request and writes its log line. It
// goes outside the routes' layers, which RouteRecorder (around the routes)
// tells the route; a nil observer - next as it is.
func (o *HTTPObserver) Middleware(next http.Handler) http.Handler {
	if o == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		o.inFlight.Add(1)
		defer o.inFlight.Add(-1)
		ri := &routeInfo{}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r.WithContext(context.WithValue(r.Context(), routeInfoKey{}, ri)))
		elapsed := time.Since(start)
		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}
		route := routeLabel(ri.pattern)
		if o.requests != nil {
			o.requests.Inc(methodLabel(r.Method), route, strconv.Itoa(status))
			o.duration.Observe(elapsed.Seconds(), route)
		}
		if o.log != nil {
			path := r.URL.Path
			if ri.secret {
				path = route
			}
			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", path),
				slog.String("route", route),
				slog.Int("status", status),
				slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000),
				slog.Int64("bytes", sw.bytes),
			}
			if ri.userID != 0 {
				attrs = append(attrs, slog.Int("user_id", ri.userID))
			}
			o.log.LogAttrs(r.Context(), slog.LevelInfo, "request", attrs...)
		}
	})
}

// routeLabel - a ServeMux pattern without its method ("GET /node/{id}" -
// "/node/{id}"), RouteNone for none: a bounded set of labels, unlike the
// paths.
func routeLabel(pattern string) string {
	if pattern == "" {
		return RouteNone
	}
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		pattern = pattern[i+1:]
	}
	return pattern
}

// methodLabel - the method, "OTHER" for one HTTP doesn't define (any
// string a client sends would be a label of its own).
func methodLabel(m string) string {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		return m
	}
	return "OTHER"
}

// statusWriter notes the status and the bytes written.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *statusWriter) WriteHeader(code int) {
	// 1xx (103 Early Hints) may come before the final status.
	if w.status == 0 && code >= 200 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController reach the writer underneath.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Flush passes a flush on (a streamed answer).
func (w *statusWriter) Flush() {
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
