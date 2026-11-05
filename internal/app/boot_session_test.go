package app

import (
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/config"
)

// TestSessionStoreMaxAge - the session's lifetime holds on the server
// too: the cookie codecs refuse a cookie older than session.max_age, not
// only after their own 30 days.
func TestSessionStoreMaxAge(t *testing.T) {
	cfg := &config.Config{}
	cfg.Session.SecretKey = "test-secret-key-0123456789abcdef"
	cfg.Session.MaxAge = 3600
	store := newSessionStore(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if store.Options.MaxAge != 3600 {
		t.Fatalf("cookie max age %d", store.Options.MaxAge)
	}
	for i, codec := range store.Codecs {
		if got := reflect.ValueOf(codec).Elem().FieldByName("maxAge").Int(); got != 3600 {
			t.Errorf("codec %d accepts cookies for %d s", i, got)
		}
	}
}
