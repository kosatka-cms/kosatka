package sqldialect

import "testing"

func TestRebind(t *testing.T) {
	q := "SELECT a FROM t WHERE b = ? AND c = '?' AND d IN (?, ?)"
	if got := MySQL.Rebind(q); got != q {
		t.Errorf("mysql: %s", got)
	}
	if got, want := Postgres.Rebind(q), "SELECT a FROM t WHERE b = $1 AND c = '?' AND d IN ($2, $3)"; got != want {
		t.Errorf("postgres: %s, want %s", got, want)
	}
}

func TestUpsert(t *testing.T) {
	cols := []string{"name", "enabled"}
	if got, want := MySQL.Upsert("modules", cols, []string{"name"}, []string{"enabled"}),
		"INSERT INTO modules (name, enabled) VALUES (?, ?) AS new ON DUPLICATE KEY UPDATE enabled = new.enabled"; got != want {
		t.Errorf("mysql:\n%s\nwant\n%s", got, want)
	}
	if got, want := Postgres.Upsert("modules", cols, []string{"name"}, []string{"enabled"}),
		"INSERT INTO modules (name, enabled) VALUES (?, ?) ON CONFLICT (name) DO UPDATE SET enabled = EXCLUDED.enabled"; got != want {
		t.Errorf("postgres:\n%s\nwant\n%s", got, want)
	}
	if got := Postgres.Upsert("t", cols, []string{"name"}, nil); got != "INSERT INTO t (name, enabled) VALUES (?, ?) ON CONFLICT (name) DO NOTHING" {
		t.Errorf("postgres do nothing: %s", got)
	}
	if got := MySQL.Upsert("t", cols, []string{"name"}, nil); got != "INSERT INTO t (name, enabled) VALUES (?, ?) AS new ON DUPLICATE KEY UPDATE name = new.name" {
		t.Errorf("mysql do nothing: %s", got)
	}
}
