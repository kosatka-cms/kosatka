package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// tableInstalled - whether table exists: a core table always; a module's
// (entity.ModuleOfTable) once the module was installed, i.e. its migration
// set is recorded in schema_migrations. A module switched off before it was
// ever installed has no tables, so core code reading them skips them.
func tableInstalled(ctx context.Context, db *sql.DB, table string) (bool, error) {
	module := entity.ModuleOfTable(table)
	if module == "" {
		return true, nil
	}
	return moduleInstalled(ctx, db, module)
}

// moduleInstalled - whether the module was installed: its migration set
// is recorded in schema_migrations (its tables exist).
func moduleInstalled(ctx context.Context, db *sql.DB, module string) (bool, error) {
	var one int
	err := db.QueryRowContext(ctx, rebind(`SELECT 1 FROM schema_migrations WHERE set_name = ? LIMIT 1`), module).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
