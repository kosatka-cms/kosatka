package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// StatusCheck - a check of the status report: its lines (translated in
// ctx's language).
type StatusCheck func(ctx context.Context) []entity.Requirement

// StatusReport - the site's status checks (the core's and the enabled
// modules'), for the status report page and the admin's warning bar.
type StatusReport struct {
	mu       sync.Mutex
	checks   []statusCheck
	ttl      time.Duration
	counted  time.Time
	warnings int
	errors   int
}

type statusCheck struct {
	module string
	run    StatusCheck
}

// NewStatusReport - a report whose counts for the warning bar are worked
// out at most once per ttl.
func NewStatusReport(ttl time.Duration) *StatusReport {
	return &StatusReport{ttl: ttl}
}

// Add - a core check.
func (s *StatusReport) Add(c StatusCheck) { s.AddModule("", c) }

// AddModule - a module's check: run only while the module is enabled.
func (s *StatusReport) AddModule(module string, c StatusCheck) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks = append(s.checks, statusCheck{module: module, run: c})
}

// Run - every check's lines, errors first, then warnings, then the rest
// (each in check order); the warning bar's counts are refreshed.
func (s *StatusReport) Run(ctx context.Context) []entity.Requirement {
	s.mu.Lock()
	checks := append([]statusCheck(nil), s.checks...)
	s.mu.Unlock()
	var out []entity.Requirement
	for _, c := range checks {
		if c.module != "" && !ModuleEnabled(c.module) {
			continue
		}
		for _, r := range c.run(ctx) {
			r.Module = c.module
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Severity > out[j].Severity })
	warnings, errors := 0, 0
	for _, r := range out {
		switch r.Severity {
		case entity.RequirementWarning:
			warnings++
		case entity.RequirementError:
			errors++
		}
	}
	s.mu.Lock()
	s.counted, s.warnings, s.errors = time.Now(), warnings, errors
	s.mu.Unlock()
	return out
}

// Counts - how many warnings and errors the checks found, for the warning
// bar on every admin page: run again only when older than the ttl.
func (s *StatusReport) Counts(ctx context.Context) (warnings, errors int) {
	s.mu.Lock()
	fresh := !s.counted.IsZero() && time.Since(s.counted) < s.ttl
	warnings, errors = s.warnings, s.errors
	s.mu.Unlock()
	if fresh {
		return warnings, errors
	}
	s.Run(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.warnings, s.errors
}

// The core's checks.

// CheckPendingMigrations - schema changes and data steps not applied yet,
// by set (the core, each installed module): an error until
// app migrate -apply runs them.
func CheckPendingMigrations(pending func(ctx context.Context) (map[string]int, error)) StatusCheck {
	return func(ctx context.Context) []entity.Requirement {
		title := i18n.T(ctx, "Database updates")
		bySet, err := pending(ctx)
		if err != nil {
			return []entity.Requirement{{Title: title, Value: i18n.T(ctx, "Could not be checked"), Description: html.EscapeString(err.Error()), Severity: entity.RequirementWarning}}
		}
		total := 0
		var sets []string
		for set, n := range bySet {
			if n > 0 {
				total += n
				sets = append(sets, set+": "+strconv.Itoa(n))
			}
		}
		if total == 0 {
			return []entity.Requirement{{Title: title, Value: i18n.T(ctx, "Up to date")}}
		}
		sort.Strings(sets)
		return []entity.Requirement{{
			Title:       title,
			Value:       i18n.Plural(ctx, total, "@count pending", "@count pending"),
			Description: i18n.T(ctx, "The code expects changes the database doesn't have yet (@sets). Apply them: app migrate -apply.", "@sets", strings.Join(sets, ", ")),
			Severity:    entity.RequirementError,
		}}
	}
}

// CheckMissingModules - modules installed in the database that the
// binary doesn't have (built without them): a warning naming them; no
// line when there are none.
func CheckMissingModules(missing func(ctx context.Context) ([]string, error)) StatusCheck {
	return func(ctx context.Context) []entity.Requirement {
		title := i18n.T(ctx, "Modules missing from the binary")
		names, err := missing(ctx)
		if err != nil {
			return []entity.Requirement{{Title: title, Value: i18n.T(ctx, "Could not be checked"), Description: html.EscapeString(err.Error()), Severity: entity.RequirementWarning}}
		}
		if len(names) == 0 {
			return nil
		}
		return []entity.Requirement{{
			Title:       title,
			Value:       strings.Join(names, ", "),
			Description: i18n.T(ctx, "They are installed in the database, but this binary doesn't have them: their content and blocks are unavailable, their data stays. To uninstall one, deploy a binary that has it (the same version) and uninstall it on the Modules page."),
			Severity:    entity.RequirementWarning,
		}}
	}
}

// CheckWritableDir - a directory the site writes files to exists and
// takes a new file.
func CheckWritableDir(title string, dir func() string) StatusCheck {
	return func(ctx context.Context) []entity.Requirement {
		d := dir()
		r := entity.Requirement{Title: i18n.T(ctx, title), Value: d}
		if err := writableDir(d); err != nil {
			r.Severity = entity.RequirementError
			r.Description = i18n.T(ctx, "The site can't write files there: @error", "@error", err.Error())
		}
		return []entity.Requirement{r}
	}
}

// CheckWritableStore - the store the site keeps files in takes a new
// file (written and removed again). Its value is where the store keeps
// them (fmt.Stringer).
func CheckWritableStore(title string, store func() filestore.Store) StatusCheck {
	return func(ctx context.Context) []entity.Requirement {
		s := store()
		r := entity.Requirement{Title: i18n.T(ctx, title), Value: fmt.Sprint(s)}
		if err := writableStore(ctx, s); err != nil {
			r.Severity = entity.RequirementError
			r.Description = i18n.T(ctx, "The site can't write files there: @error", "@error", err.Error())
		}
		return []entity.Requirement{r}
	}
}

// writableStore - nil when a file can be stored in s and removed again.
func writableStore(ctx context.Context, s filestore.Store) error {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	key := ".status-check-" + hex.EncodeToString(b)
	if _, err := s.Put(ctx, key, strings.NewReader("ok"), filestore.PutOptions{Exclusive: true}); err != nil {
		return err
	}
	return s.Delete(ctx, key)
}

// writableDir - nil when dir exists and a file can be created in it.
func writableDir(dir string) error {
	if dir == "" {
		return os.ErrNotExist
	}
	f, err := os.CreateTemp(dir, ".status-check-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(filepath.Clean(name))
}

// CheckSiteURL - the site's address for absolute links is set
// (service.SiteURL); without it links are built from each request and
// e-mails with links aren't sent.
func CheckSiteURL() StatusCheck {
	return func(ctx context.Context) []entity.Requirement {
		r := entity.Requirement{Title: i18n.T(ctx, "Site address"), Value: entity.NewSystemSettings().Domain}
		if r.Value == "" {
			r.Value = i18n.T(ctx, "Not set")
			r.Severity = entity.RequirementWarning
			r.Description = i18n.T(ctx, "Sitemap, RSS and canonical links are built from each request's host, and e-mails with links (registration, password reset, notifications) are not sent. Set the address in <a href=\":url\">Site information</a>.", ":url", "/admin/config/system/site-information")
		}
		return []entity.Requirement{r}
	}
}

// CheckMail - the site can send e-mail (registration, password reset,
// notifications).
func CheckMail(status func() MailStatus) StatusCheck {
	return func(ctx context.Context) []entity.Requirement {
		st := status()
		r := entity.Requirement{Title: i18n.T(ctx, "Mail")}
		switch {
		case !st.Enabled:
			r.Value = i18n.T(ctx, "Off")
			r.Severity = entity.RequirementWarning
			r.Description = i18n.T(ctx, "The site sends no e-mail: registration confirmations and password resets don't arrive. Set it up on the <a href=\":url\">Mail</a> page.", ":url", "/admin/config/mail")
		case st.Source == MailSourceNone:
			r.Value = i18n.T(ctx, "Not configured")
			r.Severity = entity.RequirementWarning
			r.Description = i18n.T(ctx, "Sending is on but no transport is configured. Set it up on the <a href=\":url\">Mail</a> page.", ":url", "/admin/config/mail")
		default:
			r.Value = st.Transport
		}
		return []entity.Requirement{r}
	}
}

// CheckSecrets - the secrets kept in the database (the outside services'
// keys saved in the admin, the two-factor secrets) are sealed with the
// site's key: a warning without a key or with a guessable one (weak), an
// error for the ones the key doesn't open (unreadable: per what holds
// them, a catalog string).
func CheckSecrets(sealed, weak func() bool, unreadable func(ctx context.Context) (map[string]int, error)) StatusCheck {
	return func(ctx context.Context) []entity.Requirement {
		r := entity.Requirement{Title: i18n.T(ctx, "Secrets in the database")}
		if !sealed() {
			r.Value = i18n.T(ctx, "Not sealed")
			r.Severity = entity.RequirementWarning
			r.Description = i18n.T(ctx, "The keys saved in the admin (mail, CAPTCHA, social sign-in) and the users' two-factor secrets are kept unsealed: a copy of the database gives them away. Set SECRETS_KEY to a long random string (<code>openssl rand -base64 32</code>) and restart the site: it seals them at start.")
			return []entity.Requirement{r}
		}
		r.Value = i18n.T(ctx, "Sealed")
		bad, err := unreadable(ctx)
		if err != nil {
			r.Value = i18n.T(ctx, "Could not be checked")
			r.Description = html.EscapeString(err.Error())
			r.Severity = entity.RequirementWarning
			return []entity.Requirement{r}
		}
		if len(bad) == 0 {
			if weak() {
				r.Value = i18n.T(ctx, "The key is easy to guess")
				r.Severity = entity.RequirementWarning
				r.Description = i18n.T(ctx, "SECRETS_KEY is an example value or has too few different characters: a copy of the database could be opened by trying keys. Replace it with a long random string (<code>openssl rand -base64 32</code>): set SECRETS_KEY=new,old and restart - the site seals everything again with the new key; then drop the old one.")
			}
			return []entity.Requirement{r}
		}
		var where []string
		for label, n := range bad {
			where = append(where, fmt.Sprintf("%s (%d)", i18n.T(ctx, label), n))
		}
		sort.Strings(where)
		r.Value = i18n.T(ctx, "Some don't open")
		r.Severity = entity.RequirementError
		r.Description = i18n.T(ctx, "Sealed with a key the site no longer has: @where. If SECRETS_KEY was replaced, put the old key after the new one (SECRETS_KEY=new,old) and restart; otherwise enter them again in the admin (users set up two-factor sign-in again; their recovery codes still work).", "@where", html.EscapeString(strings.Join(where, ", ")))
		return []entity.Requirement{r}
	}
}

// CheckSQLMode - MySQL sessions run in the strict mode the engine expects
// (too long a value is an error, not silently cut); "server" turns it off.
func CheckSQLMode(driver, mode string) StatusCheck {
	return func(ctx context.Context) []entity.Requirement {
		if driver != "mysql" {
			return nil
		}
		r := entity.Requirement{Title: i18n.T(ctx, "Database SQL mode"), Value: mode}
		if strings.EqualFold(strings.TrimSpace(mode), "server") || !strings.Contains(strings.ToUpper(mode), "STRICT_") {
			r.Severity = entity.RequirementWarning
			r.Description = i18n.T(ctx, "Strict mode is off: a too long value is cut silently instead of being an error. Unset DB_SQL_MODE to use the engine's default.")
		}
		return []entity.Requirement{r}
	}
}

// CheckCacheRefresh - the background refresh of menus, blocks and View
// blocks keeps running: its last run is at most two intervals old.
func CheckCacheRefresh(last func() time.Time, interval time.Duration) StatusCheck {
	return func(ctx context.Context) []entity.Requirement {
		t := last()
		r := entity.Requirement{Title: i18n.T(ctx, "Background cache refresh")}
		switch {
		case t.IsZero():
			r.Value = i18n.T(ctx, "Not run yet")
			r.Severity = entity.RequirementWarning
		case interval > 0 && time.Since(t) > 2*interval:
			r.Value = i18n.T(ctx, "Last run @ago ago", "@ago", time.Since(t).Round(time.Minute).String())
			r.Severity = entity.RequirementWarning
			r.Description = i18n.T(ctx, "Menus, blocks and lists may show stale content. Check the service's log.")
		default:
			r.Value = i18n.T(ctx, "Last run @ago ago", "@ago", time.Since(t).Round(time.Second).String())
		}
		return []entity.Requirement{r}
	}
}
