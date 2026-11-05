package app

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
)

// missingModulesHint - what a module missing from the binary means and
// what to do about it (the startup log, app migrate).
const missingModulesHint = "installed in the database, not in this binary: its content and blocks are unavailable, its data stays; to uninstall it, deploy a binary that has it (the same version) and uninstall it on the Modules page"

// missingModules - the modules installed in the site's database (their
// migration set recorded: their tables and data are there) that this
// binary doesn't have, a build without them: their content doesn't open,
// their pages and blocks are gone, their criteria drop out of Views, and
// only a binary with them can uninstall them. Sorted.
func missingModules(ctx context.Context, r migrations.Runner) ([]string, error) {
	names, err := r.SetNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the installed modules: %w", err)
	}
	known := map[string]bool{}
	for _, s := range migrations.Sets() {
		known[s.Name] = true
	}
	var out []string
	for _, n := range names {
		if !known[n] {
			out = append(out, n)
		}
	}
	return out, nil
}
