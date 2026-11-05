//go:build dbintegration

package schematest

import (
	"os"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// schemaQueries - the schema as tab-separated rows: C (column), I (index),
// F (foreign key); scripts/schema_drift.sh runs the same on the live
// database.
func dump(t *testing.T, db testdb.DB, f *os.File) {
	q := []string{
		`SELECT 'C', table_name, column_name, column_type, is_nullable, COALESCE(column_default,'<null>'), extra FROM information_schema.columns WHERE table_schema = DATABASE() ORDER BY table_name, ordinal_position`,
		`SELECT 'I', table_name, GROUP_CONCAT(column_name ORDER BY seq_in_index), MAX(non_unique), index_name, MAX(index_type), '' FROM information_schema.statistics WHERE table_schema = DATABASE() GROUP BY table_name, index_name ORDER BY 1,2`,
		`SELECT 'F', k.table_name, k.column_name, k.referenced_table_name, k.referenced_column_name, r.delete_rule, k.constraint_name FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.constraint_name=k.constraint_name WHERE k.table_schema = DATABASE() AND k.referenced_table_name IS NOT NULL ORDER BY 2,3`,
	}
	for _, s := range q {
		rows, err := db.DB.Query(s)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			v := make([]string, 7)
			p := make([]any, 7)
			for i := range v {
				p[i] = &v[i]
			}
			if err := rows.Scan(p...); err != nil {
				t.Fatal(err)
			}
			line := v[0]
			for _, x := range v[1:] {
				line += "\t" + x
			}
			if _, err := f.WriteString(line + "\n"); err != nil {
				t.Fatal(err)
			}
		}
	}
}
