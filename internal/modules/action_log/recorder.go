package actionlog

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// Recorder keeps the actions the site records (modulekit.OnAction) while
// the module is on, as its settings say: the groups turned off aren't
// kept, nor the address unless asked to.
type Recorder struct {
	repo   *Repo
	users  modulekit.UserRepository
	logger *slog.Logger
	now    func() time.Time

	mu       sync.Mutex
	settings *Settings
}

// NewRecorder creates a Recorder.
func NewRecorder(repo *Repo, users modulekit.UserRepository, logger *slog.Logger) *Recorder {
	return &Recorder{repo: repo, users: users, logger: logger, now: time.Now}
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
		modulekit.LogFallback(r.logger, err, "action log settings (the defaults used)")
		return defaultSettings()
	}
	r.mu.Lock()
	r.settings = &st
	r.mu.Unlock()
	return st
}

// SaveSettings saves the settings.
func (r *Recorder) SaveSettings(ctx context.Context, st Settings) error {
	if err := r.repo.SaveSettings(ctx, st); err != nil {
		return err
	}
	r.mu.Lock()
	r.settings = &st
	r.mu.Unlock()
	return nil
}

// Record keeps the action e (the listener of modulekit.OnAction). Its
// failure is logged: the action itself is done.
func (r *Recorder) Record(ctx context.Context, e modulekit.ActionEvent) {
	group := ""
	if t, ok := modulekit.ActionTypeOf(e.Type); ok {
		group = t.Group
	}
	st := r.Settings(ctx)
	if st.Off(group) {
		return
	}
	entry := Entry{At: e.At, Type: e.Type, Group: group, ActorID: e.Actor.UserID, Object: e.Object, Changes: e.Changes}
	if st.LogIP {
		entry.IP = e.Actor.IP
	}
	if e.Actor.UserID > 0 && r.users != nil {
		if u, err := r.users.ByID(ctx, e.Actor.UserID); err == nil && u != nil {
			entry.ActorName = u.Username
		}
	}
	if err := r.repo.Add(ctx, entry); err != nil && r.logger != nil {
		r.logger.Error("recording an action", "error", err, "type", e.Type)
	}
}

// Purge deletes the entries older than the settings keep them - the
// module's background job.
func (r *Recorder) Purge(ctx context.Context) error {
	days := r.Settings(ctx).KeepDays
	if days <= 0 {
		return nil
	}
	_, err := r.repo.Purge(ctx, r.now().AddDate(0, 0, -days))
	return err
}
