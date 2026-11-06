//go:build dbintegration

package database_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/lib/secretbox"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// useSecretsKey sets the site's key for the test (none after it).
func useSecretsKey(t *testing.T, keys string) {
	t.Helper()
	box, err := secretbox.New(keys)
	if err != nil {
		t.Fatal(err)
	}
	database.SetSecrets(box)
	t.Cleanup(func() { database.SetSecrets(nil) })
}

// storedSecret - the value a secret column holds, as the database has it.
func storedSecret(t *testing.T, db testdb.DB, query string) string {
	t.Helper()
	var v string
	if err := db.DB.QueryRow(query).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestSecretsSealed - with a key the secrets saved are sealed in the
// database and read back plain: mail, CAPTCHA, social sign-in, two-factor;
// the rest of the row stays as it is.
func TestSecretsSealed(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		useSecretsKey(t, "the-site-key-0123456789")
		ctx := context.Background()
		seedUser(t, db)

		mail := database.NewMailSettingsRepository(db.DB)
		if err := mail.Update(ctx, &entity.MailSettings{Enabled: true, Transport: "mailapi", FromAddress: "site@example.com", TransportSettings: map[string]map[string]string{"mailapi": {"api_key": "re_live_key"}}, SMTPPassword: "smtp-pass"}); err != nil {
			t.Fatal(err)
		}
		captcha := database.NewCaptchaSettingsRepository(db.DB)
		if err := captcha.Update(ctx, &entity.CaptchaSettings{Enabled: true, SiteKey: "site-key", SecretKey: "captcha-secret"}); err != nil {
			t.Fatal(err)
		}
		oauth := database.NewOAuthSettingsRepository(db.DB)
		if err := oauth.Upsert(ctx, entity.OAuthProviderSettings{Provider: "google", Enabled: true, ClientID: "id", ClientSecret: "oauth-secret"}); err != nil {
			t.Fatal(err)
		}
		twoFactor := database.NewTwoFactorRepository(db.DB)
		if err := twoFactor.Enable(ctx, 1, "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP", 0, []string{"hash"}, time.Now()); err != nil {
			t.Fatal(err)
		}

		for query, plain := range map[string]string{
			"SELECT value FROM mail_transport_settings": "re_live_key",
			"SELECT smtp_password FROM mail_settings":   "smtp-pass",
			"SELECT secret_key FROM captcha_settings":   "captcha-secret",
			"SELECT client_secret FROM oauth_settings":  "oauth-secret",
			"SELECT secret FROM user_two_factor":        "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP",
		} {
			if v := storedSecret(t, db, query); !secretbox.Sealed(v) || strings.Contains(v, plain) {
				t.Errorf("%s: %q, want it sealed", query, v)
			}
		}
		if v := storedSecret(t, db, "SELECT from_address FROM mail_settings"); v != "site@example.com" {
			t.Errorf("from_address %q", v)
		}

		if m, err := mail.Get(ctx); err != nil || m.TransportSetting("mailapi", "api_key") != "re_live_key" || m.SMTPPassword != "smtp-pass" {
			t.Errorf("mail read back: %+v %v", m, err)
		}
		if c, err := captcha.Get(ctx); err != nil || c.SecretKey != "captcha-secret" {
			t.Errorf("captcha read back: %+v %v", c, err)
		}
		if list, err := oauth.List(ctx); err != nil || list["google"].ClientSecret != "oauth-secret" {
			t.Errorf("oauth read back: %+v %v", list, err)
		}
		if tf, err := twoFactor.ByUser(ctx, 1); err != nil || tf.Secret != "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP" {
			t.Errorf("two-factor read back: %+v %v", tf, err)
		}
	})
}

// TestSealSecrets - the secrets saved before the site had a key are
// sealed at start, the ones sealed with an older key sealed again with the
// new; a key that opens none reads them as none and counts them.
func TestSealSecrets(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		// Saved without a key: plain.
		database.SetSecrets(nil)
		mail := database.NewMailSettingsRepository(db.DB)
		if err := mail.Update(ctx, &entity.MailSettings{Transport: "mailapi", FromAddress: "site@example.com", TransportSettings: map[string]map[string]string{"mailapi": {"api_key": "re_plain"}}}); err != nil {
			t.Fatal(err)
		}
		if v := storedSecret(t, db, "SELECT value FROM mail_transport_settings"); v != "re_plain" {
			t.Fatalf("without a key: %q", v)
		}

		useSecretsKey(t, "the-old-key-0123456789")
		if sealed, bad, err := database.SealSecrets(ctx, db.DB); err != nil || sealed != 1 || len(bad) != 0 {
			t.Fatalf("sealing the plain one: %d %v %v", sealed, bad, err)
		}
		byOld := storedSecret(t, db, "SELECT value FROM mail_transport_settings")
		if !secretbox.Sealed(byOld) {
			t.Fatalf("not sealed: %q", byOld)
		}

		useSecretsKey(t, "the-new-key-0123456789,the-old-key-0123456789")
		if sealed, bad, err := database.SealSecrets(ctx, db.DB); err != nil || sealed != 1 || len(bad) != 0 {
			t.Fatalf("sealing again with the new key: %d %v %v", sealed, bad, err)
		}
		if v := storedSecret(t, db, "SELECT value FROM mail_transport_settings"); v == byOld {
			t.Error("still sealed with the old key")
		}
		if sealed, _, err := database.SealSecrets(ctx, db.DB); err != nil || sealed != 0 {
			t.Errorf("a second run sealed %d (%v)", sealed, err)
		}

		useSecretsKey(t, "a-key-it-was-never-sealed-with")
		if m, err := mail.Get(ctx); err != nil || m.TransportSetting("mailapi", "api_key") != "" {
			t.Errorf("a wrong key read %+v %v, want no key", m, err)
		}
		bad, err := database.UnreadableSecrets(ctx, db.DB)
		if err != nil || bad["Mail: the sending services' settings"] != 1 || len(bad) != 1 {
			t.Errorf("unreadable %v %v", bad, err)
		}
	})
}
