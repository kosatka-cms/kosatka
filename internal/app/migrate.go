package app

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// runMigrate is "app migrate": the schema migrations of the core and of
// every module that is enabled or already installed
// (internal/infrastructure/migrations). Without a flag it lists what
// would run; -apply runs it (with -additive only, per set, the steps
// before its first schema file that drops or changes something - what an
// older binary still running is fine with, before its restart);
// -baseline records every file as applied without running it - once, for
// a database whose schema already matches them (scripts/schema_drift.sh
// shows whether it does).
func runMigrate(args []string) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	apply := fs.Bool("apply", false, "apply the pending migrations")
	additive := fs.Bool("additive", false, "with -apply: per set, only the steps before its first schema file that drops or changes something (an older binary still running is fine with them)")
	baseline := fs.Bool("baseline", false, "record the migrations as applied without running them (a database that already has the schema)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *apply && *baseline {
		fmt.Fprintln(os.Stderr, "migrate: -apply and -baseline exclude each other")
		return 2
	}
	if *additive && !*apply {
		fmt.Fprintln(os.Stderr, "migrate: -additive goes with -apply")
		return 2
	}
	cfg := loadConfig()
	// The config's theme, as the site has it: a data step placing blocks
	// (PlaceModuleBlocks) uses it on a site that chose none on the
	// Appearance page.
	paths.SetTheme(cfg.Theme)
	db, _, err := NewEntConn(&cfg.DB)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	r := migrations.Runner{DB: db, Dialect: sqldialect.Dialect(cfg.DB.Dialect())}
	if err := r.EnsureTable(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		return 1
	}
	// The modules switched on or off; a fresh database (no core yet, so
	// no modules table) has none switched off.
	states := map[string]bool{}
	core, err := r.Applied(ctx, migrations.CoreSet)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		return 1
	}
	if len(core) > 0 {
		if states, err = database.NewModuleRepository(db).States(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "migrate: module states:", err)
			return 1
		}
		missing, err := missingModules(ctx, r)
		if err != nil {
			fmt.Fprintln(os.Stderr, "migrate:", err)
			return 1
		}
		for _, name := range missing {
			fmt.Printf("%s: %s\n", name, missingModulesHint)
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	siteInfo := database.NewSiteInformationRepository(db)
	siteLanguage := func(ctx context.Context) (string, error) {
		site, err := siteInfo.Get(ctx)
		if err != nil || site == nil {
			return "", err
		}
		return site.DefaultLanguage, nil
	}
	total, left := 0, 0
	for _, set := range migrations.Sets() {
		applied, err := r.Applied(ctx, set.Name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "migrate:", err)
			return 1
		}
		enabled, known := states[set.Name]
		if !known {
			// No state row: enabled, unless the module is off by default.
			m, _ := entity.ModuleByName(set.Name)
			enabled = !m.OffByDefault
		}
		// The core always; a module when it's enabled or its tables exist
		// already.
		if set.Name != migrations.CoreSet && !enabled && len(applied) == 0 {
			fmt.Printf("%s: not installed and switched off - skipped\n", set.Name)
			continue
		}
		if *baseline && len(applied) > 0 {
			fmt.Printf("%s: already tracked (%d applied) - baseline skipped\n", set.Name, len(applied))
			continue
		}
		steps, err := r.Plan(ctx, set.Name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "migrate:", err)
			return 1
		}
		// A module installed here on a site (enabled, its tables not there
		// yet) gets what the Modules page gives it after its set: its
		// blocks, its labels in the site's language. A fresh database has
		// no site yet: app install refuses one with the core.
		installing := *apply && len(core) > 0 && set.Name != migrations.CoreSet && len(applied) == 0 && len(steps) > 0
		// -additive: the first step that drops or changes something and the
		// set's later steps wait for the restart.
		var stop *migrations.Step
		if *additive {
			steps, stop = migrations.AdditivePrefix(steps)
		}
		for _, st := range steps {
			label := st.Version + "_" + st.Description
			total++
			switch {
			case *apply:
				if err := r.Run(ctx, set.Name, st); err != nil {
					fmt.Fprintln(os.Stderr, "migrate:", err)
					return 1
				}
				fmt.Printf("%s: %s %s\n", set.Name, map[string]string{"schema": "applied", "install": "installed", "data": "ran", "mark": "recorded (fresh install, nothing to update)"}[st.Kind], label)
			case *baseline:
				// The database has this already: record every step,
				// run none (the install step included).
				if err := r.MarkApplied(ctx, set.Name, migrations.File{Version: st.Version, Description: st.Description}); err != nil {
					fmt.Fprintln(os.Stderr, "migrate:", err)
					return 1
				}
				fmt.Printf("%s: recorded %s (baseline, not run)\n", set.Name, label)
			default:
				fmt.Printf("%s: pending %s %s\n", set.Name, st.Kind, label)
			}
		}
		if stop != nil {
			fmt.Printf("%s: stopped before %s %s_%s: it drops or changes something\n", set.Name, stop.Kind, stop.Version, stop.Description)
			left++
			installing = false
		}
		if installing {
			finishModuleInstall(ctx, r, set.Name, siteLanguage, logger)
			fmt.Printf("%s: installed - its blocks placed, its labels in the site's language\n", set.Name)
		}
	}
	switch {
	case left > 0:
		fmt.Printf("migrate: %d set(s) stopped before a step that drops or changes something: run -apply once the older binary is stopped\n", left)
	case total == 0:
		fmt.Println("migrate: nothing to do")
	case !*apply && !*baseline:
		fmt.Println("migrate: run with -apply to apply them")
	}
	return 0
}
