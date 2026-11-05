package languages

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// Repo - what the Languages page reads of the site's content (the core's
// nodes, counted).
// A built-in module, tested with the engine, may read the core's tables
// in its SQL; a module of its own repository goes through the SDK's
// services instead (docs/modules/sdk.md): the core's columns aren't its
// API.
type Repo struct {
	db *sql.DB
}

// NewRepo creates a Repo on the site's database.
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

// NodeCounts - the number of nodes (the trash's too) of each language,
// by code.
func (r *Repo) NodeCounts(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT langcode, COUNT(*) FROM nodes GROUP BY langcode`)
	if err != nil {
		return nil, fmt.Errorf("counting nodes by language: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int{}
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, fmt.Errorf("counting nodes by language: %w", err)
		}
		out[code] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("counting nodes by language: %w", err)
	}
	return out, nil
}

// SlugsUnder - whether a node's address starts with prefix/ (a prefix
// would hide it).
func (r *Repo) SlugsUnder(ctx context.Context, prefix string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, modulekit.Rebind(`SELECT COUNT(*) FROM nodes WHERE slug LIKE ?`), prefix+"/%").Scan(&n)
	if err != nil {
		return false, fmt.Errorf("looking for nodes under %q: %w", prefix, err)
	}
	return n > 0, nil
}
