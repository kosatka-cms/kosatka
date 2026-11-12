//go:build dbintegration

package app

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// The starter set's image styles the tests look for
// (starter/starter.yaml).
const (
	articleCardStyle = "article_card"
	articleWideStyle = "article_wide"
)

// syncBuffer - a log buffer the server's goroutines write while the test
// reads it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// serveSite - a new site served inside the test's process (unlike
// installSite, which runs the built binary): the schema and createSite as
// "app install" does, then the app's handler (buildHandler) on an httptest
// server. The handlers' code runs here, so it counts toward the coverage
// (go test -coverpkg). The app keeps its settings in package variables:
// one site at a time.
func serveSite(t *testing.T, db testdb.DB) *site {
	t.Helper()
	return serveSiteWith(t, db, false)
}

// serveSiteWith is serveSite, with the demo content ("install -demo") if
// demo.
func serveSiteWith(t *testing.T, db testdb.DB, demo bool) *site {
	t.Helper()
	opts := siteOptions{}
	if demo {
		opts.demoSet = defaultDemoSet
	}
	return serveSiteOpts(t, db, opts, "en")
}

// serveSiteOpts is serveSite in lang, installed with opts (a demo set, a
// theme).
func serveSiteOpts(t *testing.T, db testdb.DB, opts siteOptions, lang string) *site {
	t.Helper()
	t.Chdir(repoRoot(t)) // the templates, translations and static files

	ctx := context.Background()
	r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
	for _, set := range migrations.Sets() {
		if _, err := r.Install(ctx, set.Name); err != nil {
			t.Fatal(err)
		}
	}

	ts := httptest.NewUnstartedServer(nil)
	base := "http://" + ts.Listener.Addr().String()
	driver, tls := "mysql", ""
	if db.Driver == "pgx" {
		driver, tls = "postgres", "disable"
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, fmt.Appendf(nil, `env: local
public_base_url: %s
db:
  driver: %s
  host: %s
  port: %d
  scheme: %s
  username: %s
  password: %s
  tls: %q
session:
  secret_key: smoke-test-session-key-0123456789abcdef
files:
  public_dir: %s
  private_dir: %s
`, base, driver, db.Conn.Host, db.Conn.Port, db.Conn.Name, db.Conn.User, db.Conn.Password, tls, t.TempDir(), t.TempDir()), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.MustLoadPath(cfgPath)
	entity.SetSiteDomain(cfg.PublicBaseURL)
	paths.SetTheme(cfg.Theme)
	paths.SetPublicFiles(cfg.Files.PublicDir, cfg.Files.PublicURL)
	paths.SetPrivateFiles(cfg.Files.PrivateDir)

	conn, drv, err := NewEntConn(&cfg.DB)
	if err != nil {
		t.Fatal(err)
	}
	client := generated.NewClient(generated.Driver(drv))
	password, err := createSite(ctx, r, client, "Smoke Site", "admin", "admin@example.com", lang, opts)
	if err != nil {
		t.Fatal(err)
	}
	// The bot trap off: the tests post forms at once, without the time
	// they were shown (TestBotTrap turns it on).
	if err := client.CommentSettings.Create().SetID(1).SetBotTrap(false).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	logs := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	previous := slog.Default()
	slog.SetDefault(logger)
	// The site's background loops stop with the server, before the
	// database closes.
	jobsCtx, stopJobs := context.WithCancel(context.Background())
	var jobs backgroundJobs
	ts.Config.Handler, jobs = buildHandler(jobsCtx, http.NewServeMux(), drv, cfg, logger, nil, nil)
	ts.Start()

	s := &site{t: t, base: base, password: password, log: logs}
	// The database closes last, after the test has read the log: a cache
	// refresh a save started in the background (CacheService.ForceUpdate)
	// may still be running when the server stops.
	t.Cleanup(func() { _ = conn.Close() })
	var once sync.Once
	s.stop = func() {
		once.Do(func() {
			ts.Close()
			stopJobs()
			wait, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if !jobs.Wait(wait) {
				t.Error("the background jobs didn't stop")
			}
			slog.SetDefault(previous)
		})
	}
	t.Cleanup(s.stop)
	jar, _ := cookiejar.New(nil)
	s.client = &http.Client{Jar: jar, Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return s
}

// repoRoot - the repository's root (two levels above this file), whatever
// the working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}
