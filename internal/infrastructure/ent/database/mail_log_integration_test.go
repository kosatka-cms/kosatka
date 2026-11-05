//go:build dbintegration

package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestMailLogRepository - the log's entries newest first, filtered by a
// part of the address (any case), the email and the result, a page of
// them with the total; when each email was last sent (the sent ones of
// a key only); the old entries deleted.
func TestMailLogRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		repo := database.NewMailLogRepository(db.DB)
		base := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
		for i, e := range []entity.MailLogEntry{
			{Key: "account.password_reset", To: "Anna@Example.com", Subject: "Reset", Status: entity.MailStatusSent},
			{Key: "account.password_reset", To: "bob@example.org", Subject: "Reset", Status: entity.MailStatusFailed, Error: "550"},
			{Key: "", To: "site@example.com", Subject: "A form", Status: entity.MailStatusSent},
			{Key: "pm.new", To: "anna@example.com", Subject: "Message", Status: entity.MailStatusUnsent},
		} {
			e.SentAt = base.Add(time.Duration(i) * time.Hour)
			if err := repo.Add(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
		list := func(f entity.MailLogFilter) (string, int) {
			t.Helper()
			f.Limit = 10
			rows, total, err := repo.List(ctx, f)
			if err != nil {
				t.Fatal(err)
			}
			out := ""
			for _, r := range rows {
				out += r.To + ";"
			}
			return out, total
		}
		for _, c := range []struct {
			f     entity.MailLogFilter
			want  string
			total int
		}{
			{entity.MailLogFilter{}, "anna@example.com;site@example.com;bob@example.org;Anna@Example.com;", 4},
			{entity.MailLogFilter{To: "ANNA"}, "anna@example.com;Anna@Example.com;", 2},
			{entity.MailLogFilter{Key: "account.password_reset", Status: entity.MailStatusFailed}, "bob@example.org;", 1},
		} {
			if got, total := list(c.f); got != c.want || total != c.total {
				t.Errorf("%+v: %s (%d), want %s (%d)", c.f, got, total, c.want, c.total)
			}
		}
		if rows, total, err := repo.List(ctx, entity.MailLogFilter{Limit: 1, Offset: 1}); err != nil || total != 4 || len(rows) != 1 || rows[0].To != "site@example.com" {
			t.Errorf("a page: %v %d %v", rows, total, err)
		}
		last, err := repo.LastSent(ctx)
		if err != nil || len(last) != 1 || !last["account.password_reset"].Equal(base) {
			t.Errorf("last sent: %v %v", last, err)
		}
		if err := repo.DeleteBefore(ctx, base.Add(90*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if got, total := list(entity.MailLogFilter{}); got != "anna@example.com;site@example.com;" || total != 2 {
			t.Errorf("after deleting: %s (%d)", got, total)
		}
	})
}
