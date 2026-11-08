package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// The core's data steps.
func init() {
	RegisterUpdate(CoreSet, Update{Version: "20261106173400", Description: "mail_resend_key", Run: moveResendKey})
	RegisterUpdate(CoreSet, Update{Version: "20261108174000", Description: "last_comments_title", Run: showLastCommentsTitle})
}

// showLastCommentsTitle - the latest comments block (plugin
// "last_comments") had a heading of its own, "Recent comments", whatever
// its placement's "Show title"; now its heading is the placement's, as
// every system block's - the title, or else the block's name. Its
// placements without "Show title" get it, so a site keeps a heading over
// the block: the block's name, or the title given to the placement. One
// already showing its title shows it once now instead of twice.
func showLastCommentsTitle(ctx context.Context, env Env) error {
	if _, err := env.DB.ExecContext(ctx, env.Dialect.Rebind("UPDATE block_placements SET show_title = ? WHERE placement_type = ? AND plugin_id = ? AND show_title = ?"),
		true, "plugin", "last_comments", false); err != nil {
		return fmt.Errorf("showing the latest comments block's title: %w", err)
	}
	return nil
}

// moveResendKey - Resend left the engine for a module of its own
// (kosatka-module-resend, its transport "resend" with the setting
// "api_key"): the key saved in the admin is copied from
// mail_settings.resend_api_key to that setting in mail_transport_settings
// as it is (sealed with the site's key or not, both columns are). The old
// column stays as it was - the older binary, still running while the
// migrations apply before the restart, sends with it, and so does one
// rolled back to - and is dropped in the next minor release. The saved
// transport stays "resend": the site sends through the module once its
// binary has it.
func moveResendKey(ctx context.Context, env Env) error {
	tx, err := env.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("moving the Resend key: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var key string
	err = tx.QueryRowContext(ctx, env.Dialect.Rebind("SELECT resend_api_key FROM mail_settings WHERE id = ?"), 1).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && key == "") {
		return nil
	}
	if err != nil {
		return fmt.Errorf("moving the Resend key: %w", err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, env.Dialect.Rebind("SELECT COUNT(*) FROM mail_transport_settings WHERE transport = ? AND name = ?"), "resend", "api_key").Scan(&n); err != nil {
		return fmt.Errorf("moving the Resend key: %w", err)
	}
	if n > 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, env.Dialect.Rebind("INSERT INTO mail_transport_settings (transport, name, value) VALUES (?, ?, ?)"), "resend", "api_key", key); err != nil {
		return fmt.Errorf("moving the Resend key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("moving the Resend key: %w", err)
	}
	return nil
}
