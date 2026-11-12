package app

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/lib/metrics"
)

func TestNewMetricsRegistryOff(t *testing.T) {
	if reg := newMetricsRegistry(&config.Config{}, nil, nil); reg != nil {
		t.Error("metrics without metrics.listen")
	}
	if startMetricsServer("127.0.0.1:0", nil, slog.New(slog.DiscardHandler)) != nil {
		t.Error("a metrics server without metrics")
	}
}

func TestMetricsHandler(t *testing.T) {
	db, err := sql.Open("mysql", "u:p@tcp(127.0.0.1:1)/x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	reg := newMetricsRegistry(&config.Config{Metrics: config.MetricsConfig{Listen: "127.0.0.1:0"}}, db, nil)
	h := metricsHandler(reg, slog.New(slog.DiscardHandler))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != metrics.ContentType {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{"kosatka_build_info{", "go_goroutines ", "kosatka_db_open_connections ", "kosatka_db_up 1"} {
		if !strings.Contains(string(body), "\n"+want) {
			t.Errorf("no %s in\n%s", want, body)
		}
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("/: %d", rec.Code)
	}
}

func TestRequestLogger(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	if requestLogger(&config.Config{}, log) != nil {
		t.Error("request lines without log.requests")
	}
	if requestLogger(&config.Config{Log: config.LogConfig{Requests: true}}, log) != log {
		t.Error("no request lines with log.requests")
	}
}
