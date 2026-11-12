package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/internal/scaffold"
	"github.com/kosatka-cms/kosatka/internal/themeinstall"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/translations"
)

// themeUsage - "app theme" without a known command.
const themeUsage = `usage:
  app theme new <name> [-label l] [-dir d]    start a theme: manifest, stylesheet, translations, README (in themes/<name>,
                                              or -dir: ../kosatka-theme-<name> for a repository of its own)
  app theme list [-updates]                   the themes: installed ones with their version and source
  app theme add <repository>[@v1.2.0] [-name n] install a theme from its git repository (its newest release by default)
  app theme update <name>[@v1.2.0]            move an installed theme to a release (its newest by default)
  app theme remove <name>                     delete an installed theme
  app theme verify                            check the installed themes are what themes/themes.lock says
  app theme check [dir]                       check a theme before its release (default: the current directory)
  app theme search [words] [-catalog url]     the themes' catalog (default: $THEMES_CATALOG_URL)
  app theme index -list themes.txt -out index.json
                                              build the catalog from a list of the themes' repositories
                                              (GitHub's stars with $GITHUB_TOKEN, else the anonymous limit)`

// themeNetworkTimeout - the limit of one command's git work.
const themeNetworkTimeout = 2 * time.Minute

// runTheme is "app theme …": the themes kept in their own git
// repositories (internal/themeinstall) - no database or config needed;
// run from the project's directory (the one with web/ and themes/).
func runTheme(args []string) int {
	return themeCommand(args, os.Stdout, os.Stderr)
}

func themeCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, themeUsage)
		return 2
	}
	in := themeinstall.Installer{Log: stdout}
	ctx, cancel := context.WithTimeout(context.Background(), themeNetworkTimeout)
	defer cancel()
	fail := func(err error) int {
		_, _ = fmt.Fprintln(stderr, "theme:", err)
		return 1
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet("theme "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	switch cmd {
	case "list":
		updates := fs.Bool("updates", false, "also ask each installed theme's repository for a newer release")
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		return themeList(ctx, in, *updates, stdout, stderr)
	case "new":
		label := fs.String("label", "", "its name on the Appearance page (default: from <name>, sea-breeze - Sea breeze)")
		dir := fs.String("dir", "", "where to write it (default: themes/<name>)")
		positional, err := parseInterleaved(fs, rest)
		if err != nil {
			return 2
		}
		if len(positional) != 1 {
			_, _ = fmt.Fprintln(stderr, themeUsage)
			return 2
		}
		return themeNew(positional[0], *label, *dir, stdout, stderr)
	case "add":
		name := fs.String("name", "", "the theme's directory under themes/ (default: the repository's name)")
		positional, err := parseInterleaved(fs, rest)
		if err != nil {
			return 2
		}
		if len(positional) != 1 {
			_, _ = fmt.Fprintln(stderr, themeUsage)
			return 2
		}
		source, version := splitVersion(positional[0])
		if _, err := in.Add(ctx, source, version, *name); err != nil {
			return fail(err)
		}
		_, _ = fmt.Fprintln(stdout, "commit themes/ (the theme and themes/themes.lock) and deploy; activate it on the Appearance page")
		return 0
	case "update":
		if err := fs.Parse(rest); err != nil || fs.NArg() != 1 {
			_, _ = fmt.Fprintln(stderr, themeUsage)
			return 2
		}
		name, version := splitVersion(fs.Arg(0))
		if _, err := in.Update(ctx, name, version); err != nil {
			return fail(err)
		}
		return 0
	case "remove":
		if err := fs.Parse(rest); err != nil || fs.NArg() != 1 {
			_, _ = fmt.Fprintln(stderr, themeUsage)
			return 2
		}
		if err := in.Remove(fs.Arg(0)); err != nil {
			return fail(err)
		}
		_, _ = fmt.Fprintln(stdout, "a site with it active shows the engine's templates until another theme is chosen")
		return 0
	case "search":
		catalogURL := fs.String("catalog", os.Getenv("THEMES_CATALOG_URL"), "the catalog's address (an index.json URL or file)")
		words, err := parseInterleaved(fs, rest)
		if err != nil {
			return 2
		}
		if *catalogURL == "" {
			return fail(fmt.Errorf("no catalog: -catalog <url> or THEMES_CATALOG_URL"))
		}
		return themeSearch(ctx, *catalogURL, strings.Join(words, " "), stdout, stderr)
	case "index":
		list := fs.String("list", "themes.txt", "the themes' repositories, one a line")
		out := fs.String("out", "index.json", "where the catalog goes")
		if err := fs.Parse(rest); err != nil || fs.NArg() != 0 {
			_, _ = fmt.Fprintln(stderr, themeUsage)
			return 2
		}
		sources, err := themeinstall.ReadSources(*list)
		if err != nil {
			return fail(err)
		}
		// Building fetches every repository: more time than one install.
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(len(sources)+1)*themeNetworkTimeout)
		defer cancel()
		catalog, failed := in.BuildIndex(ctx, sources, themeinstall.GitHubStats(nil, "", os.Getenv("GITHUB_TOKEN")))
		for _, err := range failed {
			_, _ = fmt.Fprintln(stderr, "theme: left out:", err)
		}
		b, err := json.MarshalIndent(catalog, "", "  ")
		if err != nil {
			return fail(err)
		}
		if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
			return fail(err)
		}
		_, _ = fmt.Fprintf(stdout, "%s: %d theme(s), %d left out\n", *out, len(catalog.Themes), len(failed))
		return 0
	case "check":
		if err := fs.Parse(rest); err != nil || fs.NArg() > 1 {
			_, _ = fmt.Fprintln(stderr, themeUsage)
			return 2
		}
		dir := "."
		if fs.NArg() == 1 {
			dir = fs.Arg(0)
		}
		return themeCheck(ctx, in, dir, stdout, stderr)
	case "verify":
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		problems, err := themeinstall.Verify()
		if err != nil {
			return fail(err)
		}
		for _, p := range problems {
			_, _ = fmt.Fprintf(stderr, "theme %s: %s\n", p.Theme, p.Reason)
		}
		if len(problems) > 0 {
			_, _ = fmt.Fprintln(stderr, "theme: reinstall them (app theme update <name>@<version>) or update to a release")
			return 1
		}
		_, _ = fmt.Fprintln(stdout, "themes: as in themes/themes.lock")
		return 0
	}
	_, _ = fmt.Fprintln(stderr, themeUsage)
	return 2
}

// themeCheck prints what Installer.Check finds in the theme in dir, the
// problems' and warnings' count; 1 when there is a problem.
func themeCheck(ctx context.Context, in themeinstall.Installer, dir string, stdout, stderr io.Writer) int {
	engine, err := i18n.ParseSources(translations.Sources)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "theme:", err)
		return 1
	}
	findings, err := in.Check(ctx, dir, engine)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "theme:", err)
		return 1
	}
	problems := 0
	for _, f := range findings {
		if f.Problem {
			problems++
		}
		_, _ = fmt.Fprintln(stdout, f)
	}
	_, _ = fmt.Fprintf(stdout, "%d problem(s), %d warning(s)\n", problems, len(findings)-problems)
	if problems > 0 {
		return 1
	}
	return 0
}

// parseInterleaved parses args with fs, the flags before or after the
// positional arguments, which it returns.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
	return positional, nil
}

// themeNew writes a new theme's files (scaffold.ThemeFiles) into dir
// (default: themes/<name>), which must not exist or be empty.
func themeNew(name, label, dir string, stdout, stderr io.Writer) int {
	files, err := scaffold.ThemeFiles(scaffold.Theme{Name: name, Label: label, MinVersion: themes.EngineRelease()})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "theme:", err)
		return 2
	}
	if dir == "" {
		root, err := paths.ThemesDir()
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "theme:", err)
			return 1
		}
		dir = filepath.Join(root, name)
	}
	written, err := scaffold.Write(dir, files)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "theme:", err)
		return 1
	}
	for _, f := range written {
		_, _ = fmt.Fprintln(stdout, filepath.Join(dir, filepath.FromSlash(f)))
	}
	_, _ = fmt.Fprintf(stdout, "next: preview it on the Appearance page; replace the engine's templates under templates/; app theme check %s before a release\n", dir)
	return 0
}

// themeList prints every theme: its directory, name, and for an
// installed one its version and source (and, with updates, the newest
// release when it is newer).
func themeList(ctx context.Context, in themeinstall.Installer, updates bool, stdout, stderr io.Writer) int {
	options, err := themes.Available()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "theme:", err)
		return 1
	}
	lock, err := themes.ReadLock()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "theme:", err)
		return 1
	}
	invalid, err := themes.Invalid()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "theme:", err)
		return 1
	}
	for _, th := range invalid {
		_, _ = fmt.Fprintf(stderr, "%-16s not a usable theme: %s\n", th.Dir, th.Err)
	}
	for _, th := range options {
		if th.Dir == "" {
			continue
		}
		entry, installed := lock.Themes[th.Dir]
		if !installed {
			_, _ = fmt.Fprintf(stdout, "%-16s %-20s shipped with the project\n", th.Dir, th.Name)
			continue
		}
		line := fmt.Sprintf("%-16s %-20s %s  %s", th.Dir, th.Name, entry.Version, entry.Source)
		if updates {
			if tags, err := in.Tags(ctx, entry.Source); err != nil {
				line += "  (updates: " + err.Error() + ")"
			} else if newest := newestRelease(tags); newest != "" && semver.Compare(newest, entry.Version) > 0 {
				line += "  → " + newest + " available"
			}
		}
		_, _ = fmt.Fprintln(stdout, line)
	}
	return 0
}

// themeSearch prints the catalog's themes matching words: each one's name,
// release, stars, title and description, and the command installing it
// (or, installed, its version and whether the catalog's is newer).
func themeSearch(ctx context.Context, address, words string, stdout, stderr io.Writer) int {
	catalog, err := themes.FetchCatalog(ctx, nil, address)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "theme:", err)
		return 1
	}
	lock, err := themes.ReadLock()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "theme:", err)
		return 1
	}
	found := 0
	for _, th := range catalog.Themes {
		if !th.Match(words) {
			continue
		}
		found++
		_, _ = fmt.Fprintf(stdout, "%s %s  ★%d  %s\n", th.Name, th.Version, th.Stars, th.Title)
		if th.Description != "" {
			_, _ = fmt.Fprintf(stdout, "    %s\n", th.Description)
		}
		switch entry, installed := lock.Themes[th.Name]; {
		case installed && entry.Source == th.Source && semver.Compare(th.Version, entry.Version) > 0:
			_, _ = fmt.Fprintf(stdout, "    installed %s: app theme update %s\n", entry.Version, th.Name)
		case installed && entry.Source == th.Source:
			_, _ = fmt.Fprintf(stdout, "    installed %s\n", entry.Version)
		case paths.ThemeDir(th.Name) != "":
			_, _ = fmt.Fprintf(stdout, "    themes/%s is another theme's: app theme add %s -name <name>\n", th.Name, th.Source)
		default:
			_, _ = fmt.Fprintf(stdout, "    app theme add %s\n", th.Source)
		}
	}
	if found == 0 {
		_, _ = fmt.Fprintln(stdout, "no themes found")
	}
	return 0
}

// newestRelease - the first tag that isn't a pre-release (Tags lists the
// newest first).
func newestRelease(tags []string) string {
	for _, t := range tags {
		if semver.Prerelease(t) == "" {
			return t
		}
	}
	return ""
}

// splitVersion - "<repository or name>@<version>" into its parts; an
// "@" of an ssh address (git@host:path) isn't a version's.
func splitVersion(arg string) (string, string) {
	i := strings.LastIndex(arg, "@")
	if i <= 0 || strings.ContainsAny(arg[i+1:], "/:") {
		return arg, ""
	}
	return arg[:i], arg[i+1:]
}
