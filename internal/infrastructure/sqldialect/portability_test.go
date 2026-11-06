package sqldialect

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// mysqlOnly - SQL that works on MySQL only, with what to use instead.
var mysqlOnly = map[string]string{
	"ON DUPLICATE KEY": "Dialect.Upsert",
	"LastInsertId":     "Dialect.InsertID",
	"RAND()":           "Dialect.Random",
	"CURDATE":          "Dialect.CurrentDate",
	"UNIX_TIMESTAMP":   "Dialect.UnixSeconds",
	"INSERT IGNORE":    "Dialect.Upsert with no columns to update",
	"REPLACE INTO":     "Dialect.Upsert",
	"GROUP_CONCAT":     "aggregate in Go, or a dialect helper",
}

// legacyMySQLOnly - the MySQL-only SQL written before these helpers, as
// "file|construct" → count. All of it is migrated (docs/contributing/database.md):
// the list stays empty, never add one.
var legacyMySQLOnly = map[string]int{}

// TestNoNewMySQLOnlySQL - Go code doesn't gain MySQL-only SQL: new code
// uses this package's helpers, so the engine can run on PostgreSQL.
func TestNoNewMySQLOnlySQL(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	found := map[string]int{}
	for _, dir := range []string{"internal", "pkg", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if strings.Contains(rel, "/generated") || strings.HasSuffix(rel, "/sqldialect") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for construct := range mysqlOnly {
				if n := strings.Count(string(b), construct); n > 0 {
					found[rel+"|"+construct] = n
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	var keys []string
	for k := range found {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if found[k] > legacyMySQLOnly[k] {
			construct := k[strings.Index(k, "|")+1:]
			t.Errorf("%s: MySQL-only %q (%d, allowed %d) - use sqldialect's %s", k, construct, found[k], legacyMySQLOnly[k], mysqlOnly[construct])
		}
	}
	for k, allowed := range legacyMySQLOnly {
		if found[k] < allowed {
			t.Errorf("%s is migrated (%d left of %d) - lower or delete its legacyMySQLOnly entry", k, found[k], allowed)
		}
	}
}
