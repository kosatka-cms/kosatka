package notfoundlog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/not_found_log/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/not_found_log/ent/generated/notfoundpath"
	"github.com/kosatka-cms/kosatka/internal/modules/not_found_log/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Path - a path the site answered 404 to: how many times, when first and
// last, where the last visitor came from; Ignored - not among the new.
type Path struct {
	ID        int
	Path      string
	Hits      int
	FirstSeen time.Time
	LastSeen  time.Time
	Referrer  string
	Ignored   bool
}

// The paths a list shows (Filter.Status).
const (
	StatusNew     = "new"
	StatusIgnored = "ignored"
	StatusAll     = "all"
)

// Filter - what the list shows: Q - a part of the path; Status - the new
// ones (the default), the ignored ones, or all.
type Filter struct {
	Q      string
	Status string
}

// Repo - the module's tables.
type Repo struct {
	db *sql.DB
}

// NewRepo creates a Repo on db, in the repositories' dialect.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// client - the module's ent client: the context's transaction (InTx - a
// hook running in the core's change, say), else the database.
func (r *Repo) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// pathHash - a path's key.
func pathHash(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:])
}

// maxPaths - the paths the log keeps at most: anyone can ask for made-up
// addresses, and a full log adds no new one (a known one is still
// counted) until old ones are purged or the log is cleared. A variable
// for the tests.
var maxPaths = 10_000

// Hit counts a request answered 404 to path: a new path is added (while
// the log has fewer than maxPaths), a known one gets one more hit, now as
// its last, referrer as its last source.
func (r *Repo) Hit(ctx context.Context, path, referrer string, now time.Time) error {
	hash := pathHash(path)
	update := func() (int, error) {
		return r.client(ctx).NotFoundPath.Update().Where(notfoundpath.PathHash(hash)).
			AddHits(1).SetLastSeen(now).SetReferrer(referrer).Save(ctx)
	}
	n, err := update()
	if err == nil && n == 0 {
		err = r.add(ctx, hash, path, referrer, now)
		if generated.IsConstraintError(err) {
			// Another request added it meanwhile: count this one too.
			_, err = update()
		}
	}
	if err != nil {
		return fmt.Errorf("logging a 404 (path %v): %w", path, err)
	}
	return nil
}

// add - a new path's row, unless the log holds maxPaths already.
func (r *Repo) add(ctx context.Context, hash, path, referrer string, now time.Time) error {
	total, err := r.client(ctx).NotFoundPath.Query().Count(ctx)
	if err != nil || total >= maxPaths {
		return err
	}
	return r.client(ctx).NotFoundPath.Create().SetPathHash(hash).SetPath(path).SetHits(1).
		SetFirstSeen(now).SetLastSeen(now).SetReferrer(referrer).Exec(ctx)
}

func (f Filter) where() []predicate.NotFoundPath {
	var w []predicate.NotFoundPath
	switch f.Status {
	case StatusIgnored:
		w = append(w, notfoundpath.Ignored(true))
	case StatusAll:
	default:
		w = append(w, notfoundpath.Ignored(false))
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		w = append(w, notfoundpath.PathContainsFold(q))
	}
	return w
}

// List returns the paths the filter shows, the most asked for first, a
// page at a time, with how many there are.
func (r *Repo) List(ctx context.Context, f Filter, limit, offset int) ([]Path, int, error) {
	q := r.client(ctx).NotFoundPath.Query().Where(f.where()...)
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting 404 paths: %w", err)
	}
	rows, err := q.Order(generated.Desc(notfoundpath.FieldHits), generated.Desc(notfoundpath.FieldLastSeen)).
		Limit(limit).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("listing 404 paths: %w", err)
	}
	out := make([]Path, len(rows))
	for i, row := range rows {
		out[i] = toPath(row)
	}
	return out, total, nil
}

func toPath(row *generated.NotFoundPath) Path {
	return Path{ID: row.ID, Path: row.Path, Hits: row.Hits, FirstSeen: row.FirstSeen, LastSeen: row.LastSeen, Referrer: row.Referrer, Ignored: row.Ignored}
}

// ByID returns a path.
func (r *Repo) ByID(ctx context.Context, id int) (Path, error) {
	row, err := r.client(ctx).NotFoundPath.Get(ctx, id)
	if err != nil {
		return Path{}, fmt.Errorf("loading a 404 path (id %v): %w", id, err)
	}
	return toPath(row), nil
}

// CountNew - how many paths are new (not ignored).
func (r *Repo) CountNew(ctx context.Context) (int, error) {
	n, err := r.client(ctx).NotFoundPath.Query().Where(notfoundpath.Ignored(false)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting new 404 paths: %w", err)
	}
	return n, nil
}

// SetIgnored marks a path ignored or new again.
func (r *Repo) SetIgnored(ctx context.Context, id int, ignored bool) error {
	if err := r.client(ctx).NotFoundPath.UpdateOneID(id).SetIgnored(ignored).Exec(ctx); err != nil {
		return fmt.Errorf("ignoring a 404 path (id %v): %w", id, err)
	}
	return nil
}

// Delete forgets a path (a redirect now answers it).
func (r *Repo) Delete(ctx context.Context, id int) error {
	if err := r.client(ctx).NotFoundPath.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("deleting a 404 path (id %v): %w", id, err)
	}
	return nil
}

// Clear forgets every path; returns how many.
func (r *Repo) Clear(ctx context.Context) (int, error) {
	n, err := r.client(ctx).NotFoundPath.Delete().Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("clearing the 404 log: %w", err)
	}
	return n, nil
}

// Purge forgets the paths not asked for since before, and the ones
// answered by a redirect now (redirected); returns how many.
func (r *Repo) Purge(ctx context.Context, before time.Time, redirected func(path string) bool) (int, error) {
	n := 0
	if !before.IsZero() {
		m, err := r.client(ctx).NotFoundPath.Delete().Where(notfoundpath.LastSeenLT(before)).Exec(ctx)
		if err != nil {
			return 0, fmt.Errorf("purging the 404 log: %w", err)
		}
		n += m
	}
	if redirected == nil {
		return n, nil
	}
	rows, err := r.client(ctx).NotFoundPath.Query().Select(notfoundpath.FieldID, notfoundpath.FieldPath).All(ctx)
	if err != nil {
		return n, fmt.Errorf("listing 404 paths: %w", err)
	}
	var gone []int
	for _, row := range rows {
		if redirected(row.Path) {
			gone = append(gone, row.ID)
		}
	}
	if len(gone) > 0 {
		m, err := r.client(ctx).NotFoundPath.Delete().Where(notfoundpath.IDIn(gone...)).Exec(ctx)
		if err != nil {
			return n, fmt.Errorf("forgetting redirected 404 paths: %w", err)
		}
		n += m
	}
	return n, nil
}

// Settings - the module's settings: KeepDays - a path not asked for
// longer is forgotten; ShowCount - the count by the Redirects menu item;
// Ignore - the patterns of paths never logged.
type Settings struct {
	KeepDays  int
	ShowCount bool
	Ignore    []string
}

// defaultIgnore - what scanners keep asking for on any site.
var defaultIgnore = []string{"*.php", "/wp-*", "/wordpress*", "/.env*", "/.git*", "/.aws*", "/phpmyadmin*", "/cgi-bin/*", "*.asp", "*.aspx", "*.jsp", "/xmlrpc*", "/vendor/*", "/.well-known/*"}

// defaultSettings - the settings until the page is saved.
func defaultSettings() Settings {
	return Settings{KeepDays: 90, ShowCount: true, Ignore: defaultIgnore}
}

// Settings returns the module's settings (the defaults until saved).
func (r *Repo) Settings(ctx context.Context) (Settings, error) {
	row, err := r.client(ctx).NotFoundSettings.Get(ctx, 1)
	if generated.IsNotFound(err) {
		return defaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("loading the 404 log settings: %w", err)
	}
	return Settings{KeepDays: row.KeepDays, ShowCount: row.ShowCount, Ignore: patternLines(row.IgnorePatterns)}, nil
}

// SaveSettings saves the module's settings.
func (r *Repo) SaveSettings(ctx context.Context, s Settings) error {
	patterns := strings.Join(s.Ignore, "\n")
	err := r.client(ctx).NotFoundSettings.UpdateOneID(1).SetKeepDays(s.KeepDays).SetShowCount(s.ShowCount).SetIgnorePatterns(patterns).Exec(ctx)
	if generated.IsNotFound(err) {
		err = r.client(ctx).NotFoundSettings.Create().SetID(1).SetKeepDays(s.KeepDays).SetShowCount(s.ShowCount).SetIgnorePatterns(patterns).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the 404 log settings: %w", err)
	}
	return nil
}

// patternLines - the non-empty lines of text, trimmed.
func patternLines(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
