package app

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/modules"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// statusReportTTL - how long the admin's warning bar trusts the checks'
// last counts.
const statusReportTTL = 5 * time.Minute

// statusDeps - what the status report's checks and sources read.
type statusDeps struct {
	Runner migrations.Runner
	DB     *sql.DB
	Config *config.Config
	Mail   *service.MailService
	Cache  *service.CacheService
	// Scheduler / Schedule - the background jobs and what the scheduled
	// publishing waits for.
	Scheduler *service.Scheduler
	Schedule  entity.NodeScheduleRepository
	Modules   []modules.Wired
}

// setupStatusReport - the status report (Reports → Status report and the
// admin's warning bar): the core's checks, the enabled modules' own
// (modulekit.Parts.Requirements), the modules' state and the database.
func setupStatusReport(d statusDeps) {
	report := service.NewStatusReport(statusReportTTL)
	report.Add(service.CheckPendingMigrations(func(ctx context.Context) (map[string]int, error) {
		return pendingMigrations(ctx, d.Runner)
	}))
	report.Add(service.CheckMissingModules(func(ctx context.Context) ([]string, error) {
		return missingModules(ctx, d.Runner)
	}))
	report.Add(service.CheckWritableStore(i18n.N("Public files"), paths.PublicStore))
	report.Add(service.CheckSiteURL())
	report.Add(service.CheckMail(d.Mail.Status))
	report.Add(service.CheckSQLMode(d.Config.DB.Dialect(), d.Config.DB.SQLMode))
	// A development site's key may be an example: no warning.
	weak := func() bool { return d.Config.IsProd() && database.SecretsKeyWeak() }
	report.Add(service.CheckSecrets(database.SecretsSealed, weak, func(ctx context.Context) (map[string]int, error) {
		return database.UnreadableSecrets(ctx, d.DB)
	}))
	report.Add(service.CheckCacheRefresh(func() time.Time { return d.Cache.GetSidebarData().LastUpdated }, d.Cache.UpdateInterval()))
	report.Add(service.CheckScheduler(d.Scheduler.LastTick, d.Schedule.Schedule))
	for _, m := range d.Modules {
		if m.Requirements != nil {
			report.AddModule(m.Name, m.Requirements)
		}
	}
	interfaces.SetStatusReport(report, interfaces.StatusSources{
		Modules: func(ctx context.Context) ([]entity.StatusModule, error) {
			return statusModules(ctx, d.Runner)
		},
		Database: func(ctx context.Context) (string, string, error) {
			return databaseVersion(ctx, d.DB, d.Config.DB.Dialect())
		},
	})
}

// pendingMigrations - the steps not applied yet, by set: the core and
// every installed module (a module never installed has nothing pending -
// enabling it installs it).
func pendingMigrations(ctx context.Context, r migrations.Runner) (map[string]int, error) {
	out := map[string]int{}
	for _, set := range migrations.Sets() {
		if set.Name != migrations.CoreSet {
			// Read only (Installed would create schema_migrations).
			applied, err := r.Applied(ctx, set.Name)
			if err != nil {
				return nil, fmt.Errorf("checking whether %s is installed: %w", set.Name, err)
			}
			if len(applied) == 0 {
				continue
			}
		}
		steps, err := r.Plan(ctx, set.Name)
		if err != nil {
			return nil, fmt.Errorf("planning %s: %w", set.Name, err)
		}
		out[set.Name] = len(steps)
	}
	return out, nil
}

// statusModules - every module: label, enabled, installed, the step of
// its set applied last (version, name, when).
func statusModules(ctx context.Context, r migrations.Runner) ([]entity.StatusModule, error) {
	out := make([]entity.StatusModule, 0, len(entity.Modules))
	for _, m := range entity.Modules {
		last, installed, err := r.LastApplied(ctx, m.Name)
		if err != nil {
			return nil, fmt.Errorf("reading %s migrations: %w", m.Name, err)
		}
		out = append(out, entity.StatusModule{
			Name:          m.Name,
			Label:         i18n.T(ctx, m.Label),
			Installed:     installed,
			Enabled:       installed && service.ModuleEnabled(m.Name),
			SchemaVersion: last.Version,
			SchemaStep:    last.Description,
			SchemaUpdated: last.AppliedAt,
		})
	}
	return out, nil
}

// databaseVersion - the database server's version.
func databaseVersion(ctx context.Context, db *sql.DB, dialect string) (string, string, error) {
	query := "SELECT VERSION()"
	if dialect == config.DriverPostgres {
		query = "SHOW server_version"
	}
	var v string
	if err := db.QueryRowContext(ctx, query).Scan(&v); err != nil {
		return dialect, "", fmt.Errorf("reading the database version: %w", err)
	}
	return dialect, v, nil
}
