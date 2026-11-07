package migrations

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
)

// TestStepVersionsUnique - a data step's version is not a schema file's of
// its set (both are recorded in schema_migrations by version), in either
// database.
func TestStepVersionsUnique(t *testing.T) {
	for set, steps := range updates {
		for _, d := range []sqldialect.Dialect{sqldialect.MySQL, sqldialect.Postgres} {
			files, err := Files(d, set)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]string{}
			for _, f := range files {
				seen[f.Version] = "schema " + f.Description
			}
			for _, u := range steps {
				if prev, ok := seen[u.Version]; ok {
					t.Errorf("%s/%s: step %s has the version of %s (%s)", d, set, u.Description, prev, u.Version)
				}
				seen[u.Version] = "step " + u.Description
			}
		}
	}
}

// TestStepAdditive - a schema file that only adds is additive, its
// comments and foreign keys' actions aside; one dropping or changing a
// column isn't; Go steps are.
func TestStepAdditive(t *testing.T) {
	for _, tc := range []struct {
		stmts []string
		want  bool
	}{
		{[]string{"CREATE TABLE `a` (`id` int, CONSTRAINT `a_b` FOREIGN KEY (`b`) REFERENCES `b` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE)"}, true},
		{[]string{"-- Modify \"users\" table\nALTER TABLE `users` ADD COLUMN `session_version` bigint NOT NULL DEFAULT 0"}, true},
		{[]string{"ALTER TABLE `users` ADD COLUMN `deleted_at` timestamp NULL"}, true},
		{[]string{"ALTER TABLE `a` ADD COLUMN `x` int", "ALTER TABLE `comments` DROP COLUMN `name`"}, false},
		{[]string{"ALTER TABLE `t` MODIFY COLUMN `description` longtext NULL"}, false},
		{[]string{`ALTER TABLE "t" ALTER COLUMN "description" TYPE text`}, false},
		{[]string{"DELETE FROM `t`"}, false},
	} {
		if got := (Step{Kind: "schema", file: File{Stmts: tc.stmts}}).Additive(); got != tc.want {
			t.Errorf("%q: additive %v, want %v", tc.stmts, got, tc.want)
		}
	}
	for _, kind := range []string{"data", "install", "mark"} {
		if !(Step{Kind: kind}).Additive() {
			t.Errorf("a %s step isn't additive", kind)
		}
	}
	// Every set's baseline only adds.
	for _, set := range Sets() {
		for _, d := range []sqldialect.Dialect{sqldialect.MySQL, sqldialect.Postgres} {
			files, err := Files(d, set.Name)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range files {
				if f.Description == "baseline" && !(Step{Kind: "schema", file: f}).Additive() {
					t.Errorf("%s/%s: the baseline isn't additive", d, set.Name)
				}
			}
		}
	}
}

// TestAdditivePrefix - the steps before the first schema file that drops
// or changes something run (data steps and adding files among them); it
// and the steps after it, adding ones too, wait.
func TestAdditivePrefix(t *testing.T) {
	adds := Step{Version: "2", Kind: "schema", file: File{Stmts: []string{"ALTER TABLE `a` ADD COLUMN `x` int"}}}
	drops := Step{Version: "3", Kind: "schema", file: File{Stmts: []string{"ALTER TABLE `a` DROP COLUMN `y`"}}}
	steps := []Step{{Version: "1", Kind: "data"}, adds, drops, {Version: "4", Kind: "data"}, adds}
	run, stop := AdditivePrefix(steps)
	if len(run) != 2 || run[0].Version != "1" || run[1].Version != "2" || stop == nil || stop.Version != "3" {
		t.Errorf("run %+v, stop %+v", run, stop)
	}
	if run, stop := AdditivePrefix([]Step{{Version: "1", Kind: "data"}, adds}); len(run) != 2 || stop != nil {
		t.Errorf("all additive: run %+v, stop %+v", run, stop)
	}
	if run, stop := AdditivePrefix(nil); len(run) != 0 || stop != nil {
		t.Errorf("none: run %+v, stop %+v", run, stop)
	}
}

// TestCheckHistory - recorded versions the binary doesn't know, without
// its first file recorded, are another history (a database set up before
// the files were squashed): an error naming them and the first file; a
// database with the first file and newer unknown versions (rolled back),
// a fresh one, or one with known versions only is fine.
func TestCheckHistory(t *testing.T) {
	files := []File{{Version: "20261105193024", Description: "baseline"}, {Version: "20261106173341", Description: "mail_transport_settings"}}
	saved := updates["testset"]
	updates["testset"] = []Update{{Version: "20261106173400", Description: "data"}}
	t.Cleanup(func() { updates["testset"] = saved })
	for name, tc := range map[string]struct {
		applied map[string]bool
		unknown []string
	}{
		"fresh":                   {map[string]bool{}, nil},
		"current":                 {map[string]bool{"20261105193024": true, InstallVersion: true, "20261106173400": true}, nil},
		"a newer binary's":        {map[string]bool{"20261105193024": true, "20271201000000": true}, nil},
		"only the install step":   {map[string]bool{InstallVersion: true}, nil},
		"another history":         {map[string]bool{"20261102172620": true, "20261104154526": true, InstallVersion: true}, []string{"20261102172620", "20261104154526"}},
		"another, later recorded": {map[string]bool{"20261102172620": true, "20261106173341": true}, []string{"20261102172620"}},
	} {
		err := checkHistory("testset", files, tc.applied)
		var he *HistoryError
		if tc.unknown == nil {
			if err != nil {
				t.Errorf("%s: %v", name, err)
			}
			continue
		}
		if !errors.As(err, &he) || !slices.Equal(he.Unknown, tc.unknown) || he.Baseline != "20261105193024_baseline" {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if msg := err.Error(); !strings.Contains(msg, "testset: the database's migrations are of another history: it has "+tc.unknown[0]) || !strings.Contains(msg, "20261105193024_baseline") {
			t.Errorf("%s: %q", name, msg)
		}
	}
}
