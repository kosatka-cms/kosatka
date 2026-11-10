package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/lib/metrics"
	"github.com/kosatka-cms/kosatka/internal/lib/requestid"
)

func TestRequestIDMiddlewareNewID(t *testing.T) {
	var seen string
	h := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestid.FromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set(requestid.Header, "from-a-visitor")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if seen == "" || seen == "from-a-visitor" {
		t.Fatalf("id %q: a visitor's header taken or none made", seen)
	}
	if got := rec.Header().Get(requestid.Header); got != seen {
		t.Errorf("header %q, context %q", got, seen)
	}
}

func TestRequestIDMiddlewareTrustedProxy(t *testing.T) {
	for header, keep := range map[string]bool{"abc123": true, "bad id\n": false} {
		var seen string
		h := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = requestid.FromContext(r.Context())
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:5000" // loopback: trusted by default
		req.Header.Set(requestid.Header, header)
		h.ServeHTTP(httptest.NewRecorder(), req)
		if (seen == header) != keep || seen == "" {
			t.Errorf("header %q: id %q", header, seen)
		}
	}
}

// observed - a mux with one route behind the observer, a layer between
// them copying the request (as the site's middleware does) and setting
// the user.
func observed(o *HTTPObserver) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /node/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	mux.HandleFunc("GET /preview/{token}", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("POST /fail", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusInternalServerError)
	})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), userIDKey, 7)
		RouteRecorder(mux).ServeHTTP(w, r.WithContext(ctx))
	})
	return RequestIDMiddleware(o.Middleware(inner))
}

func TestHTTPObserverMetrics(t *testing.T) {
	reg := metrics.NewRegistry()
	h := observed(NewHTTPObserver(reg, nil))
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/node/1"}, {http.MethodGet, "/node/2"}, {http.MethodPost, "/fail"},
		{http.MethodGet, "/missing"}, {"BREW", "/node/1"},
	} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(c.method, c.path, nil))
	}
	var b strings.Builder
	if err := reg.Write(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		`kosatka_http_requests_total{method="GET",route="/node/{id}",code="200"} 2`,
		`kosatka_http_requests_total{method="POST",route="/fail",code="500"} 1`,
		`kosatka_http_requests_total{method="GET",route="none",code="404"} 1`,
		`kosatka_http_requests_total{method="OTHER",route="none",code="405"} 1`,
		`kosatka_http_request_duration_seconds_count{route="/node/{id}"} 2`,
		"kosatka_http_requests_in_flight 0",
	} {
		if !strings.Contains(out, want+"\n") {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
}

func TestHTTPObserverRequestLog(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(requestid.NewHandler(slog.NewJSONHandler(&buf, nil)))
	h := observed(NewHTTPObserver(nil, log))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/node/5?secret=1", nil))
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("%v: %s", err, buf.String())
	}
	want := map[string]any{
		"msg": "request", "method": "GET", "path": "/node/5", "route": "/node/{id}",
		"status": float64(200), "bytes": float64(5), "user_id": float64(7),
		"request_id": rec.Header().Get(requestid.Header),
	}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("%s = %v, want %v (%s)", k, line[k], v, buf.String())
		}
	}
	if _, ok := line["duration_ms"]; !ok {
		t.Errorf("no duration_ms: %s", buf.String())
	}
	if strings.Contains(buf.String(), "secret") {
		t.Errorf("the query string logged: %s", buf.String())
	}
}

func TestHTTPObserverNil(t *testing.T) {
	if NewHTTPObserver(nil, nil) != nil {
		t.Fatal("an observer with nothing to do")
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	var o *HTTPObserver
	if h := o.Middleware(next); h == nil {
		t.Error("nil handler")
	}
}

func TestRouteLabel(t *testing.T) {
	for in, want := range map[string]string{"": RouteNone, "GET /a/{id}": "/a/{id}", "/b/": "/b/"} {
		if got := routeLabel(in); got != want {
			t.Errorf("routeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHTTPObserverSecretPath(t *testing.T) {
	var buf bytes.Buffer
	h := observed(NewHTTPObserver(nil, slog.New(slog.NewJSONHandler(&buf, nil))))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/preview/s3cr3t", nil))
	if strings.Contains(buf.String(), "s3cr3t") || !strings.Contains(buf.String(), `"path":"/preview/{token}"`) {
		t.Errorf("the token logged: %s", buf.String())
	}
}
