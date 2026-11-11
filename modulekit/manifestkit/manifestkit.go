// Package manifestkit - the part of the module SDK (modulekit) a module's
// manifest and own ent schema use: the module's description and its
// schema's registration. Light on purpose - it imports the core's
// entities and migrations only - so the manifests stay importable from
// the core's own tests (they register the modules), where the rest of
// modulekit, built on the core's services, would be an import cycle.
package manifestkit

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/imagestyle"
	entrole "github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/role"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/textformatdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
)

// The module's description (its manifest's Module()): the core's types.
type (
	// Module - a module's registry entry.
	Module = entity.Module
	// ModuleData - a table of the module's that the Modules page counts.
	ModuleData = entity.ModuleData
	// ModuleMenuItem - its page in the admin toolbar.
	ModuleMenuItem = entity.ModuleMenuItem
	// ModulePluginBlock - a plugin block it provides.
	ModulePluginBlock = entity.ModulePluginBlock
	// ModuleCaptchaForm - its form that can show the CAPTCHA.
	ModuleCaptchaForm = entity.ModuleCaptchaForm
	// AdminMenuItem - an admin toolbar item.
	AdminMenuItem = entity.AdminMenuItem
	// PluginBlockID - a plugin block's id.
	PluginBlockID = entity.PluginBlockID
	// ModulePermission - a permission it adds.
	ModulePermission = entity.ModulePermission
	// ModuleFileSource - its texts that can link to users' files.
	ModuleFileSource = entity.ModuleFileSource
	// ModuleNodeType - a content type it provides.
	ModuleNodeType = entity.ModuleNodeType
	// ContentType - a content type's machine name.
	ContentType = entity.ContentType
	// ModuleLabelColumn - its install step's labels app install translates.
	ModuleLabelColumn = entity.ModuleLabelColumn
	// ModuleNodeFiles - its tables of files attached to nodes.
	ModuleNodeFiles = entity.ModuleNodeFiles
	// CommentMode - a material's comments: open, closed, hidden.
	CommentMode = entity.CommentMode
)

// The comment modes (a demo node's, a content type's default).
const (
	CommentModeOpen   = entity.CommentModeOpen
	CommentModeClosed = entity.CommentModeClosed
	CommentModeHidden = entity.CommentModeHidden
)

// RoleUser - the role every signed-in user has (a module's install or
// data step grants it the module's permissions).
const RoleUser = entity.RoleUser

// DialectPostgres - InstallEnv.Dialect of a PostgreSQL site (else MySQL).
const DialectPostgres = sqldialect.Postgres

// PluginBlockTargetKey - the plugin config key of the thing a block shows.
const PluginBlockTargetKey = entity.PluginBlockTargetKey

// BlockVisibilityHomepage - a block placement shown on the front page
// only.
const BlockVisibilityHomepage = entity.BlockVisibilityHomepage

// The admin toolbar's sections a module's page joins (ModuleMenuItem's
// Section); AdminGroupModules - the Configuration section's group of the
// modules' settings pages, AdminGroupTexts - of the texts' and files',
// AdminGroupSite - of the site's own
// (AdminMenuItem.Group: the item goes after its group's last one).
var (
	AdminSectionContent       = entity.AdminSectionContent
	AdminSectionStructure     = entity.AdminSectionStructure
	AdminSectionConfiguration = entity.AdminSectionConfiguration
	AdminSectionReports       = entity.AdminSectionReports
	AdminGroupModules         = entity.AdminGroupModules
	AdminGroupTexts           = entity.AdminGroupTexts
	AdminGroupSite            = entity.AdminGroupSite
)

// The groups of the Modules page a module picks for its Category (Module
// .Category; none or another - "Other").
const (
	CategoryContent        = entity.ModuleCategoryContent
	CategoryMedia          = entity.ModuleCategoryMedia
	CategoryCommunity      = entity.ModuleCategoryCommunity
	CategoryAdministration = entity.ModuleCategoryAdministration
	CategoryIntegrations   = entity.ModuleCategoryIntegrations
	CategoryOther          = entity.ModuleCategoryOther
)

// Dialect - a site's database (DialectPostgres, else MySQL): its Rebind
// puts a query's ? placeholders in the database's own.
type Dialect = sqldialect.Dialect

// InstallEnv - what a module's install or data step works with: the
// database (DB, Dialect) - its own client opens a driver on it (Driver);
// the core's rows it changes through this package's helpers.
type InstallEnv struct {
	DB      *sql.DB
	Dialect Dialect
}

// envOf - the core's step env as a module's.
func envOf(e migrations.Env) InstallEnv { return InstallEnv{DB: e.DB, Dialect: e.Dialect} }

// core - env as the core's step env: its client on the step's database.
func (env InstallEnv) core() migrations.Env {
	return migrations.Env{DB: env.DB, Dialect: env.Dialect, Client: generated.NewClient(generated.Driver(Driver(env)))}
}

// Update - a module's data step: Go code moving or fixing its data, at
// its Version among its migration files (a timestamp).
type Update struct {
	Version     string
	Description string
	Run         func(ctx context.Context, env InstallEnv) error
}

// RegisterInstall sets the module's install step: it runs once, when the
// module is installed, after its tables are created (its RegisterSteps).
func RegisterInstall(set string, run func(ctx context.Context, env InstallEnv) error) {
	migrations.RegisterInstall(set, func(ctx context.Context, e migrations.Env) error { return run(ctx, envOf(e)) })
}

// RegisterUpdate adds a data step of the module (its RegisterSteps).
func RegisterUpdate(set string, u Update) {
	migrations.RegisterUpdate(set, migrations.Update{Version: u.Version, Description: u.Description,
		Run: func(ctx context.Context, e migrations.Env) error { return u.Run(ctx, envOf(e)) }})
}

// PlaceModuleBlocks places the module's plugin blocks that name a region
// (ModulePluginBlock.Region) in the site's active theme, as installing it
// does: a data step for sites that had the module before the block.
func PlaceModuleBlocks(ctx context.Context, env InstallEnv, module string) error {
	return migrations.PlaceModuleBlocks(ctx, env.core(), module)
}

// Driver - the step's database as an ent driver, for the module's own
// client (its generated NewClient(Driver(…))).
func Driver(env InstallEnv) dialect.Driver { return entsql.OpenDB(string(env.Dialect), env.DB) }

// ImageStyle - an image style a module's pages use (its install step
// makes it, EnsureImageStyle).
type ImageStyle struct {
	Name, Label, Mode, Anchor string
	Width, Height             int
}

// EnsureImageStyle makes the core's image style st unless one of its
// name is there (kept by an uninstall, or made by the site): left as it
// is then.
func EnsureImageStyle(ctx context.Context, env InstallEnv, st ImageStyle) error {
	client := env.core().Client
	exists, err := client.ImageStyle.Query().Where(imagestyle.Name(st.Name)).Exist(ctx)
	if err != nil || exists {
		return err
	}
	now := time.Now()
	if err := client.ImageStyle.Create().SetName(st.Name).SetLabel(st.Label).
		SetWidth(st.Width).SetHeight(st.Height).SetMode(st.Mode).SetAnchor(st.Anchor).
		SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
		return fmt.Errorf("image style %s: %w", st.Name, err)
	}
	return nil
}

// FullAccessRoles - the names of the roles with full access (on a usual
// site, the administrator's), by name: an install step names them where
// its settings give something to roles, so what only they may do shows
// in the settings as theirs.
func FullAccessRoles(ctx context.Context, env InstallEnv) ([]string, error) {
	rows, err := env.core().Client.Role.Query().Order(entrole.ByName()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("the roles: %w", err)
	}
	var out []string
	for _, r := range rows {
		if (entity.Role{Permissions: r.Permissions}).Grants(entity.PermFullAccess) {
			out = append(out, r.Name)
		}
	}
	return out, nil
}

// GrantPermission gives the role named role (RoleUser - every signed-in
// user's) the permission perm, keeping its others - an install step's
// default of who may use the module. A site without the role - nothing.
func GrantPermission(ctx context.Context, env InstallEnv, role, perm string) error {
	client := env.core().Client
	r, err := client.Role.Query().Where(entrole.Name(role)).Only(ctx)
	if generated.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("the %s role: %w", role, err)
	}
	perms := maps.Clone(r.Permissions)
	if perms == nil {
		perms = map[string]any{}
	}
	perms[perm] = true
	if err := client.Role.UpdateOneID(r.ID).SetPermissions(perms).Exec(ctx); err != nil {
		return fmt.Errorf("granting %q to the %s role: %w", perm, role, err)
	}
	return nil
}

// EnableTextFilter turns the module's text filter name
// (modulekit.AddTextFilter) on for the text formats with the visual
// editor - its install step: the formats a site writes in with links and
// pictures. A format with it on already is left as it is. (A running site
// reads the formats' filters again after installing a module.)
func EnableTextFilter(ctx context.Context, env InstallEnv, name string) error {
	client := env.core().Client
	formats, err := client.TextFormatDef.Query().Where(textformatdef.HasRichEditor(true)).All(ctx)
	if err != nil {
		return fmt.Errorf("text formats: %w", err)
	}
	for _, f := range formats {
		names := strings.Fields(f.Filters)
		if slices.Contains(names, name) {
			continue
		}
		names = append(names, name)
		if err := client.TextFormatDef.UpdateOneID(f.ID).SetFilters(strings.Join(names, " ")).Exec(ctx); err != nil {
			return fmt.Errorf("text format %s: %w", f.MachineName, err)
		}
	}
	return nil
}

// Schema - the module's own ent schema (its ent/, the conventions in
// modulekit/schemakit): its tables and migration files.
type Schema = migrations.Schema

// RegisterSchema registers the module's own schema for its set (its
// manifest's RegisterSteps): its tables and migrations are those.
func RegisterSchema(set string, s Schema) { migrations.RegisterSchema(set, s) }

// external - a module from outside the engine (modulekit.Register): its
// description and its steps.
type external struct {
	module Module
	steps  func()
}

// registered - the modules from outside the engine, in the order they
// registered.
var registered []external

// Register records a module from outside the engine: modulekit.Register
// calls it from the module's init, before the engine reads its modules
// (manifests.Register adds them after its own); steps - the module's
// RegisterSteps (its schema, install, update and demo steps), nil - none.
func Register(m Module, steps func()) {
	registered = append(registered, external{m, steps})
}

// Registered - the modules from outside the engine and their steps.
func Registered() ([]Module, []func()) {
	mods := make([]Module, 0, len(registered))
	var steps []func()
	for _, e := range registered {
		mods = append(mods, e.module)
		if e.steps != nil {
			steps = append(steps, e.steps)
		}
	}
	return mods, steps
}
