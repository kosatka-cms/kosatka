//go:build dbintegration

package errorreports

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestErrorReportRepository - reports are stored with or without a sender
// and listed newest first with the sender's name, a page at a time; one
// marked fixed leaves the new ones (the list and the count) and keeps its
// time when marked again.
func TestErrorReportRepository(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		if _, err := db.DB.Exec(`INSERT INTO users (id, username, slug, password_hash) VALUES (1, 'alice', 'alice', '')`); err != nil {
			t.Fatal(err)
		}
		reports := NewErrorReportRepository(db.DB, testkit.Deps(db).Users)
		uid := 1
		if _, err := reports.Create(ctx, ErrorReport{UserID: &uid, PageURL: "/a", SelectedText: "typo", Comment: "fix", ClientIP: "10.0.0.1"}); err != nil {
			t.Fatal(err)
		}
		if _, err := reports.Create(ctx, ErrorReport{PageURL: "/b", SelectedText: "x"}); err != nil {
			t.Fatal(err)
		}
		list, total, err := reports.ListForAdmin(ctx, ErrorReportListOpts{All: true})
		if err != nil || total != 2 || len(list) != 2 {
			t.Fatalf("reports: %+v %d %v", list, total, err)
		}
		byURL := map[string]ErrorReport{}
		for _, r := range list {
			byURL[r.PageURL] = r
		}
		if a := byURL["/a"]; a.Username != "alice" || a.UserSlug != "alice" || a.Comment != "fix" || a.ClientIP != "10.0.0.1" || a.UserAgent != "" {
			t.Errorf("signed-in report: %+v", a)
		}
		if b := byURL["/b"]; b.UserID != nil || b.Username != "" {
			t.Errorf("anonymous report: %+v", b)
		}
		if list, total, _ := reports.ListForAdmin(ctx, ErrorReportListOpts{Limit: 1, Offset: 1, All: true}); total != 2 || len(list) != 1 {
			t.Errorf("page 2: %+v %d", list, total)
		}

		a := byURL["/a"]
		if n, err := reports.CountNew(ctx); err != nil || n != 2 {
			t.Fatalf("new before: %d %v", n, err)
		}
		if err := reports.Resolve(ctx, a.ID); err != nil {
			t.Fatal(err)
		}
		if n, _ := reports.CountNew(ctx); n != 1 {
			t.Errorf("new after one fixed: %d", n)
		}
		if list, total, _ := reports.ListForAdmin(ctx, ErrorReportListOpts{}); total != 1 || len(list) != 1 || list[0].PageURL != "/b" || list[0].ResolvedAt != nil {
			t.Errorf("new ones: %+v %d", list, total)
		}
		all, _, _ := reports.ListForAdmin(ctx, ErrorReportListOpts{All: true})
		var resolvedAt *time.Time
		for _, r := range all {
			if r.ID == a.ID {
				resolvedAt = r.ResolvedAt
			}
		}
		if resolvedAt == nil {
			t.Fatalf("the fixed one: %+v", all)
		}
		if _, err := db.DB.Exec(db.Dialect.Rebind(`UPDATE error_reports SET resolved_at = ? WHERE id = ?`), time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), a.ID); err != nil {
			t.Fatal(err)
		}
		if err := reports.Resolve(ctx, a.ID); err != nil {
			t.Fatal(err)
		}
		all, _, _ = reports.ListForAdmin(ctx, ErrorReportListOpts{All: true})
		for _, r := range all {
			if r.ID == a.ID && (r.ResolvedAt == nil || r.ResolvedAt.Year() != 2026 || r.ResolvedAt.Month() != 1) {
				t.Errorf("marked again: %v, want the first time kept", r.ResolvedAt)
			}
		}

		if err := reports.Reopen(ctx, a.ID); err != nil {
			t.Fatal(err)
		}
		if n, _ := reports.CountNew(ctx); n != 2 {
			t.Errorf("new after one reopened: %d", n)
		}
		if err := reports.Reopen(ctx, a.ID); err != nil {
			t.Errorf("reopening a new one: %v", err)
		}
	})
}
