package app

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/config"
)

// TestDumpCommand - each database's tool, its password in the
// environment only.
func TestDumpCommand(t *testing.T) {
	for _, tc := range []struct {
		driver, tool, env string
	}{{"mysql", "mysqldump", "MYSQL_PWD=s3cret"}, {"postgres", "pg_dump", "PGPASSWORD=s3cret"}} {
		name, args, env := dumpCommand(config.DBConfig{Driver: tc.driver, Host: "db", Port: 5, Username: "u", Password: "s3cret", Scheme: "site"})
		if name != tc.tool || len(env) != 1 || env[0] != tc.env {
			t.Errorf("%s: %s %v", tc.driver, name, env)
		}
		if joined := strings.Join(args, " "); strings.Contains(joined, "s3cret") || !strings.Contains(joined, "site") {
			t.Errorf("%s: args %q", tc.driver, joined)
		}
	}
}
