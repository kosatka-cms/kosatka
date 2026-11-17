//go:build dbintegration

package database_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestBlockedIPSearch - the blocked IPs a page at a time, newest first,
// with how many there are; a search matches the address or the comment,
// in any case.
func TestBlockedIPSearch(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		ips := database.NewBlockedIPRepository(db.DB)
		for i, c := range []struct{ ip, comment string }{
			{"10.0.0.1", "Spam bot"}, {"10.0.0.2", ""}, {"192.0.2.0/24", "scanner"}, {"198.51.100.7", "SPAM again"}, {"2001:db8::/32", ""},
		} {
			if _, err := ips.Create(ctx, c.ip, c.comment); err != nil {
				t.Fatalf("create %d: %v", i, err)
			}
		}
		ipsOf := func(q string, offset, limit int) string {
			t.Helper()
			list, total, err := ips.Search(ctx, q, offset, limit)
			if err != nil {
				t.Fatal(err)
			}
			out := fmt.Sprint(total, ":")
			for _, b := range list {
				out += " " + b.IP
			}
			return out
		}
		for _, c := range []struct {
			q             string
			offset, limit int
			want          string
		}{
			{"", 0, 2, "5: 2001:db8::/32 198.51.100.7"},
			{"", 4, 2, "5: 10.0.0.1"},
			{"10.0", 0, 50, "2: 10.0.0.2 10.0.0.1"},
			{"spam", 0, 50, "2: 198.51.100.7 10.0.0.1"},
			{"nothing", 0, 50, "0:"},
		} {
			if got := ipsOf(c.q, c.offset, c.limit); got != c.want {
				t.Errorf("%q from %d: %s, want %s", c.q, c.offset, got, c.want)
			}
		}
	})
}
