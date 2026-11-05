package app

import (
	"context"
	"net/http"
	"testing"
)

// TestRouteAnswers - an address one of the engine's routes takes is
// answered; one only the catch-all (content and the fallbacks) takes
// isn't.
func TestRouteAnswers(t *testing.T) {
	mux := http.NewServeMux()
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, p := range []string{"GET /rss.xml", "GET /users/{alias}", "POST /login", catchAllPattern} {
		mux.Handle(p, h)
	}
	for path, want := range map[string]bool{"rss.xml": true, "users/ann": true, "login": false, "feeds/news.xml": false} {
		if got, err := routeAnswers(context.Background(), mux, path); err != nil || got != want {
			t.Errorf("%s: %v %v, want %v", path, got, err, want)
		}
	}
}
