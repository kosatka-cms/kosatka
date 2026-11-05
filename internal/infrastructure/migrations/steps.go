package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	// The schema's defaults, hooks and interceptors (the blocks fields'
	// items left out of node queries) - with every core client.
	_ "github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/runtime"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
)

// Env - what a Go step works with.
//
// An install step (RegisterInstall) writes its starter rows with Client:
// the schema is the current one then, so the generated client fits it
// (names checked by the compiler, values sent as parameters). A data step
// (RegisterUpdate) runs on the schema as it was when the step was
// released, which the generated client doesn't describe - it uses SQL on
// DB, values only as parameters (? and Dialect.Rebind, or ent's
// dialect/sql builder); DDL and information_schema are SQL too.
type Env struct {
	DB      *sql.DB
	Dialect sqldialect.Dialect
	Client  *generated.Client
}

// env - the Env of the runner's database.
func (r Runner) env() Env {
	return Env{DB: r.DB, Dialect: r.Dialect, Client: generated.NewClient(generated.Driver(entsql.OpenDB(string(r.Dialect), r.DB)))}
}

// Update - a data step of a set: Go code
// for what a schema file can't do (moving or fixing data). Its Version
// shares the set's sequence with the schema files (a timestamp like
// 20261015120000, after the files it relies on) and is recorded the
// same way.
type Update struct {
	Version     string
	Description string
	Run         func(ctx context.Context, env Env) error
}

// InstallVersion - the schema_migrations version of a set's install step.
const InstallVersion = "install"

var (
	updates  = map[string][]Update{}
	installs = map[string]func(ctx context.Context, env Env) error{}
)

// RegisterUpdate adds a data step to a set (from an init function).
func RegisterUpdate(set string, u Update) {
	updates[set] = append(updates[set], u)
}

// RegisterInstall sets a set's install step: it runs once, after the set's tables are first created - the rows the
// module can't work without. One that failed runs again on the next run
// (it isn't recorded, nor are the set's data steps), so a set's install
// step is there from its first version: initial rows a set gets later are
// a data step (RegisterUpdate), the sites having it already need them.
func RegisterInstall(set string, run func(ctx context.Context, env Env) error) {
	installs[set] = run
}

// Step - one pending item of a set: a schema file or a data step.
type Step struct {
	Version     string
	Description string
	// Kind - "schema", "data", "install" or "mark" (a data step recorded
	// without running: a fresh install has no old data to update).
	Kind string
	file File
	run  func(ctx context.Context, env Env) error
}

// Plan - what Run would do for the set, in order. A set with nothing
// applied yet is installed: every schema file, then its install step,
// and its data steps are only recorded; so is one whose install failed
// (installUnfinished): its pending schema files, the install step, the
// marks. Otherwise the pending schema files and data steps run in
// version order.
func (r Runner) Plan(ctx context.Context, set string) ([]Step, error) {
	files, err := Files(r.Dialect, set)
	if err != nil {
		return nil, err
	}
	applied, err := r.Applied(ctx, set)
	if err != nil {
		return nil, err
	}
	var steps []Step
	for _, f := range files {
		if !applied[f.Version] {
			steps = append(steps, Step{Version: f.Version, Description: f.Description, Kind: "schema", file: f})
		}
	}
	if len(applied) == 0 || installUnfinished(set, applied) {
		if run, ok := installs[set]; ok {
			steps = append(steps, Step{Version: InstallVersion, Description: "install", Kind: "install", run: run})
		}
		for _, u := range sortedUpdates(set) {
			steps = append(steps, Step{Version: u.Version, Description: u.Description, Kind: "mark"})
		}
		return steps, nil
	}
	for _, u := range sortedUpdates(set) {
		if !applied[u.Version] {
			steps = append(steps, Step{Version: u.Version, Description: u.Description, Kind: "data", run: u.Run})
		}
	}
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].Version < steps[j].Version })
	return steps, nil
}

// changing - a schema statement's words that drop or change something;
// fkAction - a foreign key's ON DELETE/ON UPDATE action, which only
// looks like one.
var (
	changing = regexp.MustCompile(`\b(DROP|TRUNCATE|DELETE|RENAME|MODIFY|CHANGE)\b|\bALTER\s+COLUMN\b`)
	fkAction = regexp.MustCompile(`\bON\s+(DELETE|UPDATE)\s+(CASCADE|RESTRICT|SET\s+NULL|SET\s+DEFAULT|NO\s+ACTION)\b`)
)

// Additive reports whether the step leaves alone what an older binary
// still running uses: a schema file that only adds (no statement drops,
// truncates, deletes, renames or changes a column - its comments and its
// foreign keys' ON DELETE/ON UPDATE actions aside), or a Go step.
func (s Step) Additive() bool {
	if s.Kind != "schema" {
		return true
	}
	for _, stmt := range s.file.Stmts {
		var code []string
		for _, line := range strings.Split(stmt, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				code = append(code, line)
			}
		}
		if changing.MatchString(fkAction.ReplaceAllString(strings.Join(code, "\n"), "")) {
			return false
		}
	}
	return true
}

// AdditivePrefix - steps (a set's plan, in order) up to the first that
// isn't Additive: what an older binary still running is fine with (run);
// stop - that first one, which with the steps after it waits for the
// restart (nil - every step is additive).
func AdditivePrefix(steps []Step) (run []Step, stop *Step) {
	for i := range steps {
		if !steps[i].Additive() {
			return steps[:i], &steps[i]
		}
	}
	return steps, nil
}

// installUnfinished - whether the set's install stopped at its install
// step: the set has one, it isn't recorded, and neither is any data step
// (a fresh install records them after it).
func installUnfinished(set string, applied map[string]bool) bool {
	if _, ok := installs[set]; !ok || applied[InstallVersion] {
		return false
	}
	for _, u := range updates[set] {
		if applied[u.Version] {
			return false
		}
	}
	return true
}

// Run executes a planned step and records it.
func (r Runner) Run(ctx context.Context, set string, s Step) error {
	switch s.Kind {
	case "schema":
		return r.Apply(ctx, set, s.file)
	case "install", "data":
		if err := s.run(ctx, r.env()); err != nil {
			return fmt.Errorf("%s %s (%s): %w", set, s.Version, s.Description, err)
		}
	}
	return r.record(ctx, set, File{Version: s.Version, Description: s.Description})
}

// Install brings a set up to date - installs it when nothing of it is
// applied yet - and returns the steps run.
func (r Runner) Install(ctx context.Context, set string) ([]Step, error) {
	if err := r.EnsureTable(ctx); err != nil {
		return nil, err
	}
	steps, err := r.Plan(ctx, set)
	if err != nil {
		return nil, err
	}
	for i, s := range steps {
		if err := r.Run(ctx, set, s); err != nil {
			return steps[:i], err
		}
	}
	return steps, nil
}

// Installed - whether anything of the set is applied (its tables exist).
func (r Runner) Installed(ctx context.Context, set string) (bool, error) {
	if err := r.EnsureTable(ctx); err != nil {
		return false, err
	}
	applied, err := r.Applied(ctx, set)
	return len(applied) > 0, err
}

func sortedUpdates(set string) []Update {
	us := append([]Update(nil), updates[set]...)
	sort.SliceStable(us, func(i, j int) bool { return us[i].Version < us[j].Version })
	return us
}
