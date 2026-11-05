package migrations

import (
	"context"
	"fmt"

	entschema "entgo.io/ent/dialect/sql/schema"
)

// FileColumn - a column of a module's table referencing files.
type FileColumn struct{ Table, Column string }

// FileColumns - the columns of the set's tables referencing files (from
// the ent schema): the files only the module uses go with it.
func FileColumns(set string) []FileColumn {
	var out []FileColumn
	for _, s := range Sets() {
		if s.Name != set || set == CoreSet {
			continue
		}
		for _, t := range s.Tables {
			for _, fk := range t.ForeignKeys {
				if fk.RefTable != nil && fk.RefTable.Name == "files" && len(fk.Columns) == 1 {
					out = append(out, FileColumn{Table: t.Name, Column: fk.Columns[0].Name})
				}
			}
		}
	}
	return out
}

// Uninstall removes a module set: its tables (each
// dropped once no other table of the set references it) and its
// schema_migrations rows - installing it again starts from scratch. MySQL
// can't roll DDL back: a failed drop stops it, and running it again goes on
// (the tables are dropped IF EXISTS).
func (r Runner) Uninstall(ctx context.Context, set string) error {
	if set == CoreSet {
		return fmt.Errorf("the core can't be uninstalled")
	}
	var tables []*entschema.Table
	found := false
	for _, s := range Sets() {
		if s.Name == set {
			tables, found = s.Tables, true
		}
	}
	if !found {
		return fmt.Errorf("unknown set %q", set)
	}
	for _, t := range dropOrder(tables) {
		if _, err := r.DB.ExecContext(ctx, "DROP TABLE IF EXISTS "+t.Name); err != nil {
			return fmt.Errorf("%s: drop %s: %w", set, t.Name, err)
		}
	}
	_, err := r.DB.ExecContext(ctx, r.Dialect.Rebind(`DELETE FROM schema_migrations WHERE set_name = ?`), set)
	return err
}

// dropOrder - tables in an order that drops each after every table of
// the list referencing it (references to itself or outside the list
// don't count).
func dropOrder(tables []*entschema.Table) []*entschema.Table {
	left := map[string]*entschema.Table{}
	for _, t := range tables {
		left[t.Name] = t
	}
	var out []*entschema.Table
	for len(left) > 0 {
		progressed := false
		for _, t := range tables {
			if _, ok := left[t.Name]; !ok || referenced(t.Name, left) {
				continue
			}
			out = append(out, t)
			delete(left, t.Name)
			progressed = true
		}
		if !progressed {
			// A reference cycle: drop the rest in their order.
			for _, t := range tables {
				if _, ok := left[t.Name]; ok {
					out = append(out, t)
					delete(left, t.Name)
				}
			}
		}
	}
	return out
}

// referenced - whether a table of left other than name references name.
func referenced(name string, left map[string]*entschema.Table) bool {
	for other, t := range left {
		if other == name {
			continue
		}
		for _, fk := range t.ForeignKeys {
			if fk.RefTable != nil && fk.RefTable.Name == name {
				return true
			}
		}
	}
	return false
}
