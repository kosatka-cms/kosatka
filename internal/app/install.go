package app

import (
	"cmp"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"golang.org/x/crypto/bcrypt"

	"github.com/kosatka-cms/kosatka/internal/app/demoset"
	"github.com/kosatka-cms/kosatka/internal/app/starter"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/role"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/modules"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/translations"
)

// exitInstalledAlready - app install's exit code on a database with a
// site already: a start script tells it from a failure (exit 1) and runs
// app migrate instead.
const exitInstalledAlready = 3

// runInstall is "app install": a new site on an empty database (MySQL or
// PostgreSQL, db.driver) - the schema of the core and of the modules it
// has (-modules, installModules), their install steps (the core's:
// settings, roles, text and date formats), the site name, language and
// first administrator (its generated password is printed once), and the
// starter content: content types, front page, main menu; with -demo also
// demo articles and an About page. Refused on a database that has the
// core already (exitInstalledAlready).
func runInstall(args []string) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	siteName := fs.String("site-name", "", "the site name (required)")
	adminName := fs.String("admin-name", "", "the administrator's username (required)")
	adminEmail := fs.String("admin-email", "", "the administrator's e-mail, also the site e-mail (required)")
	language := fs.String("language", "en", "the site's language: its default language and that of the starter content (en, a built-in language, or one of the translations catalog)")
	translationsURL := fs.String("translations", "", "the translations catalog a -language that isn't built in is downloaded from (default: translations_catalog_url)")
	demo := fs.Bool("demo", false, "add demo content: articles with covers on the front page and an About page in the main menu")
	demoSet := fs.String("demo-set", "", "the demo content's set (implies -demo): "+strings.Join(demoSetNames(), ", ")+"; empty - "+defaultDemoSet)
	theme := fs.String("theme", "", "the site's theme (a directory of themes/); empty - the demo set's, else the starter's")
	moduleList := fs.String("modules", "", "the modules the site has, comma-separated, \"all\" or \"none\" (empty - the standard ones); the others are installed when enabled on the Modules page")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	name, admin, email := strings.TrimSpace(*siteName), strings.TrimSpace(*adminName), strings.TrimSpace(*adminEmail)
	if name == "" || admin == "" || email == "" {
		fmt.Fprintln(os.Stderr, "install: -site-name, -admin-name and -admin-email are required")
		return 2
	}
	cfg := loadConfig()
	db, drv, err := NewEntConn(&cfg.DB)
	if err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	// An installed site first: its container runs install on every start,
	// and the flags of a new site (a demo set's language, say) don't
	// matter to it.
	r := migrations.Runner{DB: db, Dialect: sqldialect.Dialect(cfg.DB.Dialect())}
	installed, err := r.Installed(context.Background(), migrations.CoreSet)
	if err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		return 1
	}
	if installed {
		fmt.Fprintln(os.Stderr, "install: the site is installed already (the core is in schema_migrations); updates go through \"migrate\"")
		return exitInstalledAlready
	}

	lang := strings.ToLower(strings.TrimSpace(*language))
	// A language that isn't built in comes from the translations catalog:
	// checked there before anything is installed, downloaded after the
	// schema, before the starter content written in it.
	var packs *service.InterfaceStrings
	if langs := builtInLanguages(); !slices.Contains(langs, lang) {
		catalogURL := cmp.Or(*translationsURL, cfg.TranslationsCatalogURL)
		if catalogURL == "" {
			fmt.Fprintf(os.Stderr, "install: -language %q isn't built in (%s): give the translations catalog (-translations <url> or translations_catalog_url)\n", lang, strings.Join(langs, ", "))
			return 2
		}
		packs = installTranslations(db, catalogURL)
		index, _, err := packs.Catalog(context.Background(), false)
		if err != nil {
			fmt.Fprintln(os.Stderr, "install: -language:", err)
			return 1
		}
		if _, ok := index.Language(lang); !ok {
			fmt.Fprintf(os.Stderr, "install: unknown -language %q: not built in (%s), not in the translations catalog\n", lang, strings.Join(langs, ", "))
			return 2
		}
	}
	opts := siteOptions{theme: strings.TrimSpace(*theme)}
	var setModules []string
	if *demo || *demoSet != "" {
		opts.demoSet = strings.TrimSpace(*demoSet)
		if opts.demoSet == "" {
			opts.demoSet = defaultDemoSet
		}
		var err error
		if setModules, err = demoSetModules(opts.demoSet, lang); err != nil {
			fmt.Fprintln(os.Stderr, "install: -demo-set:", err)
			return 2
		}
	}
	chosen, err := installModules(*moduleList, setModules)
	if err != nil {
		fmt.Fprintln(os.Stderr, "install: -modules:", err)
		return 2
	}
	if opts.theme != "" {
		if _, err := themes.LoadInfoFor(opts.theme); err != nil {
			fmt.Fprintf(os.Stderr, "install: -theme %q: %v\n", opts.theme, err)
			return 2
		}
	}

	// The demo covers go to the public files' store.
	if err := setupFiles(cfg.Files); err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		return 1
	}
	// In the site's language: the starter's and the demo's nodes get it.
	ctx := i18n.WithLang(context.Background(), lang)
	for _, set := range migrations.Sets() {
		if set.Name != migrations.CoreSet && !chosen[set.Name] {
			fmt.Printf("%s: not installed (enabling it on the Modules page installs it)\n", set.Name)
			continue
		}
		steps, err := r.Install(ctx, set.Name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "install:", set.Name+":", err)
			return 1
		}
		fmt.Printf("%s: installed (%d steps)\n", set.Name, len(steps))
	}
	if packs != nil {
		res, err := packs.Download(ctx, lang)
		if err != nil {
			fmt.Fprintln(os.Stderr, "install: the", lang, "translation:", err)
			fmt.Fprintln(os.Stderr, "install: the database is installed only in part: start again on an empty one")
			return 1
		}
		fmt.Printf("translations: %s installed (%d strings)\n", lang, res.Kept)
	}

	client := generated.NewClient(generated.Driver(drv))
	password, err := createSite(ctx, r, client, name, admin, email, lang, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		if password != "" {
			// The site and its administrator are saved, the modules' rows
			// aren't all there.
			fmt.Printf("install: the administrator %q has the password (shown once): %s\n", admin, password)
		}
		fmt.Fprintln(os.Stderr, "install: the database is installed only in part: start again on an empty one")
		return 1
	}
	fmt.Printf("install: done - %q (%s), administrator %q\n", name, lang, admin)
	fmt.Printf("install: the administrator's password (shown once, change it after signing in): %s\n", password)
	if strings.TrimSpace(cfg.SecretsKey) == "" {
		key, err := newSecretsKey()
		if err != nil {
			fmt.Fprintln(os.Stderr, "install:", err)
			return 1
		}
		fmt.Printf("install: SECRETS_KEY is not set - put this one in the site's env file and keep a copy with the backups (it seals the keys saved in the admin and the two-factor secrets): SECRETS_KEY=%s\n", key)
	}
	return 0
}

// newSecretsKey - a random key for SECRETS_KEY: 32 bytes, in hex.
func newSecretsKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// installModules - the modules app install installs and enables, by its
// -modules value: "" - the standard ones (entity.Module.InstallByDefault)
// and the ones the demo set has content for (demo), "all", "none", or
// their names separated by commas; with the modules they require. An
// unknown name is an error.
func installModules(value string, demo []string) (map[string]bool, error) {
	out := map[string]bool{}
	switch v := strings.TrimSpace(value); v {
	case "":
		for _, m := range entity.Modules {
			if m.InstallByDefault {
				out[m.Name] = true
			}
		}
		for _, name := range demo {
			if _, ok := entity.ModuleByName(name); !ok {
				return nil, fmt.Errorf("the demo set's module %q: no such module", name)
			}
			out[name] = true
		}
	case "all":
		for _, m := range entity.Modules {
			out[m.Name] = true
		}
	case "none":
	default:
		for _, name := range strings.Split(v, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, ok := entity.ModuleByName(name); !ok {
				names := make([]string, 0, len(entity.Modules))
				for _, m := range entity.Modules {
					names = append(names, m.Name)
				}
				return nil, fmt.Errorf("unknown module %q (available: %s)", name, strings.Join(names, ", "))
			}
			out[name] = true
		}
	}
	for added := true; added; {
		added = false
		for _, m := range entity.Modules {
			if !out[m.Name] {
				continue
			}
			for _, req := range m.Requires {
				if !out[req] {
					out[req], added = true, true
				}
			}
		}
	}
	return out, nil
}

// installedModules - the modules whose set is installed, by name.
func installedModules(ctx context.Context, r migrations.Runner) (map[string]bool, error) {
	out := map[string]bool{}
	for _, m := range entity.Modules {
		ok, err := r.Installed(ctx, m.Name)
		if err != nil {
			return nil, fmt.Errorf("module %s: %w", m.Name, err)
		}
		out[m.Name] = ok
	}
	return out, nil
}

// installTranslations - the service that downloads a language from the
// translations catalog at catalogURL into db (as the Languages tab does).
func installTranslations(db *sql.DB, catalogURL string) *service.InterfaceStrings {
	sources, _ := i18n.ParseSources(translations.Sources)
	s := service.NewInterfaceStrings(database.NewInterfaceStringRepository(db), sources)
	s.SetPacks(database.NewInterfacePackRepository(db), catalogURL, builtInLanguages())
	return s
}

// labeler - the starter content's labels in the site's language (they're
// data, written once; the site renames them in the admin): the language's
// downloaded translation in client's database and the catalogs of the
// modules from outside the engine - a module's install step labels its
// starter rows too (localizeModuleRows).
func labeler(ctx context.Context, client *generated.Client, lang string) (func(string) string, error) {
	if lang == i18n.SourceLanguage {
		return func(s string) string { return s }, nil
	}
	tr, err := i18n.Load(modules.Catalogs()...)
	if err != nil {
		return nil, err
	}
	rows, err := database.NewInterfacePackRepositoryOn(client).Strings(ctx)
	if err != nil {
		return nil, err
	}
	var pack []i18n.Entry
	for _, r := range rows {
		if r.Lang == lang {
			pack = append(pack, i18n.Entry{Lang: r.Lang, Context: r.Context, Source: r.Source, Text: r.Text, Forms: r.Forms})
		}
	}
	tr = tr.WithPacks(pack)
	return func(s string) string { return tr.Text(lang, "", s) }, nil
}

// siteOptions - what app install makes of a new site beyond its name,
// language and administrator: its demo content's set ("" - none) and
// theme ("" - the demo set's, else the starter's).
type siteOptions struct {
	demoSet string
	theme   string
}

// createSite names the site, sets its language and theme, creates its
// administrator with a generated password, which it returns, and the
// starter content (with a demo set - and the demo content in lang, the
// installed modules' too) - all of it in one transaction. Then the
// installed modules' rows are localized and their blocks placed; the
// installed modules are on, the others off (setModuleStates). An error
// after the transaction comes with the password: the administrator is
// there.
func createSite(ctx context.Context, r migrations.Runner, client *generated.Client, siteName, adminName, adminEmail, lang string, opts siteOptions) (string, error) {
	label, err := labeler(ctx, client, lang)
	if err != nil {
		return "", err
	}
	installed, err := installedModules(ctx, r)
	if err != nil {
		return "", err
	}
	set, err := starter.Load()
	if err != nil {
		return "", err
	}
	var demoSet *demoset.Set
	if opts.demoSet != "" && opts.demoSet != defaultDemoSet {
		if demoSet, err = demoset.Load(opts.demoSet, lang); err != nil {
			return "", err
		}
		if demoSet.Site.Theme != "" {
			set.Site.Theme = demoSet.Site.Theme
		}
	}
	if opts.theme != "" {
		set.Site.Theme = opts.theme
	}
	secret := make([]byte, 18)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	password := base64.RawURLEncoding.EncodeToString(secret)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return "", err
	}
	c := tx.Client()
	demo := installDemo{set: opts.demoSet, lang: lang, data: demoSet}
	err = func() error {
		if err := c.SiteInformation.UpdateOneID(1).
			SetSiteName(siteName).SetSiteEmail(adminEmail).SetDefaultLanguage(lang).
			SetLogo(set.Site.Logo).SetTheme(set.Site.Theme).
			Exec(ctx); err != nil {
			return fmt.Errorf("site information: %w", err)
		}
		adminRole, err := c.Role.Query().Where(role.Name(entity.RoleAdmin)).Only(ctx)
		if err != nil {
			return fmt.Errorf("the %s role: %w", entity.RoleAdmin, err)
		}
		admin, err := c.User.Create().
			SetUsername(adminName).
			SetSlug(entity.UserSlug(adminName)).
			SetEmail(entity.NormalizeEmail(adminEmail)).
			SetPasswordHash(string(hash)).
			SetStatus(entity.UserStatusActive).
			SetEmailVerified(true).
			SetRoleID(adminRole.ID).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("administrator: %w", err)
		}
		return createStarterContent(ctx, c, set, admin.ID, label, demo, installed, string(r.Dialect))
	}()
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	for _, m := range entity.Modules {
		if !installed[m.Name] {
			continue
		}
		if err := localizeModuleRows(ctx, r, m, label); err != nil {
			return password, err
		}
		// The module's blocks in the site's theme (set above).
		if err := r.PlaceModuleBlocks(ctx, m.Name); err != nil {
			return password, err
		}
	}
	if err := setModuleStates(ctx, client, installed); err != nil {
		return password, err
	}
	// The starter and demo content, written past the repositories: its
	// words for the search.
	if _, err := database.RebuildSearchIndex(ctx, client); err != nil {
		return password, err
	}
	return password, nil
}

// setModuleStates records each module's state on a new site: on when it
// is installed, off when it isn't - a state for every module, as a module
// without one is on (entity.Module.OffByDefault aside), tables or not.
func setModuleStates(ctx context.Context, client *generated.Client, installed map[string]bool) error {
	now := time.Now()
	for _, m := range entity.Modules {
		if err := client.Module.Create().SetID(m.Name).SetEnabled(installed[m.Name]).SetUpdatedAt(now).Exec(ctx); err != nil {
			return fmt.Errorf("module %s: %w", m.Name, err)
		}
	}
	return nil
}

// installDemo - the demo content of a new site: its set ("" - none;
// defaultDemoSet - the blog of createDemoContent, else data, a set of
// demoset) in lang.
type installDemo struct {
	set, lang string
	data      *demoset.Set
}

// createStarterContent sets up what a new site starts with, through c
// (createSite's transaction): the core's rows in the site's language, the starter set
// (starter.yaml: content types, Views, menus…) and the blocks of its
// theme; with a demo set - the demo content (createDemoContent, or the
// set's data), the installed modules' with the blog's.
func createStarterContent(ctx context.Context, c *generated.Client, set *starter.Set, createdBy int, label func(string) string, demo installDemo, installed map[string]bool, dialect string) error {
	var created starter.Result
	var err error
	steps := []struct {
		name string
		run  func() error
	}{
		{"labels", func() error { return localizeCoreRows(ctx, c, label) }},
		{"starter set", func() error { created, err = set.Apply(ctx, c, createdBy, label); return err }},
		{"theme blocks", func() error { return placeThemeBlocks(ctx, c, set.Site.Theme) }},
	}
	switch {
	case demo.data != nil:
		steps = append(steps, struct {
			name string
			run  func() error
		}{"demo set " + demo.set, func() error {
			_, err := demo.data.Apply(ctx, demoset.Target{Client: c, CreatedBy: createdBy, Theme: set.Site.Theme, Now: time.Now(),
				Lang: demo.lang, Dialect: dialect, Installed: installed})
			return err
		}})
	case demo.set != "":
		steps = append(steps, struct {
			name string
			run  func() error
		}{"demo content", func() error {
			return createDemoContent(ctx, c, createdBy, demoStarter(set, created), demo.lang, installed, dialect)
		}})
	}
	for _, s := range steps {
		if err := s.run(); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}
	return nil
}

// readLabels - the label column's values by row id.
func readLabels(ctx context.Context, db *sql.DB, lc entity.ModuleLabelColumn) (map[int]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, "+lc.Column+" FROM "+lc.Table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	labels := map[int]string{}
	for rows.Next() {
		var id int
		var v string
		if err := rows.Scan(&id, &v); err != nil {
			return nil, err
		}
		labels[id] = v
	}
	return labels, rows.Err()
}

// moduleInstaller - what installs a module enabled for the first time on
// the Modules page: its tables (its migrations) and install step, then
// finishModuleInstall; an installed module is left as it is - its
// updates go through "app migrate".
func moduleInstaller(r migrations.Runner, siteLanguage func(context.Context) (string, error), logger *slog.Logger) func(context.Context, string) error {
	return func(ctx context.Context, module string) error {
		installed, err := r.Installed(ctx, module)
		if err != nil || installed {
			return err
		}
		if _, err := r.Install(ctx, module); err != nil {
			return err
		}
		finishModuleInstall(ctx, r, module, siteLanguage, logger)
		return nil
	}
}

// finishModuleInstall - what installing a module does after its set ran,
// on the Modules page and in "app migrate": its blocks in the site's
// theme (PlaceModuleBlocks), then the labels its install step wrote in the
// site's language (siteLanguage) as app install puts them. A block not
// placed or labels left in English are logged, not a failure: the module
// is installed by then.
func finishModuleInstall(ctx context.Context, r migrations.Runner, module string, siteLanguage func(context.Context) (string, error), logger *slog.Logger) {
	service.LogFallback(logger, r.PlaceModuleBlocks(ctx, module), "error placing a module's blocks", "module", module)
	m, _ := entity.ModuleByName(module)
	if len(m.LabelColumns) == 0 {
		return
	}
	lang, err := siteLanguage(ctx)
	if err != nil {
		service.LogFallback(logger, err, "error loading the site language for a module's labels", "module", module)
		return
	}
	client := generated.NewClient(generated.Driver(entsql.OpenDB(string(r.Dialect), r.DB)))
	label, err := labeler(ctx, client, lang)
	if err == nil {
		err = localizeModuleRows(ctx, r, m, label)
	}
	service.LogFallback(logger, err, "error translating a module's labels", "module", module)
}

// localizeModuleRows puts the labels module m's install step wrote in
// English (entity.Module.LabelColumns) in the site's language: on a new
// site, and when the Modules page installs it later (wiring's
// installModule).
func localizeModuleRows(ctx context.Context, r migrations.Runner, m entity.Module, label func(string) string) error {
	for _, lc := range m.LabelColumns {
		labels, err := readLabels(ctx, r.DB, lc)
		if err != nil {
			return fmt.Errorf("%s labels: %w", lc.Table, err)
		}
		for id, v := range labels {
			if t := label(v); t != v {
				if _, err := r.DB.ExecContext(ctx, r.Dialect.Rebind("UPDATE "+lc.Table+" SET "+lc.Column+" = ? WHERE id = ?"), t, id); err != nil {
					return fmt.Errorf("%s labels: %w", lc.Table, err)
				}
			}
		}
	}
	return nil
}

// localizeCoreRows puts the labels the install steps wrote in English
// (text and date formats, image styles, role descriptions)
// in the site's language.
func localizeCoreRows(ctx context.Context, c *generated.Client, label func(string) string) error {
	formats, err := c.TextFormatDef.Query().All(ctx)
	if err != nil {
		return err
	}
	for _, f := range formats {
		if err := c.TextFormatDef.UpdateOne(f).SetLabel(label(f.Label)).Exec(ctx); err != nil {
			return err
		}
	}
	dates, err := c.DateFormat.Query().All(ctx)
	if err != nil {
		return err
	}
	for _, d := range dates {
		if err := c.DateFormat.UpdateOne(d).SetLabel(label(d.Label)).Exec(ctx); err != nil {
			return err
		}
	}
	styles, err := c.ImageStyle.Query().All(ctx)
	if err != nil {
		return err
	}
	for _, st := range styles {
		if err := c.ImageStyle.UpdateOne(st).SetLabel(label(st.Label)).Exec(ctx); err != nil {
			return err
		}
	}
	roles, err := c.Role.Query().All(ctx)
	if err != nil {
		return err
	}
	for _, r := range roles {
		if err := c.Role.UpdateOne(r).SetDescription(label(r.Description)).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// placeThemeBlocks places the blocks the theme comes with (theme.yaml
// "blocks", themes.PlaceBlocks; the copyright block in the footer's
// bottom line): blocks, so the site edits or removes them.
func placeThemeBlocks(ctx context.Context, c *generated.Client, theme string) error {
	info, err := themes.LoadInfoFor(theme)
	if err != nil {
		return err
	}
	return themes.PlaceBlocks(ctx, database.NewBlockRepository(c), entity.ThemeKey(theme), info.Blocks)
}
