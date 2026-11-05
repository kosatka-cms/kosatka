package migrations_test

import (
	"os"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	entmigrate "github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/migrate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
)

// TestMain registers the engine's modules: the sets are the core's and
// theirs.
func TestMain(m *testing.M) {
	manifests.Register()
	os.Exit(m.Run())
}

// TestSets - every ent table is in exactly one set, the core first; a
// module's tables reference only core tables or its own (never the core
// another module's), so the core can be created without any module.
func TestSets(t *testing.T) {
	sets := migrations.Sets()
	if sets[0].Name != migrations.CoreSet {
		t.Fatalf("first set %q", sets[0].Name)
	}
	owner := map[string]string{}
	for _, s := range sets {
		for _, tb := range s.Tables {
			if prev, ok := owner[tb.Name]; ok {
				t.Errorf("table %s in %s and %s", tb.Name, prev, s.Name)
			}
			owner[tb.Name] = s.Name
		}
	}
	// Every table of the core schema is in a set (a module's own schema's
	// are in its set already).
	for _, tb := range entmigrate.Tables {
		if _, ok := owner[tb.Name]; !ok {
			t.Errorf("table %s in no set", tb.Name)
		}
	}
	for _, m := range entity.Modules {
		for _, name := range m.Tables {
			if owner[name] != m.Name {
				t.Errorf("module %s lists %s, not an ent table of its set", m.Name, name)
			}
		}
	}
	for _, s := range sets {
		for _, tb := range s.Tables {
			for _, fk := range tb.ForeignKeys {
				from, to := owner[tb.Name], owner[fk.RefTable.Name]
				if from != to && to != migrations.CoreSet {
					t.Errorf("%s (%s) references %s (%s): only core tables may be referenced across sets", tb.Name, from, fk.RefTable.Name, to)
				}
			}
		}
	}
}

// TestEmbeddedFiles - every set has its files for both databases and
// their atlas.sum matches.
func TestEmbeddedFiles(t *testing.T) {
	for _, d := range []sqldialect.Dialect{sqldialect.MySQL, sqldialect.Postgres} {
		for _, s := range migrations.Sets() {
			files, err := migrations.Files(d, s.Name)
			if err != nil {
				t.Errorf("%s/%s: %v", d, s.Name, err)
				continue
			}
			if len(s.Tables) > 0 && len(files) == 0 {
				t.Errorf("%s/%s: no migration files", d, s.Name)
			}
		}
	}
}
