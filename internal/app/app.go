// Package app - the engine's program: the server and its commands
// (install, migrate, theme…), run by Main.
package app

import (
	"context"
	"crypto/rand"
	coreSQL "database/sql"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"entgo.io/ent/dialect/sql"
	// The database drivers db.driver chooses from.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/lib/slogquiet"
	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// loadConfig - the site's config (config.Load) with its engine directory
// applied: the commands read the engine's files (themes, translations,
// templates) from where the server does. Without one the process exits
// with the reason.
func loadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "app:", err)
		os.Exit(1)
	}
	if cfg.EngineDir != "" {
		paths.SetEngineDir(cfg.EngineDir)
	}
	return cfg
}

// Main runs the engine: "app <command> …" runs a command and exits;
// without one it serves the site. The engine's binary (cmd/app) calls it,
// and so does a site's own main package that imports modules from
// outside the engine (kosatka.Main).
func Main() {
	// The modules the binary carries, before anything reads the registry
	// (the migrate and install commands too).
	manifests.Register()
	// The engine's own files, for the commands too (they read no config
	// before running); the config's engine_dir below, when it sets one.
	paths.SetEngineDir(os.Getenv("ENGINE_DIR"))
	// "app <command> …" runs a command and exits; without one the app
	// serves the site.
	if code, ok := dispatch(os.Args[1:], os.Stderr); ok {
		os.Exit(code)
	}
	cfg := loadConfig()
	entity.SetSiteDomain(cfg.PublicBaseURL)
	paths.SetTheme(cfg.Theme)
	if err := setupFiles(cfg.Files); err != nil {
		panic(err.Error())
	}
	proxies, err := httputil.ParseTrustedProxies(cfg.TrustedProxies)
	if err != nil {
		panic("trusted_proxies (TRUSTED_PROXIES): " + err.Error())
	}
	httputil.SetTrustedProxies(proxies)

	log := setupLogger(cfg.IsLocal())
	slog.SetDefault(log)
	log.Info("Starting server...")

	log.Info("Connecting DB...")
	dbConn, drv, err := NewEntConn(&cfg.DB)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		panic(err)
	}
	log.Info("DB connected")

	if err = drv.DB().Ping(); err != nil {
		log.Error("failed to ping database", "error", err)

		panic(err)
	}

	log.Info("DB pinged")

	setupSecrets(context.Background(), cfg, drv.DB(), log)

	mux := http.NewServeMux()

	// The process's life: the background loops (the cache's refresh, the
	// scheduler) run until it ends.
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
	handler, jobs := buildHandler(ctx, mux, drv, cfg, log)

	serverOne := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: handler,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
		ReadTimeout:  config.Seconds(cfg.Timeouts.ServerRead),
		WriteTimeout: config.Seconds(cfg.Timeouts.ServerWrite),
		IdleTimeout:  config.Seconds(cfg.Timeouts.ServerIdle),
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	serveErr := make(chan error, 1)
	go func() {
		log.Info(fmt.Sprintf("Listening on port: %d", cfg.Port))
		serveErr <- serverOne.ListenAndServe()
	}()

	// A signal, or the server failing (its port taken, say): a process
	// not serving exits, its service manager sees it.
	exitCode := 0
	select {
	case <-sigChan:
		log.Info("Shutting down server...")
	case err := <-serveErr:
		log.Error("error listening for server", "error", err)
		exitCode = 1
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.Seconds(cfg.Timeouts.Shutdown))
	defer shutdownCancel()
	if err := serverOne.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", "error", err)
		exitCode = 1
	}
	// The background loops end before the database closes.
	cancelCtx()
	if !jobs.Wait(shutdownCtx) {
		log.Warn("the background jobs didn't end in time")
	}
	if err := dbConn.Close(); err != nil {
		log.Error("error closing database connection", "error", err)
	} else {
		log.Info("Database connection closed")
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	log.Info("Server stopped gracefully")
}

// NewEntConn opens the site database (MySQL or PostgreSQL, cfg.Driver);
// the repositories built from its *sql.DB speak its dialect.
func NewEntConn(cfg *config.DBConfig) (*coreSQL.DB, *sql.Driver, error) {
	conn, err := coreSQL.Open(cfg.SQLDriver(), cfg.DSN())
	if err != nil {
		return nil, nil, err
	}

	database.SetDialect(cfg.Dialect())
	driver := sql.OpenDB(cfg.Dialect(), conn)
	db := driver.DB()

	maxOpenConns := cfg.MaxOpenConns
	if maxOpenConns == 0 {
		maxOpenConns = 25 // default
	}
	maxIdleConns := cfg.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 5 // default
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Minute) // Recycle connections before the server closes them
	db.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Minute) // Close idle connections

	return db, driver, nil
}

// setupLogger - the service's log: text lines on standard output
// (level=INFO …, the production guide greps them), DEBUG and up on a
// development machine (local, env local), INFO and up on a live site.
func setupLogger(local bool) *slog.Logger {
	level := slog.LevelInfo
	if local {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	// Client disconnects (context canceled, broken pipe) are not server
	// errors: logged at DEBUG.
	return slog.New(slogquiet.New(log.Handler()))
}

// generateSessionSecret generates a random 32-byte secret key for session encryption.
func generateSessionSecret() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic("failed to generate session secret: " + err.Error())
	}
	return base64.URLEncoding.EncodeToString(bytes)
}
