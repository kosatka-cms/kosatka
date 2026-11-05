package notfoundlog

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// pathMax - a longer path isn't logged (a scanner's, most likely).
const pathMax = 1024

// Recorder logs the paths answered 404 (modulekit.OnNotFound) while the
// module is on, skipping the ignore patterns, and forgets the old ones.
type Recorder struct {
	repo      *Repo
	redirects modulekit.RedirectService
	logger    *slog.Logger
	now       func() time.Time

	mu       sync.Mutex
	settings *Settings
	ignore   []*regexp.Regexp
}

// NewRecorder creates a Recorder.
func NewRecorder(repo *Repo, redirects modulekit.RedirectService, logger *slog.Logger) *Recorder {
	return &Recorder{repo: repo, redirects: redirects, logger: logger, now: time.Now}
}

// Settings returns the module's settings (kept in memory once read).
func (r *Recorder) Settings(ctx context.Context) Settings {
	r.mu.Lock()
	cached := r.settings
	r.mu.Unlock()
	if cached != nil {
		return *cached
	}
	st, err := r.repo.Settings(ctx)
	if err != nil {
		modulekit.LogFallback(r.logger, err, "404 log settings (the defaults used)")
		st = defaultSettings()
	}
	r.remember(st)
	return st
}

// SaveSettings saves the settings.
func (r *Recorder) SaveSettings(ctx context.Context, st Settings) error {
	if err := r.repo.SaveSettings(ctx, st); err != nil {
		return err
	}
	r.remember(st)
	return nil
}

func (r *Recorder) remember(st Settings) {
	ignore := make([]*regexp.Regexp, 0, len(st.Ignore))
	for _, p := range st.Ignore {
		ignore = append(ignore, globRegexp(p))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings, r.ignore = &st, ignore
}

// globRegexp - a pattern of paths (* - any characters, the rest as is,
// letter case aside) as a regexp of the whole path.
func globRegexp(pattern string) *regexp.Regexp {
	parts := strings.Split(pattern, "*")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile("(?i)^" + strings.Join(parts, ".*") + "$")
}

// Ignored - whether path matches an ignore pattern.
func (r *Recorder) Ignored(ctx context.Context, path string) bool {
	r.Settings(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, re := range r.ignore {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

// Record logs a request answered 404 (the listener of
// modulekit.OnNotFound). Its failure is logged: the answer is given.
func (r *Recorder) Record(ctx context.Context, path, referrer string) {
	if path == "" || len(path) > pathMax || r.Ignored(ctx, path) {
		return
	}
	if len(referrer) > pathMax {
		referrer = referrer[:pathMax]
	}
	if err := r.repo.Hit(ctx, path, referrer, r.now()); err != nil && r.logger != nil {
		r.logger.Error("logging a 404", "error", err, "path", path)
	}
}

// Redirected - whether a redirect answers path now.
func (r *Recorder) Redirected(path string) bool {
	if r.redirects == nil {
		return false
	}
	_, ok := r.redirects.GetRedirect(strings.TrimPrefix(path, "/"))
	return ok
}

// Purge forgets the paths not asked for longer than the settings keep
// them, and those a redirect answers now - the module's background job.
func (r *Recorder) Purge(ctx context.Context) error {
	var before time.Time
	if days := r.Settings(ctx).KeepDays; days > 0 {
		before = r.now().AddDate(0, 0, -days)
	}
	_, err := r.repo.Purge(ctx, before, r.Redirected)
	return err
}

// CountNew - the number by the Redirects menu item (the core's, shown
// with the module off too): the new paths, while the module is on and the
// settings show it.
func (r *Recorder) CountNew(ctx context.Context) (int, error) {
	if !r.Settings(ctx).ShowCount {
		return 0, nil
	}
	return r.repo.CountNew(ctx)
}
