package app

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
)

// Several sites on one server share one engine (docs/guides/multisite.md):
// an engine directory holds the binary and its web/ (templates, static
// files) and themes/; each site is a directory of the sites root holding
// its config (site.yaml), its secrets (site.env, environment variables as
// a systemd EnvironmentFile has them), its themes and its files, and runs
// as its own service with its own database. With engine releases side by
// side (<home>/releases/<name>/, sites_releases.go) each site links the
// one it runs (its engine link): app sites update moves the sites to the
// release it runs from one by one, app sites rollback back. Every step
// runs the engine binary for the site, as its service runs it - the
// site's directory as the working directory, its env file, config and
// engine directory - so a site is read exactly as it runs.

const (
	// siteConfigFile, siteEnvFile - a site's config and secrets in its
	// directory.
	siteConfigFile = "site.yaml"
	siteEnvFile    = "site.env"
	// defaultRestart - how update restarts a site ({site} - its name):
	// the deploy/systemd/kosatka-site@.service instance.
	defaultRestart = "systemctl restart kosatka-site@{site}"
)

// hostSite - a site of the sites root: its name (its directory's) and
// directory.
type hostSite struct {
	Name, Dir string
}

// sitesHost - the engine and the sites on this server, and how a step
// reaches them (replaced in the tests).
type sitesHost struct {
	// engine, binary - this binary's engine directory and the binary;
	// release - its directory when it is a release (<home>/releases/<name>),
	// "" - a single engine directory.
	engine, binary, release, root string
	out                           io.Writer
	// run runs the binary of engine for site with args, its output; engine
	// "" - the site's own (its engine link, else this binary's).
	run func(ctx context.Context, site hostSite, engine string, args ...string) (string, error)
	// command runs a command (the restart), its output.
	command func(ctx context.Context, name string, args ...string) (string, error)
	// status - the HTTP status of a GET of url.
	status func(ctx context.Context, url string) (int, error)
	// pause waits between health checks.
	pause func(time.Duration)
}

// runSites is "app sites list|update …".
func runSites(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, sitesUsage)
		return 2
	}
	h, err := newSitesHost(os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sites:", err)
		return 1
	}
	if args[0] == "add" {
		return h.runSitesAdd(args[1:])
	}
	fs := flag.NewFlagSet("sites "+args[0], flag.ContinueOnError)
	fs.StringVar(&h.root, "root", h.root, "the sites root: a directory per site")
	switch args[0] {
	case "list":
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		return h.list(context.Background())
	case "update", "rollback":
		only := fs.String("only", "", "only these sites (comma-separated), in this order")
		restart := fs.String("restart", defaultRestart, `how to restart a site, {site} - its name ("" - don't)`)
		wait := fs.Duration("wait", time.Minute, "how long a restarted site may take to answer /healthz")
		backup := fs.Bool("backup", true, "dump each site's database before its migrations (update)")
		keep := fs.Int("keep", 5, "how many dumps a site keeps (update)")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if args[0] == "rollback" {
			return h.rollback(context.Background(), splitList(*only), *restart, *wait)
		}
		return h.update(context.Background(), splitList(*only), updateOptions{restart: *restart, wait: *wait, backup: *backup, keep: *keep})
	default:
		fmt.Fprintf(os.Stderr, "sites: unknown command %q\n%s\n", args[0], sitesUsage)
		return 2
	}
}

const sitesUsage = `usage:
  app sites list [-root <dir>]            the sites of this engine: URL, pending migrations, health
  app sites add <name> -port <n> -url <address> -db-name <name> -site-name … -admin-name … -admin-email …
                                          a new site: its directory, config, install, service (DB_PASSWORD in the environment)
  app sites update [-root <dir>] [-only a,b] [-restart '<command with {site}>'] [-wait 1m] [-backup=false] [-keep 5]
                                          move each site to this engine in turn: dump, migrate, restart, check;
                                          a failed site goes back to its release, and the update stops
  app sites rollback [-root <dir>] [-only a,b] [-restart …] [-wait 1m]
                                          each site back to the release it ran before`

// newSitesHost - the sites host of the running binary: the engine
// directory is ENGINE_DIR, else the binary's; a release
// (<home>/releases/<name>) has the sites root <home>/sites, a single
// engine directory its sites/.
func newSitesHost(out io.Writer) (*sitesHost, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("finding the engine binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(binary); err == nil {
		binary = resolved
	}
	engine := os.Getenv("ENGINE_DIR")
	if engine == "" {
		engine = filepath.Dir(binary)
	}
	h := &sitesHost{engine: engine, binary: binary, root: filepath.Join(engine, "sites"), out: out, pause: time.Sleep}
	if home, ok := releaseHome(engine); ok {
		h.release, h.root = engine, filepath.Join(home, "sites")
	}
	h.run = h.runEngine
	h.command = func(ctx context.Context, name string, args ...string) (string, error) {
		b, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
		return string(b), err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	h.status = func(ctx context.Context, url string) (int, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return 0, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		_ = resp.Body.Close()
		return resp.StatusCode, nil
	}
	return h, nil
}

// runEngine runs the binary of engine ("" - the site's own, siteEngine)
// for site as its service would: in its directory, with its config, its
// env file and the engine directory, and nothing else of this process's
// environment but the basics (one site's settings never leak into
// another's).
func (h *sitesHost) runEngine(ctx context.Context, site hostSite, engine string, args ...string) (string, error) {
	if engine == "" {
		engine = h.siteEngine(site)
	}
	binary := h.binary
	if engine != h.engine {
		binary = filepath.Join(engine, filepath.Base(h.binary))
	}
	env, err := siteEnviron(engine, site)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir, cmd.Env = site.Dir, env
	b, err := cmd.CombinedOutput()
	return string(b), err
}

// siteEnviron - the environment site runs with: PATH and the like from
// this process, ENGINE_DIR and CONFIG_PATH, then its env file's variables
// (they may set those two too).
func siteEnviron(engine string, site hostSite) ([]string, error) {
	var env []string
	for _, k := range []string{"PATH", "HOME", "LANG", "LC_ALL", "TZ", "TMPDIR"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	env = append(env, "ENGINE_DIR="+engine, "CONFIG_PATH="+filepath.Join(site.Dir, siteConfigFile))
	f, err := os.Open(filepath.Join(site.Dir, siteEnvFile))
	if errors.Is(err, os.ErrNotExist) {
		return env, nil
	}
	if err != nil {
		return nil, fmt.Errorf("site %s: %w", site.Name, err)
	}
	defer func() { _ = f.Close() }()
	vars, err := parseEnvFile(f)
	if err != nil {
		return nil, fmt.Errorf("site %s: %s: %w", site.Name, siteEnvFile, err)
	}
	return append(env, vars...), nil
}

// parseEnvFile - the KEY=VALUE lines of an env file as systemd's
// EnvironmentFile reads them: blank lines and # comments skipped, an
// optional "export ", a value in single or double quotes unquoted.
func parseEnvFile(r io.Reader) ([]string, error) {
	var out []string
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.ContainsAny(key, " \t\"'") {
			return nil, fmt.Errorf("line %d: not KEY=VALUE", n)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			if value[0] == '"' {
				value = strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(value[1 : len(value)-1])
			} else {
				value = value[1 : len(value)-1]
			}
		}
		out = append(out, key+"="+value)
	}
	return out, sc.Err()
}

// sites - the sites of the root (a directory with a site.yaml), by name;
// only - just those, in that order (an unknown name is an error).
func (h *sitesHost) sites(only []string) ([]hostSite, error) {
	entries, err := os.ReadDir(h.root)
	if err != nil {
		return nil, fmt.Errorf("reading the sites root: %w", err)
	}
	var all []hostSite
	for _, e := range entries {
		dir := filepath.Join(h.root, e.Name())
		if e.IsDir() && fileExists(filepath.Join(dir, siteConfigFile)) {
			all = append(all, hostSite{Name: e.Name(), Dir: dir})
		}
	}
	if len(only) == 0 {
		return all, nil
	}
	var out []hostSite
	for _, name := range only {
		i := slices.IndexFunc(all, func(s hostSite) bool { return s.Name == name })
		if i < 0 {
			return nil, fmt.Errorf("no site %q in %s", name, h.root)
		}
		out = append(out, all[i])
	}
	return out, nil
}

// siteInfo - site's settings as it runs (app config -json, by this
// binary: the site's own release may predate the command).
func (h *sitesHost) siteInfo(ctx context.Context, site hostSite) (siteConfigInfo, error) {
	var info siteConfigInfo
	out, err := h.run(ctx, site, h.engine, "config", "-json")
	if err != nil {
		return info, fmt.Errorf("reading its config: %w: %s", err, strings.TrimSpace(out))
	}
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return info, fmt.Errorf("reading its config: %w", err)
	}
	return info, nil
}

// pending - how many migrations site has to run (app migrate's plan).
func (h *sitesHost) pending(ctx context.Context, site hostSite) (int, error) {
	out, err := h.run(ctx, site, "", "migrate")
	if err != nil {
		return 0, fmt.Errorf("%w: %s", err, lastLine(out))
	}
	return strings.Count(out, ": pending "), nil
}

// healthURL - the /healthz address of a site listening on port.
func healthURL(port int) string {
	return "http://127.0.0.1:" + strconv.Itoa(port) + middleware.HealthPath
}

// healthy - whether the site on port answers /healthz with 200; an error
// says why not.
func (h *sitesHost) healthy(ctx context.Context, port int) error {
	if port <= 0 {
		return errors.New("no port in its config")
	}
	code, err := h.status(ctx, healthURL(port))
	if uerr := (*url.Error)(nil); errors.As(err, &uerr) {
		return uerr.Err
	}
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("%s answers %d", middleware.HealthPath, code)
	}
	return nil
}

// list prints the sites: URL, the release it runs, pending migrations,
// health.
func (h *sitesHost) list(ctx context.Context) int {
	sites, err := h.sites(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sites:", err)
		return 1
	}
	if len(sites) == 0 {
		_, _ = fmt.Fprintf(h.out, "no sites in %s\n", h.root)
		return 0
	}
	tw := tabwriter.NewWriter(h.out, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(tw, "SITE\tURL\tRELEASE\tMIGRATIONS\tHEALTH")
	for _, s := range sites {
		url := "?"
		var migrations, health string
		if info, err := h.siteInfo(ctx, s); err != nil {
			health = "config error: " + firstLine(err.Error())
		} else {
			url = cmp.Or(info.PublicBaseURL, "port "+strconv.Itoa(info.Port))
			health = "ok"
			if err := h.healthy(ctx, info.Port); err != nil {
				health = "down: " + firstLine(err.Error())
			}
		}
		switch n, err := h.pending(ctx, s); {
		case err != nil:
			migrations = "error: " + firstLine(err.Error())
		case n == 0:
			migrations = "up to date"
		default:
			migrations = strconv.Itoa(n) + " pending"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.Name, url, releaseName(s.linked(siteEngineLink)), migrations, health)
	}
	_ = tw.Flush()
	return 0
}

// updateOptions - how update goes: restart ({site} - its name; "" - no
// restart), wait for /healthz, backup - a dump before the migrations,
// keep - the dumps a site keeps.
type updateOptions struct {
	restart string
	wait    time.Duration
	backup  bool
	keep    int
}

// update brings each site to this engine in turn (updateSite). The first
// failure stops it: that site goes back to the release it ran, the sites
// after it stay as they were, and the summary names them.
func (h *sitesHost) update(ctx context.Context, only []string, opts updateOptions) int {
	sites, err := h.sites(only)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sites:", err)
		return 1
	}
	for i, s := range sites {
		_, _ = fmt.Fprintf(h.out, "==> %s\n", s.Name)
		if err := h.updateSite(ctx, s, opts); err != nil {
			_, _ = fmt.Fprintf(h.out, "\nsites: stopped at %s: %v\n", s.Name, err)
			if rest := sites[i+1:]; len(rest) > 0 {
				names := make([]string, len(rest))
				for j, r := range rest {
					names[j] = r.Name
				}
				_, _ = fmt.Fprintf(h.out, "not updated: %s\n", strings.Join(names, ", "))
			}
			return 1
		}
	}
	_, _ = fmt.Fprintf(h.out, "\nsites: %d updated\n", len(sites))
	return 0
}

// updateSite brings site s to this engine: a dump of its database (not
// when it runs this release already), its engine link to this release (a release; the release it ran is kept as
// its previous one), its migrations, its restart and /healthz. A failure
// after the link moved moves it back (and restarts the site on it again
// when it was restarted); the dump is named for a database the migrations
// changed.
func (h *sitesHost) updateSite(ctx context.Context, s hostSite, opts updateOptions) error {
	info, err := h.siteInfo(ctx, s)
	if err != nil {
		return err
	}
	prev := s.linked(siteEngineLink)
	switched := h.release != "" && !samePath(prev, h.release)
	// A dump, but of a site on this release already (tried on it first):
	// nothing new to migrate.
	dump := ""
	if opts.backup && (h.release == "" || switched) {
		if dump, err = h.backupSite(ctx, s, opts.keep); err != nil {
			return err
		}
	}
	if switched {
		if err := setLink(s.Dir, siteEngineLink, h.release); err != nil {
			return fmt.Errorf("moving it to %s: %w", releaseName(h.release), err)
		}
		_, _ = fmt.Fprintf(h.out, "  release: %s -> %s\n", releaseName(prev), releaseName(h.release))
	}
	restarted, err := h.migrateRestart(ctx, s, info.Port, opts, true)
	if err == nil {
		if switched && prev != "" {
			err = setLink(s.Dir, sitePreviousLink, prev)
		}
		return err
	}
	if switched {
		if prev == "" {
			_ = os.Remove(filepath.Join(s.Dir, siteEngineLink))
		} else if lerr := setLink(s.Dir, siteEngineLink, prev); lerr != nil {
			err = errors.Join(err, fmt.Errorf("moving it back to %s: %w", releaseName(prev), lerr))
		}
		_, _ = fmt.Fprintf(h.out, "  back to release %s\n", releaseName(prev))
		if restarted {
			if _, rerr := h.migrateRestart(ctx, s, info.Port, opts, false); rerr != nil {
				err = errors.Join(err, fmt.Errorf("restarting it on %s: %w", releaseName(prev), rerr))
			}
		}
	}
	if dump != "" {
		err = fmt.Errorf("%w (its database before the update: %s)", err, dump)
	}
	return err
}

// migrateRestart runs site s's migrations (migrate; false - just the
// restart, back on its previous release), restarts it (opts.restart) and
// waits for /healthz: whether it was restarted, the error.
func (h *sitesHost) migrateRestart(ctx context.Context, s hostSite, port int, opts updateOptions, migrate bool) (bool, error) {
	if migrate {
		out, err := h.run(ctx, s, "", "migrate", "-apply")
		h.indent(out)
		if err != nil {
			return false, fmt.Errorf("migrations: %w", err)
		}
	}
	cmd := siteCommand(opts.restart, s.Name)
	if len(cmd) > 0 {
		_, _ = fmt.Fprintf(h.out, "  restart: %s\n", strings.Join(cmd, " "))
		out, err := h.command(ctx, cmd[0], cmd[1:]...)
		h.indent(out)
		if err != nil {
			return true, fmt.Errorf("restart: %w", err)
		}
	}
	return len(cmd) > 0, h.waitHealthy(ctx, port, opts.wait)
}

// siteCommand - a command template ({site} - the site's name) as its
// words; none for a blank one (no such step).
func siteCommand(template, site string) []string {
	return strings.Fields(strings.ReplaceAll(template, "{site}", site))
}

// indent writes a command's output indented under its site, but the
// modules migrate skips (switched off).
func (h *sitesHost) indent(out string) {
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line != "" && !strings.HasSuffix(line, "switched off - skipped") {
			_, _ = fmt.Fprintf(h.out, "  %s\n", line)
		}
	}
}

// splitList - a comma-separated list's items, trimmed, empty ones left out.
func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// firstLine, lastLine - a text's first and last non-empty lines.
func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

func lastLine(s string) string {
	lines := bytes.Split(bytes.TrimSpace([]byte(s)), []byte("\n"))
	return string(lines[len(lines)-1])
}

// fileExists - whether p is a file.
func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
