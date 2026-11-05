// Package migrationsgen writes the next schema migration of every set
// whose ent tables changed, for MySQL and PostgreSQL (scripts/
// migrations_gen.sh starts the databases it needs). With -check it writes
// nothing and fails if a migration is missing - the files are behind the
// ent schemas. The engine's cmd/migrations-gen runs it; so does a module
// from its own repository (kosatka.GenerateMigrations), for its own
// schema, from its tree.
//
//	go run ./cmd/migrations-gen -name add_feed_language
//	go run ./cmd/migrations-gen -check
package migrationsgen

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	// The PostgreSQL driver (sql.Open("pgx", …)).
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
)

// Main runs the generator on the command line's flags (-name, -check):
// the sets are the binary's - the engine's and the modules it imports.
func Main() {
	// The modules' tables make their own sets (migrations.Sets).
	manifests.Register()
	name := flag.String("name", "", "migration name (snake_case), e.g. add_feed_language")
	check := flag.Bool("check", false, "fail if the files are behind the ent schemas; write nothing")
	flag.Parse()
	if !*check && !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(*name) {
		fail("-name is required: lowercase letters, digits, underscores")
	}
	ctx := context.Background()
	// The project's root: each set's files are under it (the core's and
	// the modules' in this package, a module's own schema's in the
	// module - migrations.SourceDir); -check works on a copy of them.
	dir := "."
	if *check {
		tmp, err := os.MkdirTemp("", "migrations-check")
		if err != nil {
			fail(err.Error())
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		for _, src := range migrations.SourceDirs(".") {
			// The set's place in the tree (migrations.InTree), its files
			// or not yet.
			if parent := filepath.Dir(filepath.Dir(src)); dirExists(parent) {
				if err := os.MkdirAll(filepath.Join(tmp, parent), 0o755); err != nil {
					fail(err.Error())
				}
			}
			if !dirExists(src) {
				continue
			}
			if err := copyTree(src, filepath.Join(tmp, src)); err != nil {
				fail(err.Error())
			}
		}
		dir, *name = tmp, "check"
	}
	behind := false
	for _, target := range []struct {
		d   sqldialect.Dialect
		env string
	}{{sqldialect.MySQL, "KOSATKA_TEST_MYSQL_DSN"}, {sqldialect.Postgres, "KOSATKA_TEST_POSTGRES_DSN"}} {
		dsn := os.Getenv(target.env)
		if dsn == "" {
			fail(target.env + " is not set: the test database server's address (the engine's scripts/migrations_gen.sh sets it)")
		}
		db, drop := freshDatabase(target.d, dsn)
		changed, err := migrations.Generate(ctx, db, target.d, dir, *name)
		drop()
		if err != nil {
			fail(fmt.Sprintf("%s: %v", target.d, err))
		}
		for _, s := range changed {
			if *check {
				behind = true
				fmt.Printf("%s/%s: the ent schema has changes without a migration:\n%s\n", target.d, s, newest(migrations.SourceDir(dir, target.d, s)))
			} else {
				fmt.Printf("%s/%s: new migration %q\n", target.d, s, *name)
			}
		}
	}
	if behind {
		fail("write the migration with -name <name> (the engine's scripts/migrations_gen.sh <name>) and commit the files")
	}
}

// freshDatabase creates an empty database on the server and returns it
// with a function dropping it.
func freshDatabase(d sqldialect.Dialect, dsn string) (*sql.DB, func()) {
	name := fmt.Sprintf("kosatka_gen_%d", time.Now().UnixNano())
	if d == sqldialect.MySQL {
		cfg, err := mysql.ParseDSN(dsn)
		if err != nil {
			fail(err.Error())
		}
		admin, err := sql.Open("mysql", cfg.FormatDSN())
		must(err)
		_, err = admin.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
		must(err)
		cfg.DBName = name
		cfg.ParseTime = true
		db, err := sql.Open("mysql", cfg.FormatDSN())
		must(err)
		return db, func() { _ = db.Close(); _, _ = admin.Exec("DROP DATABASE " + name); _ = admin.Close() }
	}
	admin, err := sql.Open("pgx", dsn)
	must(err)
	_, err = admin.Exec("CREATE DATABASE " + name)
	must(err)
	i := strings.LastIndex(dsn, "/")
	rest := ""
	if q := strings.Index(dsn[i:], "?"); q >= 0 {
		rest = dsn[i+q:]
	}
	db, err := sql.Open("pgx", dsn[:i+1]+name+rest)
	must(err)
	return db, func() {
		_ = db.Close()
		_, _ = admin.Exec("DROP DATABASE " + name + " WITH (FORCE)")
		_ = admin.Close()
	}
}

// newest - the content of the directory's last migration file.
func newest(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err.Error()
	}
	last := ""
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") && e.Name() > last {
			last = e.Name()
		}
	}
	b, _ := os.ReadFile(filepath.Join(dir, last))
	return string(b)
}

// dirExists - whether p is a directory.
func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

func must(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "migrations-gen:", msg)
	os.Exit(1)
}
