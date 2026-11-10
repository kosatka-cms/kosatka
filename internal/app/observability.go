package app

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/kosatka-cms/kosatka/internal/buildinfo"
	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/lib/metrics"
)

// MetricsPath - the metrics server's one address.
const MetricsPath = "/metrics"

// newMetricsRegistry - the service's metrics (the engine's build, the Go
// runtime, the database pool; the requests' are added by
// middleware.NewHTTPObserver) when metrics.listen is set, else nil.
func newMetricsRegistry(cfg *config.Config, db *sql.DB) *metrics.Registry {
	if cfg.Metrics.Listen == "" {
		return nil
	}
	reg := metrics.NewRegistry()
	info := buildinfo.Get()
	reg.GaugeSamples("kosatka_build_info", "The engine's release and commit the binary was built from.",
		[]string{"version", "commit"}, func() []metrics.Sample {
			return []metrics.Sample{{Labels: []string{info.Version, info.Commit}, Value: 1}}
		})
	metrics.RegisterRuntime(reg)
	metrics.RegisterDBStats(reg, "kosatka", db.Stats)
	return reg
}

// requestLogger - where the requests' lines go (log.requests), else nil.
func requestLogger(cfg *config.Config, log *slog.Logger) *slog.Logger {
	if !cfg.Log.Requests {
		return nil
	}
	return log
}

// startMetricsServer serves reg at addr's MetricsPath, nil (no server)
// when reg is nil. A failure to listen (the port taken) is logged: the
// site goes on without its metrics.
func startMetricsServer(addr string, reg *metrics.Registry, log *slog.Logger) *http.Server {
	if reg == nil {
		return nil
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           metricsHandler(reg, log),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	go func() {
		log.Info("Metrics on " + addr + MetricsPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("the metrics server stopped", "addr", addr, "error", err)
		}
	}()
	return srv
}

// metricsHandler - GET MetricsPath: reg in the Prometheus text format;
// any other address a 404.
func metricsHandler(reg *metrics.Registry, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+MetricsPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", metrics.ContentType)
		w.Header().Set("Cache-Control", "no-store")
		if err := reg.Write(w); err != nil {
			log.Debug("writing the metrics", "error", err)
		}
	})
	return mux
}
