package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/lib/secretbox"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The secrets the site keeps in its database - the outside services' keys
// and passwords saved in the admin, the users' two-factor secrets - are
// sealed with the site's key (SECRETS_KEY, internal/lib/secretbox): written sealed,
// read opened. Without a key they are kept as they are.

// secrets - the site's key (SetSecrets); nil - none.
var secrets *secretbox.Box

// SetSecrets sets the key the secrets are sealed with (at startup).
func SetSecrets(b *secretbox.Box) { secrets = b }

// sealSecret - s as stored: sealed with the site's key.
func sealSecret(s string) (string, error) {
	sealed, err := secrets.Seal(s)
	if err != nil {
		return "", fmt.Errorf("sealing a secret: %w", err)
	}
	return sealed, nil
}

// openSecret - the secret stored as s; one the key doesn't open reads as
// none (the status report names them: UnreadableSecrets), so it is
// entered again.
func openSecret(s string) string {
	plain, _, err := secrets.Open(s)
	if err != nil {
		return ""
	}
	return plain
}

// SecretsSealed - whether the site has a key to seal the secrets with.
func SecretsSealed() bool { return secrets.Enabled() }

// SecretsKeyWeak - whether the site's key is guessable (secretbox.Box.Weak).
func SecretsKeyWeak() bool { return secrets.Weak() }

// secretColumn - a column holding secrets: its table, the table's key
// column and what the status report calls them (a catalog string).
type secretColumn struct {
	table, key, column, label string
}

// secretColumns - every column sealed with the site's key.
var secretColumns = []secretColumn{
	{"mail_transport_settings", "id", "value", i18n.N("Mail: the sending services' settings")},
	{"mail_settings", "id", "smtp_password", i18n.N("Mail: SMTP password")},
	{"captcha_settings", "id", "secret_key", i18n.N("CAPTCHA: secret key")},
	{"oauth_settings", "provider", "client_secret", i18n.N("Social sign-in: client secrets")},
	{"user_two_factor", "id", "secret", i18n.N("Two-factor sign-in of users")},
}

// secretRow - a stored secret: its row's key and its value.
type secretRow struct{ key, value string }

// readSecrets - the column's stored secrets.
func readSecrets(ctx context.Context, db *sql.DB, c secretColumn) ([]secretRow, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT %s, %s FROM %s", c.key, c.column, c.table))
	if err != nil {
		return nil, fmt.Errorf("reading %s.%s: %w", c.table, c.column, err)
	}
	defer func() { _ = rows.Close() }()
	var out []secretRow
	for rows.Next() {
		var r secretRow
		if err := rows.Scan(&r.key, &r.value); err != nil {
			return nil, fmt.Errorf("reading %s.%s: %w", c.table, c.column, err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading %s.%s: %w", c.table, c.column, err)
	}
	return out, nil
}

// unopenable - whether err is a sealed secret none of the keys opens.
func unopenable(err error) bool {
	return errors.Is(err, secretbox.ErrNoKey) || errors.Is(err, secretbox.ErrWrongKey)
}

// SealSecrets seals again what isn't sealed with the site's first key: the
// plain secrets (saved before the site had a key) and the ones sealed with
// an older key (SECRETS_KEY "new,old"). It returns how many it sealed and,
// per column (secretColumn.label), how many none of the keys opens.
func SealSecrets(ctx context.Context, db *sql.DB) (int, map[string]int, error) {
	sealed, unreadable := 0, map[string]int{}
	for _, c := range secretColumns {
		rows, err := readSecrets(ctx, db, c)
		if err != nil {
			return sealed, unreadable, err
		}
		for _, r := range rows {
			plain, current, err := secrets.Open(r.value)
			switch {
			case unopenable(err):
				unreadable[c.label]++
				continue
			case err != nil:
				return sealed, unreadable, fmt.Errorf("%s.%s: %w", c.table, c.column, err)
			case current:
				continue
			}
			value, err := sealSecret(plain)
			if err != nil {
				return sealed, unreadable, err
			}
			q := rebind(fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", c.table, c.column, c.key))
			if _, err := db.ExecContext(ctx, q, value, r.key); err != nil {
				return sealed, unreadable, fmt.Errorf("sealing %s.%s: %w", c.table, c.column, err)
			}
			sealed++
		}
	}
	return sealed, unreadable, nil
}

// UnreadableSecrets - per column (secretColumn.label), how many stored
// secrets none of the site's keys opens: sealed with a key the site no
// longer has.
func UnreadableSecrets(ctx context.Context, db *sql.DB) (map[string]int, error) {
	out := map[string]int{}
	for _, c := range secretColumns {
		rows, err := readSecrets(ctx, db, c)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if _, _, err := secrets.Open(r.value); unopenable(err) {
				out[c.label]++
			}
		}
	}
	return out, nil
}
