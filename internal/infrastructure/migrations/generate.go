package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"ariga.io/atlas/sql/migrate"
	atlasmysql "ariga.io/atlas/sql/mysql"
	atlaspostgres "ariga.io/atlas/sql/postgres"
	atlasschema "ariga.io/atlas/sql/schema"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	entschema "entgo.io/ent/dialect/sql/schema"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
)

// maxGeneratePasses - the most migrations one Generate run writes for a
// set before giving up (a diff that never settles).
const maxGeneratePasses = 5

// Generate writes, for each set, the migration file that brings the
// schema its existing files describe to the ent schema, into its
// directory under root, the project's (SourceDir) - nothing for a set
// already up to date. db is an
// empty development database of the dialect: every existing file is
// applied to it first, then each new file before the next set (a module's
// tables may reference the core's). It returns the sets that got a file.
// A table the set's files created that no ent schema of any set has any
// more is dropped (dropRemovedTables) - ent itself only diffs the tables
// it has.
func Generate(ctx context.Context, db *sql.DB, d sqldialect.Dialect, root, name string) ([]string, error) {
	r := Runner{DB: db, Dialect: d}
	if err := r.EnsureTable(ctx); err != nil {
		return nil, err
	}
	drv, err := atlasDriver(db, d)
	if err != nil {
		return nil, err
	}
	sets := Sets()
	// owned - the tables each set's files create: those that appear when
	// they are applied.
	owned := map[string][]string{}
	seen, err := tableNames(ctx, drv)
	if err != nil {
		return nil, err
	}
	entTables := map[string]bool{}
	for _, s := range sets {
		for _, t := range s.Tables {
			entTables[t.Name] = true
		}
		if err := applyDir(ctx, r, root, d, s.Name); err != nil {
			return nil, err
		}
		now, err := tableNames(ctx, drv)
		if err != nil {
			return nil, err
		}
		for t := range now {
			if !seen[t] {
				seen[t] = true
				owned[s.Name] = append(owned[s.Name], t)
			}
		}
	}
	entDialect := dialect.MySQL
	if d == sqldialect.Postgres {
		entDialect = dialect.Postgres
	}
	var changed []string
	for _, s := range sets {
		if len(s.Tables) == 0 || !InTree(root, s.Name) {
			continue
		}
		dir := SourceDir(root, d, s.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		local, err := migrate.NewLocalDir(dir)
		if err != nil {
			return nil, err
		}
		m, err := entschema.NewMigrate(entsql.OpenDB(entDialect, db),
			entschema.WithDir(local),
			entschema.WithMigrationMode(entschema.ModeInspect),
			entschema.WithDialect(entDialect),
			entschema.WithForeignKeys(true),
			// A column removed from the ent schema is dropped (its data was
			// moved first by a data step, or isn't needed).
			entschema.WithDropColumn(true),
			// An index removed from the ent schema (or replaced, e.g. a
			// unique key gaining a column) is dropped too.
			entschema.WithDropIndex(true),
			// One <version>_<name>.sql per migration (no up/down pairs:
			// MySQL can't roll DDL back anyway).
			entschema.WithFormatter(migrate.DefaultFormatter),
			entschema.WithDiffHook(sameDefaultHook),
		)
		if err != nil {
			return nil, err
		}
		// A module's tables reference core tables, so the core's go along:
		// they're applied to db already, so only the module's tables
		// differ.
		tables := s.Tables
		if s.Name != CoreSet {
			tables = append(append([]*entschema.Table{}, sets[0].Tables...), s.Tables...)
		}
		// Some changes take two passes (a table's collation, then its
		// columns'), so the set is diffed until it matches ent - else the
		// rest would land in the next set's file.
		for pass := 0; ; pass++ {
			if pass == maxGeneratePasses {
				return nil, fmt.Errorf("%s/%s: still differs from ent after %d migrations", d, s.Name, pass)
			}
			before, err := local.Files()
			if err != nil {
				return nil, err
			}
			if pass > 0 {
				time.Sleep(time.Second) // a file's version is the time in seconds
			}
			if err := m.NamedDiff(ctx, name, tables...); err != nil {
				return nil, fmt.Errorf("%s/%s: %w", d, s.Name, err)
			}
			after, err := local.Files()
			if err != nil {
				return nil, err
			}
			if len(after) == len(before) {
				// The tables match ent: then the tables ent no longer has.
				dropped, err := dropRemovedTables(ctx, drv, local, name, owned[s.Name], entTables)
				if err != nil {
					return nil, fmt.Errorf("%s/%s: %w", d, s.Name, err)
				}
				if !dropped {
					if len(before) == 0 {
						_ = os.Remove(dir) // nothing for this set yet
					}
					break
				}
			}
			if pass == 0 {
				changed = append(changed, s.Name)
			}
			if err := applyDir(ctx, r, root, d, s.Name); err != nil {
				return nil, err
			}
		}
	}
	return changed, nil
}

// atlasDriver - Atlas' driver of the dialect over db, for what ent's diff
// doesn't do (dropRemovedTables).
func atlasDriver(db *sql.DB, d sqldialect.Dialect) (migrate.Driver, error) {
	if d == sqldialect.Postgres {
		return atlaspostgres.Open(db)
	}
	return atlasmysql.Open(db)
}

// tableNames - the tables of the database's current schema.
func tableNames(ctx context.Context, drv migrate.Driver) (map[string]bool, error) {
	s, err := drv.InspectSchema(ctx, "", &atlasschema.InspectOptions{Mode: atlasschema.InspectTables})
	if err != nil {
		return nil, fmt.Errorf("listing the tables: %w", err)
	}
	names := make(map[string]bool, len(s.Tables))
	for _, t := range s.Tables {
		names[t.Name] = true
	}
	return names, nil
}

// dropRemovedTables writes the migration dropping the tables of owned
// (a set's, created by its files) that are still in the database but in
// no set's ent schema (entTables) - an ent schema removed from the code.
// Like a removed column, its data goes with it: move what is still needed
// in an earlier release. false when there is none.
func dropRemovedTables(ctx context.Context, drv migrate.Driver, dir migrate.Dir, name string, owned []string, entTables map[string]bool) (bool, error) {
	existing, err := tableNames(ctx, drv)
	if err != nil {
		return false, err
	}
	var removed []string
	for _, t := range owned {
		if existing[t] && !entTables[t] {
			removed = append(removed, t)
		}
	}
	if len(removed) == 0 {
		return false, nil
	}
	sort.Strings(removed)
	s, err := drv.InspectSchema(ctx, "", &atlasschema.InspectOptions{Tables: removed, Mode: atlasschema.InspectTables})
	if err != nil {
		return false, fmt.Errorf("inspecting the removed tables %v: %w", removed, err)
	}
	changes := make([]atlasschema.Change, 0, len(s.Tables))
	for _, t := range s.Tables {
		changes = append(changes, &atlasschema.DropTable{T: t})
	}
	// Unqualified, like ent's files (no schema name in the statements).
	plan, err := drv.PlanChanges(ctx, name, changes, func(o *migrate.PlanOptions) {
		noQualifier := ""
		o.SchemaQualifier = &noQualifier
	})
	if err != nil {
		return false, fmt.Errorf("planning the drop of %v: %w", removed, err)
	}
	if err := migrate.NewPlanner(drv, dir, migrate.PlanFormat(migrate.DefaultFormatter)).WritePlan(plan); err != nil {
		return false, fmt.Errorf("writing the drop of %v: %w", removed, err)
	}
	return true, nil
}

// InTree - whether set's migration files are in the source tree under
// root, so Generate writes them: a module's own schema's when the tree
// has its schema's directory (Dir's parent - the module's tree, or the
// engine's for its own modules), the engine's other sets in the engine's
// tree. A set of another tree is applied from the binary's files.
func InTree(root, set string) bool {
	dir := filepath.Join(root, "internal", "infrastructure", "migrations", "files")
	if s, ok := schemas[set]; ok {
		dir = filepath.Join(root, filepath.Dir(filepath.FromSlash(s.Dir)))
	}
	fi, err := os.Stat(dir)
	return err == nil && fi.IsDir()
}

// applyDir applies the pending files of a set: its directory's on disk
// in the source tree (InTree), else those the binary carries (Files).
func applyDir(ctx context.Context, r Runner, root string, d sqldialect.Dialect, set string) error {
	var files []File
	var err error
	if InTree(root, set) {
		files, err = readDir(os.DirFS(SourceDir(root, d, set)), ".")
	} else {
		files, err = Files(d, set)
	}
	if err != nil {
		return err
	}
	pending, err := r.Pending(ctx, set, files)
	if err != nil {
		return err
	}
	for _, f := range pending {
		if err := r.Apply(ctx, set, f); err != nil {
			return err
		}
	}
	return nil
}
