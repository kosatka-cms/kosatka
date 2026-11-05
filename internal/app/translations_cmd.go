package app

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/i18npack"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/translations"
)

// translationsUsage - "app translations" without a known command.
const translationsUsage = `usage:
  app translations list [-catalog <url>]            the interface's languages: built in, downloaded (their version, an update), the catalog's others
  app translations add <lang> [-catalog <url>]      download a language from the translations catalog and install it
  app translations update [<lang>] [-catalog <url>] the downloaded languages (or one) moved to the catalog's version
  app translations remove <lang>                    remove a downloaded language (not the site's interface language)
  app translations index [-src .] [-out dist] [-sources <file,…>] [-complete <lang,…>]
                                                    build a translations catalog from a translations repository (<lang>/language.yaml, <lang>/*.json)
the catalog: -catalog (an index.json's https address or a file), else translations_catalog_url / TRANSLATIONS_CATALOG_URL;
a running site takes a change up when it starts again (on the admin's Interface translations page - at once)`

// translationsTimeout - the limit of one command's downloads.
const translationsTimeout = 5 * time.Minute

// runTranslations is "app translations …": the interface's languages
// downloaded from the translations catalog (service.InterfaceStrings, as
// the admin's Interface translations page), for deploy scripts and sites
// without the admin.
func runTranslations(args []string) int {
	return translationsCommand(args, os.Stdout, os.Stderr)
}

func translationsCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, translationsUsage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	if cmd == "index" {
		return translationsIndex(rest, stdout, stderr)
	}
	fs := flag.NewFlagSet("translations "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	catalog := fs.String("catalog", "", "the translations catalog: an index.json's address or file (default: translations_catalog_url)")
	// The flags may come after the language too.
	var positional []string
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	want := map[string][2]int{"list": {0, 0}, "add": {1, 1}, "update": {0, 1}, "remove": {1, 1}}
	n, known := want[cmd]
	if !known || len(positional) < n[0] || len(positional) > n[1] {
		_, _ = fmt.Fprintln(stderr, translationsUsage)
		return 2
	}

	cfg := loadConfig()
	db, _, err := NewEntConn(&cfg.DB)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "translations:", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), translationsTimeout)
	defer cancel()
	catalogURL := cfg.TranslationsCatalogURL
	if *catalog != "" {
		catalogURL = *catalog
	}
	s, err := translationsService(ctx, db, cfg, catalogURL, stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "translations:", err)
		return 1
	}
	c := translationsCLI{s: s, stdout: stdout, stderr: stderr}
	switch cmd {
	case "list":
		return c.list(ctx)
	case "add":
		return c.download(ctx, positional[0])
	case "update":
		if len(positional) == 1 {
			return c.update(ctx, positional[0])
		}
		return c.update(ctx, "")
	default:
		return c.remove(ctx, positional[0])
	}
}

// translationsService - the interface strings' service of the site as it
// boots: its theme (whose catalogs' strings a pack may translate) and
// interface language (not removed), the module states, the downloaded
// languages in the translator (the list's figures); the catalog at
// catalogURL.
func translationsService(ctx context.Context, db *sql.DB, cfg *config.Config, catalogURL string, stderr io.Writer) (*service.InterfaceStrings, error) {
	sources, err := i18n.ParseSources(translations.Sources)
	if err != nil {
		return nil, err
	}
	info, err := database.NewSiteInformationRepository(db).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the site information: %w", err)
	}
	paths.SetTheme(cfg.Theme)
	if info.Theme != nil {
		paths.SetTheme(*info.Theme)
	}
	i18n.SetDefault(info.DefaultLanguage)
	if err := service.LoadModules(ctx, database.NewModuleRepository(db)); err != nil {
		return nil, err
	}
	s := service.NewInterfaceStrings(database.NewInterfaceStringRepository(db), sources)
	rt := &themeRuntime{logger: slog.New(slog.NewTextHandler(stderr, nil)), siteStrings: s.Entries, packStrings: s.PackEntries}
	s.SetThemeSources(rt.themeSources)
	s.SetPacks(database.NewInterfacePackRepository(db), catalogURL, builtInLanguages())
	tr, err := rt.translator(paths.Theme())
	if err != nil {
		return nil, err
	}
	i18n.SetTranslator(tr)
	return s, nil
}

// translationsCLI - the commands' output.
type translationsCLI struct {
	s              *service.InterfaceStrings
	stdout, stderr io.Writer
}

// fail prints why lang's command failed: 1.
func (c translationsCLI) fail(lang string, err error) int {
	msg := err.Error()
	switch {
	case errors.Is(err, service.ErrNoTranslationsCatalog):
		msg = "no translations catalog: -catalog <url> or translations_catalog_url / TRANSLATIONS_CATALOG_URL"
	case errors.Is(err, service.ErrUnknownPack):
		msg = "the catalog has no " + lang + " translation to download (or the language is built in)"
	case errors.Is(err, service.ErrPackChecksum):
		msg = lang + ": the file isn't what the catalog says (its checksum differs): nothing was installed"
	case errors.Is(err, service.ErrPackInUse):
		msg = lang + " is the site's interface language: choose another one in Site information first"
	case errors.Is(err, service.ErrPackNotInstalled):
		msg = lang + " isn't a downloaded language"
	}
	_, _ = fmt.Fprintln(c.stderr, "translations:", msg)
	return 1
}

// list prints the languages: code, name, state, version, how much of the
// site's strings each translates.
func (c translationsCLI) list(ctx context.Context) int {
	page, err := c.s.LanguagesPage(ctx, true)
	if err != nil {
		return c.fail("", err)
	}
	switch {
	case errors.Is(page.CatalogErr, service.ErrNoTranslationsCatalog):
		_, _ = fmt.Fprintln(c.stderr, "no translations catalog: only the built-in languages (-catalog <url> or translations_catalog_url)")
	case page.CatalogErr != nil:
		_, _ = fmt.Fprintln(c.stderr, "translations: the catalog couldn't be read:", page.CatalogErr)
	}
	for _, l := range page.Languages {
		version := "-"
		switch {
		case l.State == service.PackUpdate:
			version = l.Pack.Version + " -> " + l.Catalog.Version
		case l.Pack != nil:
			version = l.Pack.Version
		case l.Catalog != nil:
			version = l.Catalog.Version
		}
		translated := "the source strings"
		if !l.Source {
			translated = fmt.Sprintf("%d%% (%d of %d)", l.Percent(), l.Translated, l.Total)
			if l.Catalog != nil && l.Catalog.Draft {
				translated += ", a draft"
			}
		}
		_, _ = fmt.Fprintf(c.stdout, "%-8s %-20s %-10s %-16s %s\n", l.Code, l.Name, l.State, version, translated)
	}
	return 0
}

// download installs (or updates) lang from the catalog.
func (c translationsCLI) download(ctx context.Context, lang string) int {
	res, err := c.s.Download(ctx, lang)
	if err != nil {
		return c.fail(lang, err)
	}
	done := "installed"
	if res.Updated {
		done = "updated"
	}
	_, _ = fmt.Fprintf(c.stdout, "%s: %s, %d strings", lang, done, res.Kept)
	if res.Dropped > 0 {
		_, _ = fmt.Fprintf(c.stdout, " (%d of the file left out: not strings of the site, or failing the checks of a translation)", res.Dropped)
	}
	_, _ = fmt.Fprintln(c.stdout)
	return 0
}

// update moves lang ("" - every downloaded language with an update) to
// the catalog's version.
func (c translationsCLI) update(ctx context.Context, lang string) int {
	page, err := c.s.LanguagesPage(ctx, true)
	if err != nil {
		return c.fail(lang, err)
	}
	if page.CatalogErr != nil {
		return c.fail(lang, page.CatalogErr)
	}
	updated := 0
	for _, l := range page.Languages {
		if (lang != "" && l.Code != lang) || l.Pack == nil {
			continue
		}
		if l.State != service.PackUpdate {
			if lang != "" {
				_, _ = fmt.Fprintf(c.stdout, "%s: up to date (%s)\n", lang, l.Pack.Version)
				updated++
			}
			continue
		}
		if code := c.download(ctx, l.Code); code != 0 {
			return code
		}
		updated++
	}
	if updated == 0 {
		if lang != "" {
			_, _ = fmt.Fprintln(c.stderr, "translations:", lang, "isn't a downloaded language (app translations add "+lang+")")
			return 1
		}
		_, _ = fmt.Fprintln(c.stdout, "the downloaded languages are up to date")
	}
	return 0
}

// remove deletes lang's downloaded translation.
func (c translationsCLI) remove(ctx context.Context, lang string) int {
	if err := c.s.DeletePack(ctx, lang); err != nil {
		return c.fail(lang, err)
	}
	_, _ = fmt.Fprintf(c.stdout, "%s: removed (the site's own changes of its strings stay)\n", lang)
	return 0
}

// translationsIndex is "app translations index": a translations catalog
// built from a translations repository (i18npack) - no site needed. The
// strings are checked against the code's: the binary's (the engine's
// strings.json), or the -sources files (the engine's and the modules'
// from their own repositories); a string that can't stand for its source,
// or a -complete language missing one, fails it.
func translationsIndex(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("translations index", flag.ContinueOnError)
	fs.SetOutput(stderr)
	src := fs.String("src", ".", "the translations repository: <lang>/language.yaml and <lang>/<component>.json")
	out := fs.String("out", "dist", "where the catalog goes: index.json and <lang>.json")
	sourceFiles := fs.String("sources", "", "the code's strings, comma-separated strings.json files (default: the binary's)")
	complete := fs.String("complete", "", "languages that must translate every string, comma-separated")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, translationsUsage)
		return 2
	}
	fail := func(err error) int {
		_, _ = fmt.Fprintln(stderr, "translations index:", err)
		return 1
	}
	sources, err := indexSources(*sourceFiles)
	if err != nil {
		return fail(err)
	}
	langs, problems, err := i18npack.Build(os.DirFS(*src), sources)
	if err != nil {
		return fail(err)
	}
	for _, p := range problems {
		_, _ = fmt.Fprintln(stderr, "translations index:", p.Error())
	}
	failed := len(problems) > 0
	required := strings.Split(*complete, ",")
	for _, l := range langs {
		line := fmt.Sprintf("%s %s: %d of %d strings", l.Code, l.Version, len(l.Entries), len(l.Entries)+len(l.Missing))
		if len(l.Stale) > 0 {
			line += fmt.Sprintf(", %d stale left out", len(l.Stale))
		}
		if l.Draft {
			line += " (a draft)"
		}
		_, _ = fmt.Fprintln(stdout, line)
		if slices.Contains(required, l.Code) && len(l.Missing) > 0 {
			failed = true
			_, _ = fmt.Fprintf(stderr, "translations index: %s must be complete; %d strings missing:\n", l.Code, len(l.Missing))
			for i, m := range l.Missing {
				if i == 20 {
					_, _ = fmt.Fprintf(stderr, "  … and %d more\n", len(l.Missing)-20)
					break
				}
				_, _ = fmt.Fprintf(stderr, "  %q\n", m.Source)
			}
		}
	}
	if failed {
		return 1
	}
	if _, err := i18npack.Write(*out, langs, time.Now()); err != nil {
		return fail(err)
	}
	_, _ = fmt.Fprintf(stdout, "%s: index.json and %d languages\n", *out, len(langs))
	return 0
}

// indexSources - the code's strings: the files' (comma-separated
// strings.json), each string once; none given - the binary's.
func indexSources(files string) ([]i18n.Source, error) {
	if files == "" {
		return i18n.ParseSources(translations.Sources)
	}
	seen := map[string]bool{}
	var out []i18n.Source
	for _, f := range strings.Split(files, ",") {
		b, err := os.ReadFile(strings.TrimSpace(f))
		if err != nil {
			return nil, err
		}
		list, err := i18n.ParseSources(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		for _, s := range list {
			if k := s.Context + "\x04" + s.Source; !seen[k] {
				seen[k] = true
				out = append(out, s)
			}
		}
	}
	return out, nil
}
