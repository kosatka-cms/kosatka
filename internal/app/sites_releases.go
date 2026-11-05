package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Engine releases side by side: <home>/releases/<name>/ - a whole engine
// (its binary, web/, themes/) - and <home>/sites/<site>/engine, a link to
// the release the site runs (its service runs the binary through it).
// Moving the sites one by one keeps each on one release - binary and
// templates together - whatever the others run; engine.previous is the
// release a site ran before, for app sites rollback; backups/ holds its
// dumps from before its updates.
const (
	siteEngineLink   = "engine"
	sitePreviousLink = "engine.previous"
	siteBackupsDir   = "backups"
)

// releaseHome - engine's installation directory when engine is a release
// (<home>/releases/<name>).
func releaseHome(engine string) (string, bool) {
	parent := filepath.Dir(filepath.Clean(engine))
	if filepath.Base(parent) != "releases" {
		return "", false
	}
	return filepath.Dir(parent), true
}

// samePath reports whether a and b are the same directory (links
// resolved).
func samePath(a, b string) bool {
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// releaseName - a release directory's name, "-" for none.
func releaseName(dir string) string {
	if dir == "" {
		return "-"
	}
	return filepath.Base(dir)
}

// linked - the directory site's link name points to, "" for none.
func (s hostSite) linked(name string) string {
	target, err := filepath.EvalSymlinks(filepath.Join(s.Dir, name))
	if err != nil {
		return ""
	}
	return target
}

// siteEngine - the engine site runs: its engine link, else this binary's.
func (h *sitesHost) siteEngine(s hostSite) string {
	if _, err := os.Lstat(filepath.Join(s.Dir, siteEngineLink)); err == nil {
		return filepath.Join(s.Dir, siteEngineLink)
	}
	return h.engine
}

// setLink points site directory dir's link name at target (relative to
// dir, both resolved: a link above them would make a relative path miss),
// replacing it at once: a new link renamed over the old one.
func setLink(dir, name, target string) error {
	from, to := dir, target
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		from = resolved
	}
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		to = resolved
	}
	rel, err := filepath.Rel(from, to)
	if err != nil {
		rel = to
	}
	tmp := filepath.Join(dir, "."+name+".new")
	_ = os.Remove(tmp)
	if err := os.Symlink(rel, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}

// backupSite dumps site s's database into its backups/ (app backup, by
// this binary: an older release may not have the command), keeping its
// keep newest dumps: the dump's path.
func (h *sitesHost) backupSite(ctx context.Context, s hostSite, keep int) (string, error) {
	dir := filepath.Join(s.Dir, siteBackupsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("its backups directory: %w", err)
	}
	file := filepath.Join(dir, time.Now().UTC().Format("20060102-150405")+"-"+releaseName(s.linked(siteEngineLink))+".sql.gz")
	out, err := h.run(ctx, s, h.engine, "backup", file)
	h.indent(out)
	if err != nil {
		return "", fmt.Errorf("its backup: %w (-backup=false updates without one)", err)
	}
	return file, pruneBackups(dir, keep)
}

// pruneBackups deletes all but the keep newest dumps of dir (by name: they
// start with their time).
func pruneBackups(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var dumps []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql.gz") {
			dumps = append(dumps, e.Name())
		}
	}
	slices.Sort(dumps)
	for len(dumps) > max(keep, 1) {
		if err := os.Remove(filepath.Join(dir, dumps[0])); err != nil {
			return err
		}
		dumps = dumps[1:]
	}
	return nil
}

// newestBackup - site directory dir's newest dump, "" for none.
func newestBackup(dir string) string {
	entries, err := os.ReadDir(filepath.Join(dir, siteBackupsDir))
	if err != nil {
		return ""
	}
	newest := ""
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql.gz") && e.Name() > newest {
			newest = e.Name()
		}
	}
	if newest == "" {
		return ""
	}
	return filepath.Join(dir, siteBackupsDir, newest)
}

// rollback moves each site back to the release it ran before (its
// engine.previous), restarts it and waits for /healthz; the release it
// leaves becomes its previous one. Its database stays as it is: the
// migrations of the newer release aren't undone - the summary names its
// dump from before the update for that. The first failure stops it.
func (h *sitesHost) rollback(ctx context.Context, only []string, restart string, wait time.Duration) int {
	sites, err := h.sites(only)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sites:", err)
		return 1
	}
	done := 0
	for _, s := range sites {
		prev, cur := s.linked(sitePreviousLink), s.linked(siteEngineLink)
		if prev == "" || prev == cur {
			_, _ = fmt.Fprintf(h.out, "==> %s: no previous release - left as it is\n", s.Name)
			continue
		}
		_, _ = fmt.Fprintf(h.out, "==> %s\n  release: %s -> %s\n", s.Name, releaseName(cur), releaseName(prev))
		err := errors.Join(setLink(s.Dir, siteEngineLink, prev), setLink(s.Dir, sitePreviousLink, cur))
		var info siteConfigInfo
		if err == nil {
			info, err = h.siteInfo(ctx, s)
		}
		if err == nil {
			_, err = h.migrateRestart(ctx, s, info.Port, updateOptions{restart: restart, wait: wait}, false)
		}
		if err != nil {
			_, _ = fmt.Fprintf(h.out, "\nsites: rollback stopped at %s: %v\n", s.Name, err)
			return 1
		}
		if dump := newestBackup(s.Dir); dump != "" {
			_, _ = fmt.Fprintf(h.out, "  its database is as the update left it; before the update: %s\n", dump)
		}
		done++
	}
	_, _ = fmt.Fprintf(h.out, "\nsites: %d rolled back\n", done)
	return 0
}
