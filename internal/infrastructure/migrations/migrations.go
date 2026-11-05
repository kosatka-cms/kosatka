// Package migrations versions the database schema, generated: the ent
// schemas describe the tables, and
// for every change scripts/migrations_gen.sh writes a migration file per
// database (MySQL, PostgreSQL) into files/<dialect>/<set>/ (embedded in
// the binary) - or, for a module with its own ent schema (RegisterSchema),
// into that module's migration files. A set is the core or one module
// (entity.Module.Tables):
// each has its own sequence of files, so a module's tables are created
// only when the module is installed. Applied versions are recorded in
// schema_migrations; "app migrate" applies the pending ones.
package migrations

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"ariga.io/atlas/sql/migrate"
	entschema "entgo.io/ent/dialect/sql/schema"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	entmigrate "github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/migrate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
)

// CoreSet - the set of every table no module owns.
const CoreSet = "core"

//go:embed all:files
var embedded embed.FS

// Set - the core or a module, with the ent tables it owns.
type Set struct {
	Name   string
	Tables []*entschema.Table
}

// Schema - a module's own ent schema, apart from the core's
// (internal/modules/<name>/ent): its tables (its generated migrate.Tables)
// and migration files (<dialect>/*.sql with atlas.sum, embedded by the
// module); Dir - the files' directory from the project's root, where
// scripts/migrations_gen.sh writes the module's next migration.
type Schema struct {
	Tables []*entschema.Table
	Files  fs.FS
	Dir    string
}

// schemas - the modules' own schemas, by set.
var schemas = map[string]Schema{}

// RegisterSchema sets a module's own schema (its manifest's RegisterSteps,
// at startup): its set's tables and files are those, not the core
// schema's.
func RegisterSchema(set string, s Schema) {
	schemas[set] = s
}

// SourceDir - where a set's migration files for d are in the source tree
// under root (the project's): a module's own schema's directory, else
// files/<dialect>/<set> of this package.
func SourceDir(root string, d sqldialect.Dialect, set string) string {
	if s, ok := schemas[set]; ok {
		return filepath.Join(root, filepath.FromSlash(s.Dir), string(d))
	}
	return filepath.Join(root, "internal", "infrastructure", "migrations", "files", string(d), set)
}

// SourceDirs - every set's directory of migration files for every
// dialect, under root (SourceDir).
func SourceDirs(root string) []string {
	var out []string
	for _, s := range Sets() {
		for _, d := range []sqldialect.Dialect{sqldialect.MySQL, sqldialect.Postgres} {
			out = append(out, SourceDir(root, d, s.Name))
		}
	}
	return out
}

// Sets - the core first, then the modules in entity.Modules order (a
// module's tables may reference core tables, so the core goes first). A
// module's tables are its own schema's (RegisterSchema), else its
// entity.Module.Tables in the core schema.
func Sets() []Set {
	byName := map[string]*entschema.Table{}
	for _, t := range entmigrate.Tables {
		byName[t.Name] = t
	}
	owned := map[string]bool{}
	var sets []Set
	for _, m := range entity.Modules {
		s := Set{Name: m.Name}
		if own, ok := schemas[m.Name]; ok {
			s.Tables = ownTables(own.Tables, byName)
			sets = append(sets, s)
			continue
		}
		for _, name := range m.Tables {
			if t, ok := byName[name]; ok {
				s.Tables = append(s.Tables, t)
				owned[name] = true
			}
		}
		sets = append(sets, s)
	}
	core := Set{Name: CoreSet}
	for _, t := range entmigrate.Tables {
		if !owned[t.Name] {
			core.Tables = append(core.Tables, t)
		}
	}
	return append([]Set{core}, sets...)
}

// ownTables - the tables of a module's own schema without its references
// to core tables: a table of the schema named as one of the core schema
// is the module's stand-in for it (its id only, so the module's edges
// give its foreign keys - a module's schema has no core types), not a
// table of its own. Its foreign keys to a stand-in are pointed at the
// core's table, the one its migrations reference.
func ownTables(tables []*entschema.Table, core map[string]*entschema.Table) []*entschema.Table {
	out := make([]*entschema.Table, 0, len(tables))
	for _, t := range tables {
		if core[t.Name] != nil {
			continue
		}
		for _, fk := range t.ForeignKeys {
			ref := core[fk.RefTable.Name]
			if ref == nil || ref == fk.RefTable {
				continue
			}
			cols := make([]*entschema.Column, len(fk.RefColumns))
			for i, c := range fk.RefColumns {
				cols[i] = c
				if rc, ok := ref.Column(c.Name); ok {
					cols[i] = rc
				}
			}
			fk.RefTable, fk.RefColumns = ref, cols
		}
		out = append(out, t)
	}
	return out
}

// Dir - the set's directory of migration files for the dialect, relative
// to this package (files/<dialect>/<set>).
func Dir(d sqldialect.Dialect, set string) string {
	return path.Join("files", string(d), set)
}

// File - one migration file.
type File struct {
	Version     string
	Description string
	Stmts       []string
}

// Files - the set's migration files for the dialect, in order. The
// directory's atlas.sum must match its files (no file edited, added or
// removed by hand).
func Files(d sqldialect.Dialect, set string) ([]File, error) {
	if s, ok := schemas[set]; ok {
		return readDir(s.Files, string(d))
	}
	return readDir(embedded, Dir(d, set))
}

func readDir(fsys fs.FS, dir string) ([]File, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var local []migrate.File
	var out []File
	for _, n := range names {
		b, err := fs.ReadFile(fsys, path.Join(dir, n))
		if err != nil {
			return nil, err
		}
		lf := migrate.NewLocalFile(n, b)
		stmts, err := lf.Stmts()
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", dir, n, err)
		}
		local = append(local, lf)
		out = append(out, File{Version: lf.Version(), Description: lf.Desc(), Stmts: stmts})
	}
	if len(local) == 0 {
		return nil, nil
	}
	sum, err := fs.ReadFile(fsys, path.Join(dir, migrate.HashFileName))
	if err != nil {
		return nil, fmt.Errorf("%s: no %s: %w", dir, migrate.HashFileName, err)
	}
	hf, err := migrate.NewHashFile(local)
	if err != nil {
		return nil, err
	}
	want, err := hf.MarshalText()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(bytes.TrimSpace(sum), bytes.TrimSpace(want)) {
		return nil, fmt.Errorf("%s: %s doesn't match the files - migrations are generated (scripts/migrations_gen.sh), never edited by hand", dir, migrate.HashFileName)
	}
	return out, nil
}

// Runner applies migration files to a database and records them in
// schema_migrations.
type Runner struct {
	DB      *sql.DB
	Dialect sqldialect.Dialect
}

// EnsureTable creates schema_migrations (same DDL on both databases).
func (r Runner) EnsureTable(ctx context.Context) error {
	_, err := r.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
	set_name VARCHAR(64) NOT NULL,
	version VARCHAR(64) NOT NULL,
	description VARCHAR(255) NOT NULL DEFAULT '',
	applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (set_name, version)
)`)
	return err
}

// eachRow runs query and calls scan on every row; the rows are closed
// and their iteration error checked.
func eachRow(ctx context.Context, env Env, query string, args []any, scan func(*sql.Rows) error) error {
	rows, err := env.DB.QueryContext(ctx, env.Dialect.Rebind(query), args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Applied - the set's applied versions.
func (r Runner) Applied(ctx context.Context, set string) (map[string]bool, error) {
	rows, err := r.DB.QueryContext(ctx, r.Dialect.Rebind(`SELECT version FROM schema_migrations WHERE set_name = ?`), set)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

// SetNames - the sets with something applied, sorted (read only: the
// table must exist).
func (r Runner) SetNames(ctx context.Context) ([]string, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT DISTINCT set_name FROM schema_migrations ORDER BY set_name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Pending - the set's files not applied yet, in order.
func (r Runner) Pending(ctx context.Context, set string, files []File) ([]File, error) {
	applied, err := r.Applied(ctx, set)
	if err != nil {
		return nil, err
	}
	var out []File
	for _, f := range files {
		if !applied[f.Version] {
			out = append(out, f)
		}
	}
	return out, nil
}

// Apply runs a file's statements and records it. A failed statement
// stops it: MySQL can't roll back DDL, so the error names the statement
// to look at before running again.
func (r Runner) Apply(ctx context.Context, set string, f File) error {
	for i, stmt := range f.Stmts {
		if _, err := r.DB.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%s %s_%s, statement %d of %d: %w\n%s", set, f.Version, f.Description, i+1, len(f.Stmts), err, stmt)
		}
	}
	return r.record(ctx, set, f)
}

// MarkApplied records a file without running it (the baseline of a
// database whose schema already matches it).
func (r Runner) MarkApplied(ctx context.Context, set string, f File) error {
	return r.record(ctx, set, f)
}

func (r Runner) record(ctx context.Context, set string, f File) error {
	_, err := r.DB.ExecContext(ctx, r.Dialect.Rebind(`INSERT INTO schema_migrations (set_name, version, description) VALUES (?, ?, ?)`), set, f.Version, f.Description)
	return err
}
