package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

func line(t *testing.T, c StatusCheck) entity.Requirement {
	t.Helper()
	rs := c(context.Background())
	if len(rs) != 1 {
		t.Fatalf("lines: %+v", rs)
	}
	return rs[0]
}

// TestStatusReport - errors first, then warnings, then the rest; a
// disabled module's checks don't run; the bar's counts are kept for the
// ttl.
func TestStatusReport(t *testing.T) {
	t.Cleanup(func() { SetModules(map[string]bool{"status_off": true}) })
	SetModules(map[string]bool{"status_off": false})
	runs := 0
	r := NewStatusReport(time.Hour)
	r.Add(func(context.Context) []entity.Requirement {
		runs++
		return []entity.Requirement{{Title: "ok"}, {Title: "warn", Severity: entity.RequirementWarning}}
	})
	r.AddModule("status_on", func(context.Context) []entity.Requirement {
		return []entity.Requirement{{Title: "err", Severity: entity.RequirementError}}
	})
	r.AddModule("status_off", func(context.Context) []entity.Requirement {
		return []entity.Requirement{{Title: "hidden", Severity: entity.RequirementError}}
	})
	got := r.Run(context.Background())
	var titles []string
	for _, x := range got {
		titles = append(titles, x.Title)
	}
	if strings.Join(titles, ",") != "err,warn,ok" || got[0].Module != "status_on" {
		t.Fatalf("order/modules: %v %+v", titles, got)
	}
	if w, e := r.Counts(context.Background()); w != 1 || e != 1 || runs != 1 {
		t.Fatalf("counts %d/%d after %d runs", w, e, runs)
	}
	r.ttl = 0
	r.Counts(context.Background())
	if runs != 2 {
		t.Fatalf("stale counts must run the checks again: %d runs", runs)
	}
}

func TestCheckPendingMigrations(t *testing.T) {
	if r := line(t, CheckPendingMigrations(func(context.Context) (map[string]int, error) { return map[string]int{"core": 0}, nil })); r.Severity != entity.RequirementOK {
		t.Errorf("none pending: %+v", r)
	}
	r := line(t, CheckPendingMigrations(func(context.Context) (map[string]int, error) {
		return map[string]int{"core": 2, "forum": 1, "polls": 0}, nil
	}))
	if !r.IsError() || !strings.Contains(r.Description, "core: 2, forum: 1") || strings.Contains(r.Description, "polls") {
		t.Errorf("pending: %+v", r)
	}
	r = line(t, CheckPendingMigrations(func(context.Context) (map[string]int, error) { return nil, errors.New("<db> down") }))
	if !r.IsWarning() || r.Description != "&lt;db&gt; down" {
		t.Errorf("unchecked: %+v", r)
	}
}

// TestCheckMissingModules - no line while the binary has every module
// installed; a warning naming the missing ones; one when it can't tell.
func TestCheckMissingModules(t *testing.T) {
	if rs := CheckMissingModules(func(context.Context) ([]string, error) { return nil, nil })(context.Background()); len(rs) != 0 {
		t.Errorf("none missing: %+v", rs)
	}
	r := line(t, CheckMissingModules(func(context.Context) ([]string, error) { return []string{"greetings", "weather"}, nil }))
	if !r.IsWarning() || r.Value != "greetings, weather" || !strings.Contains(r.Description, "Modules page") {
		t.Errorf("missing: %+v", r)
	}
	r = line(t, CheckMissingModules(func(context.Context) ([]string, error) { return nil, errors.New("<db> down") }))
	if !r.IsWarning() || r.Description != "&lt;db&gt; down" {
		t.Errorf("unchecked: %+v", r)
	}
}

func TestCheckWritableDir(t *testing.T) {
	dir := t.TempDir()
	if r := line(t, CheckWritableDir("Files", func() string { return dir })); r.Severity != entity.RequirementOK || r.Value != dir {
		t.Errorf("writable: %+v", r)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("the check left %d files", len(entries))
	}
	missing := filepath.Join(dir, "missing")
	if r := line(t, CheckWritableDir("Files", func() string { return missing })); !r.IsError() {
		t.Errorf("missing: %+v", r)
	}
}

func TestCheckWritableStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "files")
	store := func() filestore.Store { return filestore.NewLocal(dir) }
	if r := line(t, CheckWritableStore("Files", store)); r.Severity != entity.RequirementOK || r.Value != dir {
		t.Errorf("writable: %+v", r)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Errorf("the check left %d files (%v)", len(entries), err)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// A file where the directory should be.
	if r := line(t, CheckWritableStore("Files", func() filestore.Store { return filestore.NewLocal(blocked) })); !r.IsError() {
		t.Errorf("not a directory: %+v", r)
	}
}

func TestCheckSiteURL(t *testing.T) {
	prev := entity.NewSystemSettings().Domain
	t.Cleanup(func() { entity.SetSiteDomain(prev) })
	entity.SetSiteDomain("")
	if r := line(t, CheckSiteURL()); !r.IsWarning() {
		t.Errorf("unset: %+v", r)
	}
	entity.SetSiteDomain("https://example.org")
	if r := line(t, CheckSiteURL()); r.Severity != entity.RequirementOK || r.Value != "https://example.org" {
		t.Errorf("set: %+v", r)
	}
}

func TestCheckMail(t *testing.T) {
	for st, warn := range map[MailStatus]bool{
		{Enabled: false}:                                    true,
		{Enabled: true, Source: MailSourceNone}:             true,
		{Enabled: true, Source: "admin", Transport: "smtp"}: false,
	} {
		if r := line(t, CheckMail(func() MailStatus { return st })); r.IsWarning() != warn {
			t.Errorf("%+v: %+v", st, r)
		}
	}
}

func TestCheckSQLMode(t *testing.T) {
	if rs := CheckSQLMode("postgres", "server")(context.Background()); len(rs) != 0 {
		t.Errorf("PostgreSQL has no SQL mode: %+v", rs)
	}
	if r := line(t, CheckSQLMode("mysql", "server")); !r.IsWarning() {
		t.Errorf("server mode: %+v", r)
	}
	if r := line(t, CheckSQLMode("mysql", "STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION")); r.Severity != entity.RequirementOK {
		t.Errorf("strict: %+v", r)
	}
}

func TestCheckCacheRefresh(t *testing.T) {
	at := func(d time.Time) StatusCheck { return CheckCacheRefresh(func() time.Time { return d }, 15*time.Minute) }
	if r := line(t, at(time.Time{})); !r.IsWarning() {
		t.Errorf("never: %+v", r)
	}
	if r := line(t, at(time.Now().Add(-time.Hour))); !r.IsWarning() {
		t.Errorf("stale: %+v", r)
	}
	if r := line(t, at(time.Now().Add(-time.Minute))); r.Severity != entity.RequirementOK {
		t.Errorf("fresh: %+v", r)
	}
}

// TestCheckSecrets - no key: a warning; sealed: fine; a guessable key: a
// warning; some the key doesn't open: an error naming where; a failed
// check: a warning.
func TestCheckSecrets(t *testing.T) {
	ctx := context.Background()
	none := func(context.Context) (map[string]int, error) { return nil, nil }
	no, yes := func() bool { return false }, func() bool { return true }
	if r := CheckSecrets(yes, yes, none)(ctx); len(r) != 1 || r[0].Severity != entity.RequirementWarning || !strings.Contains(r[0].Description, "new,old") {
		t.Errorf("a weak key: %+v", r)
	}
	if r := CheckSecrets(func() bool { return false }, no, none)(ctx); len(r) != 1 || r[0].Severity != entity.RequirementWarning || !strings.Contains(r[0].Description, "SECRETS_KEY") {
		t.Errorf("no key: %+v", r)
	}
	if r := CheckSecrets(func() bool { return true }, no, none)(ctx); len(r) != 1 || r[0].Severity != entity.RequirementOK {
		t.Errorf("sealed: %+v", r)
	}
	bad := func(context.Context) (map[string]int, error) {
		return map[string]int{"Two-factor sign-in of users": 2, "Mail: SMTP password": 1}, nil
	}
	if r := CheckSecrets(func() bool { return true }, no, bad)(ctx); len(r) != 1 || r[0].Severity != entity.RequirementError ||
		!strings.Contains(r[0].Description, "Mail: SMTP password (1), Two-factor sign-in of users (2)") {
		t.Errorf("unreadable: %+v", r)
	}
	failed := func(context.Context) (map[string]int, error) { return nil, errors.New("db down") }
	if r := CheckSecrets(func() bool { return true }, no, failed)(ctx); len(r) != 1 || r[0].Severity != entity.RequirementWarning {
		t.Errorf("failed: %+v", r)
	}
}
