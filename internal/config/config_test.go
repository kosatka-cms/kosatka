package config

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDBConfigDSN(t *testing.T) {
	c := DBConfig{Host: "db", Port: 3306, Username: "u", Password: "p", Scheme: "site", TLS: "skip-verify",
		SQLMode: "STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION"}
	dsn := c.DSN()
	if !strings.HasPrefix(dsn, "u:p@tcp(db:3306)/site?parseTime=true&charset=utf8mb4&clientFoundRows=true&tls=skip-verify&sql_mode=") {
		t.Fatalf("unexpected DSN: %s", dsn)
	}
	q, err := url.ParseQuery(dsn[strings.Index(dsn, "?")+1:])
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Get("sql_mode"); got != "'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION'" {
		t.Fatalf("sql_mode = %q, want a quoted SQL string", got)
	}

	for _, mode := range []string{"", SQLModeServerDefault} {
		c.SQLMode = mode
		if strings.Contains(c.DSN(), "sql_mode") {
			t.Fatalf("mode %q must not set a session sql_mode: %s", mode, c.DSN())
		}
	}
}

func TestDBConfigPostgresDSN(t *testing.T) {
	c := DBConfig{Driver: DriverPostgres, Host: "db", Port: 5432, Username: "u", Password: "p@ss", Scheme: "site", TLS: "require",
		SQLMode: "STRICT_TRANS_TABLES"}
	if c.SQLDriver() != "pgx" {
		t.Fatalf("driver = %q", c.SQLDriver())
	}
	if got, want := c.DSN(), "postgres://u:p%40ss@db:5432/site?sslmode=require"; got != want {
		t.Fatalf("DSN = %q, want %q", got, want)
	}
	c.TLS = ""
	if got := c.DSN(); strings.Contains(got, "sslmode") {
		t.Fatalf("no TLS must leave the driver's sslmode: %s", got)
	}
	if (DBConfig{}).SQLDriver() != "mysql" || (DBConfig{Driver: "mysql"}).IsPostgres() {
		t.Fatal("MySQL is the default")
	}
}

// TestEnv - env is prod unless the file or APP_ENV says local (any case,
// spaces around); any other value refuses the config: a typo would turn a
// live site's checks off.
func TestEnv(t *testing.T) {
	if old, ok := os.LookupEnv("APP_ENV"); ok {
		if err := os.Unsetenv("APP_ENV"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Setenv("APP_ENV", old) })
	}
	load := func(body string) (*Config, error) {
		t.Helper()
		p := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return LoadPath(p)
	}
	for body, local := range map[string]bool{"port: 8080\n": false, "env: prod\n": false, "env: \" Local \"\n": true} {
		cfg, err := load(body)
		if err != nil || cfg.IsLocal() != local || cfg.IsProd() == local {
			t.Errorf("%q: %+v %v, want local %v", body, cfg, err, local)
		}
	}
	for _, body := range []string{"env: production\n", "env: dev\n"} {
		if _, err := load(body); err == nil || !strings.Contains(err.Error(), "prod or local") {
			t.Errorf("%q: %v, want an error", body, err)
		}
	}
	t.Setenv("APP_ENV", "local")
	if cfg, err := load("env: prod\n"); err != nil || !cfg.IsLocal() {
		t.Errorf("APP_ENV over the file: %+v %v", cfg, err)
	}
}

func TestDBTLSDefault(t *testing.T) {
	for _, c := range []struct {
		driver, want string
	}{{DriverMySQL, "skip-verify"}, {DriverPostgres, "require"}} {
		cfg := Config{Env: "prod", DB: DBConfig{Driver: c.driver, Host: "db.example.com"}}
		applyDBTLSDefault(&cfg)
		if cfg.DB.TLS != c.want {
			t.Errorf("%s: TLS = %q, want %q", c.driver, cfg.DB.TLS, c.want)
		}
	}
}

// TestTimeoutDefaults - a config without "timeouts" gets the defaults the
// code used to hard-code; a set value replaces its default only.
func TestTimeoutDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte("port: 8080\ntimeouts:\n  database_query: 7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := MustLoadPath(p)
	got := cfg.Timeouts
	want := TimeoutConfig{ServerRead: 15, ServerWrite: 15, ServerIdle: 60, Shutdown: 30, DatabaseQuery: 7, CacheUpdate: 15, HTTPRequest: 30}
	if got != want {
		t.Errorf("timeouts %+v, want %+v", got, want)
	}
	if cfg.DB.ConnMaxLifetime != 5 || cfg.DB.ConnMaxIdleTime != 5 {
		t.Errorf("pool lifetimes %d %d", cfg.DB.ConnMaxLifetime, cfg.DB.ConnMaxIdleTime)
	}
	if Seconds(7) != 7*time.Second {
		t.Error("Seconds")
	}
}

// TestPortDefaults - without db.port the driver's usual port; without
// smtp_port the submission port 587; a set value stays.
func TestPortDefaults(t *testing.T) {
	for body, want := range map[string]int{
		"db:\n  driver: mysql\n":                  3306,
		"db:\n  driver: postgres\n":               5432,
		"db:\n  driver: postgres\n  port: 6432\n": 6432,
	} {
		p := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := MustLoadPath(p)
		if cfg.DB.Port != want || cfg.Email.SMTPPort != 587 {
			t.Errorf("%q: db port %d, smtp port %d; want %d, 587", body, cfg.DB.Port, cfg.Email.SMTPPort, want)
		}
	}
}

// TestTrustedProxies - a list in the file, or the variable's
// comma-separated one over it.
func TestTrustedProxies(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte("port: 8080\ntrusted_proxies: [10.0.0.0/8]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := MustLoadPath(p).TrustedProxies; len(got) != 1 || got[0] != "10.0.0.0/8" {
		t.Errorf("from the file: %q", got)
	}
	t.Setenv("TRUSTED_PROXIES", "172.16.0.0/12,192.0.2.10")
	if got := MustLoadPath(p).TrustedProxies; len(got) != 2 || got[1] != "192.0.2.10" {
		t.Errorf("from the variable: %q", got)
	}
}

// TestLoadPathMissing - a missing config is an error naming it, not a
// panic.
func TestLoadPathMissing(t *testing.T) {
	if _, err := LoadPath("/nonexistent/kosatka.yaml"); err == nil || !strings.Contains(err.Error(), "/nonexistent/kosatka.yaml") {
		t.Fatalf("got %v", err)
	}
}

// TestFilesDriverSections - the other keys of files are the storage
// drivers' sections: the engine's own keys stay out of them, and
// DecodeDriver reads one, the environment winning.
func TestFilesDriverSections(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	body := "files:\n  storage: blob\n  private_dir: /srv/private\n  blob:\n    bucket: site\n    path_style: true\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadPath(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Files.Storage != "blob" || cfg.Files.PrivateDir != "/srv/private" || len(cfg.Files.Drivers) != 1 || cfg.Files.Drivers["blob"] == nil {
		t.Fatalf("files = %+v", cfg.Files)
	}
	type blob struct {
		Bucket    string `yaml:"bucket" env:"KOSATKA_TEST_BLOB_BUCKET"`
		PathStyle bool   `yaml:"path_style"`
		Key       string `yaml:"-" env:"KOSATKA_TEST_BLOB_KEY"`
	}
	t.Setenv("KOSATKA_TEST_BLOB_KEY", "secret")
	var b blob
	if err := cfg.Files.DecodeDriver("blob", &b); err != nil || b != (blob{Bucket: "site", PathStyle: true, Key: "secret"}) {
		t.Errorf("blob = %+v %v", b, err)
	}
	t.Setenv("KOSATKA_TEST_BLOB_BUCKET", "other")
	b = blob{}
	if err := cfg.Files.DecodeDriver("blob", &b); err != nil || b.Bucket != "other" {
		t.Errorf("the environment doesn't win: %+v %v", b, err)
	}
	b = blob{}
	if err := cfg.Files.DecodeDriver("missing", &b); err != nil || b != (blob{Bucket: "other", Key: "secret"}) {
		t.Errorf("no section: %+v %v", b, err)
	}
}
