//go:build dbintegration

package migrations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestInstallModule - installing a module creates its tables and runs its
// install step once (the complaints' settings row); installing again does
// nothing.
func TestInstallModule(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		if _, err := r.Install(ctx, migrations.CoreSet); err != nil {
			t.Fatal(err)
		}
		if ok, _ := r.Installed(ctx, "complaints"); ok {
			t.Fatal("complaints installed before installing")
		}
		steps, err := r.Install(ctx, "complaints")
		if err != nil {
			t.Fatal(err)
		}
		kinds := map[string]int{}
		for _, s := range steps {
			kinds[s.Kind]++
		}
		if kinds["schema"] == 0 || kinds["install"] != 1 {
			t.Errorf("complaints install steps: %v", kinds)
		}
		var n int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM complaint_settings WHERE id = 1").Scan(&n); err != nil || n != 1 {
			t.Errorf("complaints settings row: %d %v", n, err)
		}
		if again, err := r.Install(ctx, "complaints"); err != nil || len(again) != 0 {
			t.Errorf("second install: %d steps, %v", len(again), err)
		}
	})
}

// TestDataSteps - on a fresh install a set's data steps are only recorded
// (there is no old data); a data step added later runs, in version order.
func TestDataSteps(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		set := "datastep_test_" + db.Driver // no schema files; one per database (the registry is global)
		var ran []string
		step := func(v string) migrations.Update {
			return migrations.Update{Version: v, Description: "step " + v, Run: func(context.Context, migrations.Env) error {
				ran = append(ran, v)
				return nil
			}}
		}
		migrations.RegisterUpdate(set, step("20260101000000"))
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		steps, err := r.Install(ctx, set)
		if err != nil || len(steps) != 1 || steps[0].Kind != "mark" || len(ran) != 0 {
			t.Fatalf("fresh install: %+v %v ran=%v", steps, err, ran)
		}
		migrations.RegisterUpdate(set, step("20270101000000"))
		migrations.RegisterUpdate(set, step("20260601000000"))
		if _, err := r.Install(ctx, set); err != nil {
			t.Fatal(err)
		}
		if len(ran) != 2 || ran[0] != "20260601000000" || ran[1] != "20270101000000" {
			t.Errorf("later steps ran: %v", ran)
		}
	})
}

// TestInstallStepRetried - an install step that failed after the set's
// schema was recorded runs again on the next run, then not any more.
func TestInstallStepRetried(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		set := "installretry_test_" + db.Driver // the registry is global
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		if _, err := r.Install(ctx, migrations.CoreSet); err != nil {
			t.Fatal(err)
		}
		fail := true
		runs := 0
		migrations.RegisterInstall(set, func(context.Context, migrations.Env) error {
			runs++
			if fail {
				return errors.New("the install failed")
			}
			return nil
		})
		migrations.RegisterUpdate(set, migrations.Update{Version: "20260101000000", Description: "a data step", Run: func(context.Context, migrations.Env) error {
			t.Error("a data step ran on an install")
			return nil
		}})
		// The set's schema in place, as before its install step failed.
		if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO schema_migrations (set_name, version, description) VALUES (?, ?, ?)`), set, "20250101000000", "schema"); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Install(ctx, set); err == nil {
			t.Fatal("the failing install step didn't run")
		}
		fail = false
		if _, err := r.Install(ctx, set); err != nil || runs != 2 {
			t.Fatalf("retried: %d runs, %v", runs, err)
		}
		if steps, err := r.Install(ctx, set); err != nil || len(steps) != 0 || runs != 2 {
			t.Errorf("after the install: %+v %v %d runs", steps, err, runs)
		}
	})
}
