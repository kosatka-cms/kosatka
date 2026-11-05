package app

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/lib/secretbox"
)

// setupSecrets - the key the secrets in the database are sealed with
// (secrets_key / SECRETS_KEY), set for the repositories; the secrets not
// sealed with it yet are sealed now (saved before the site had a key, or
// with the key being replaced). A key too short stops the start: it would
// seal in name only. No key - the secrets stay as they are, the status
// report warns.
func setupSecrets(ctx context.Context, cfg *config.Config, db *sql.DB, log *slog.Logger) {
	box, err := secretbox.New(cfg.SecretsKey)
	if err != nil {
		log.Error("secrets_key (SECRETS_KEY) is not usable", "error", err)
		panic("secrets_key (SECRETS_KEY): " + err.Error())
	}
	database.SetSecrets(box)
	level := slog.LevelInfo
	if cfg.IsProd() {
		level = slog.LevelWarn
	}
	switch {
	case !box.Enabled():
		log.Log(ctx, level, "secrets_key (SECRETS_KEY) is not set: the keys saved in the admin and the two-factor secrets are kept in the database unsealed")
	case box.Weak():
		log.Log(ctx, level, "secrets_key (SECRETS_KEY) is easy to guess (an example, or too few different characters): replace it with a long random string")
	}
	sealed, unreadable, err := database.SealSecrets(ctx, db)
	if err != nil {
		// Before the migrations widening the columns, say: the next start
		// seals the rest.
		log.Error("error sealing the secrets in the database", "error", err)
	}
	if sealed > 0 {
		log.Info("secrets sealed with the site's key", "count", sealed)
	}
	for where, n := range unreadable {
		log.Warn("secrets the site's key doesn't open: enter them again (or put the old key after the new one)", "where", where, "count", n)
	}
}
