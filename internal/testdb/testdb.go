//go:build dbintegration

// Package testdb gives integration tests a fresh, empty database on each
// server they run against: MySQL and PostgreSQL, from
// KOSATKA_TEST_MYSQL_DSN and KOSATKA_TEST_POSTGRES_DSN (a DSN with rights
// to create databases; scripts/test_db.sh and the CI start both). A server
// without its DSN is skipped.
package testdb

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
)

// DB - a fresh database on one server.
type DB struct {
	Dialect sqldialect.Dialect
	Driver  string
	DB      *sql.DB
	// Conn - how to reach the database, for a test that runs the app
	// against it (its db config section).
	Conn Conn
}

// Conn - a database's address and credentials.
type Conn struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

// Each runs fn on a fresh database of every configured server; the
// database is dropped afterwards.
func Each(t *testing.T, fn func(t *testing.T, db DB)) {
	t.Helper()
	ran := false
	if dsn := os.Getenv("KOSATKA_TEST_MYSQL_DSN"); dsn != "" {
		ran = true
		t.Run("mysql", func(t *testing.T) { fn(t, freshMySQL(t, dsn)) })
	}
	if dsn := os.Getenv("KOSATKA_TEST_POSTGRES_DSN"); dsn != "" {
		ran = true
		t.Run("postgres", func(t *testing.T) { fn(t, freshPostgres(t, dsn)) })
	}
	if !ran {
		t.Skip("no KOSATKA_TEST_MYSQL_DSN / KOSATKA_TEST_POSTGRES_DSN")
	}
}

// dbSeq numbers the databases a test binary makes: with its pid, a
// name no other package's binary, running at the same moment, takes.
var dbSeq atomic.Int64

func dbName() string {
	return fmt.Sprintf("kosatka_test_%d_%d_%d", time.Now().UnixNano(), os.Getpid(), dbSeq.Add(1))
}

func freshMySQL(t *testing.T, dsn string) DB {
	t.Helper()
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	name := dbName()
	if _, err := admin.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE " + name) })
	cfg.DBName = name
	cfg.ParseTime = true
	cfg.ClientFoundRows = true // as config.DBConfig.DSN
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	host, port := splitHostPort(t, cfg.Addr)
	return DB{Dialect: sqldialect.MySQL, Driver: "mysql", DB: db,
		Conn: Conn{Host: host, Port: port, User: cfg.User, Password: cfg.Passwd, Name: name}}
}

func freshPostgres(t *testing.T, dsn string) DB {
	t.Helper()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	name := dbName()
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE " + name + " WITH (FORCE)") })
	db, err := sql.Open("pgx", withDatabase(dsn, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	host, port := splitHostPort(t, u.Host)
	password, _ := u.User.Password()
	return DB{Dialect: sqldialect.Postgres, Driver: "pgx", DB: db,
		Conn: Conn{Host: host, Port: port, User: u.User.Username(), Password: password, Name: name}}
}

// splitHostPort - host and port of a "host:port" address.
func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

// withDatabase - a postgres:// URL pointing at another database.
func withDatabase(dsn, name string) string {
	i := strings.LastIndex(dsn, "/")
	if i < 0 {
		return dsn
	}
	rest := ""
	if q := strings.Index(dsn[i:], "?"); q >= 0 {
		rest = dsn[i+q:]
	}
	return dsn[:i+1] + name + rest
}
