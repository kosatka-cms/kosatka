package actionlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/kosatka-cms/kosatka/internal/modules/action_log/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/action_log/ent/generated/actionlogentry"
	"github.com/kosatka-cms/kosatka/internal/modules/action_log/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Entry - a recorded action: when, its type and group, who (ActorID 0 -
// nobody signed in; ActorName - their name at the time), from where (IP,
// "" - not kept), what it was done to and what it changed.
type Entry struct {
	ID        int
	At        time.Time
	Type      string
	Group     string
	ActorID   int
	ActorName string
	IP        string
	Object    modulekit.ActionObject
	Changes   []modulekit.ActionChange
}

// Filter - what the log shows: Actor - a part of who's name, ActorID -
// one user's actions; Group - one group; Object - a part of the object's
// name, or its ID; ObjectKind and ObjectID - one object (the content's
// own log); From, To - the days (zero - no limit).
type Filter struct {
	Actor      string
	ActorID    int
	Group      string
	Object     string
	ObjectKind string
	ObjectID   string
	From, To   time.Time
}

// Settings - the module's settings: KeepDays - how long entries are kept
// (0 - forever); LogIP - whether the address is kept; GroupsOff - the
// groups not recorded.
type Settings struct {
	KeepDays  int
	LogIP     bool
	GroupsOff []string
}

// defaultSettings - the settings until the page is saved.
func defaultSettings() Settings { return Settings{KeepDays: 90, LogIP: true} }

// Off - whether the group isn't recorded.
func (s Settings) Off(group string) bool {
	for _, g := range s.GroupsOff {
		if g == group {
			return true
		}
	}
	return false
}

// Repo - the module's tables.
type Repo struct {
	client *generated.Client
}

// NewRepo creates a Repo on db, in the repositories' dialect. Its client
// is the database's own, never a transaction's: an action is recorded in
// the acting request, maybe inside the core's change, and the log's write
// isn't part of it - failing (on PostgreSQL a failed statement ends the
// transaction), it would fail the change.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{client: generated.NewClient(generated.Driver(entsql.OpenDB(modulekit.DBDialect(), db)))}
}

// Add stores an entry.
func (r *Repo) Add(ctx context.Context, e Entry) error {
	changes := ""
	if len(e.Changes) > 0 {
		b, err := json.Marshal(e.Changes)
		if err != nil {
			return fmt.Errorf("encoding the changes: %w", err)
		}
		changes = string(b)
	}
	c := r.client.ActionLogEntry.Create().SetCreatedAt(e.At).SetType(e.Type).SetActionGroup(e.Group).
		SetActorName(cut(e.ActorName, 255)).SetIP(cut(e.IP, 64)).
		SetObjectKind(cut(e.Object.Kind, 32)).SetObjectID(cut(e.Object.ID, 64)).
		SetObjectLabel(cut(e.Object.Label, 512)).SetObjectType(cut(e.Object.Subtype, 255)).SetObjectURL(cut(e.Object.URL, 512)).
		SetChanges(changes)
	if e.ActorID > 0 {
		c.SetActorID(e.ActorID)
	}
	if err := c.Exec(ctx); err != nil {
		return fmt.Errorf("adding an action log entry (%s): %w", e.Type, err)
	}
	return nil
}

// where - the filter as predicates.
func (f Filter) where() []predicate.ActionLogEntry {
	var w []predicate.ActionLogEntry
	if s := strings.TrimSpace(f.Actor); s != "" {
		w = append(w, actionlogentry.ActorNameContainsFold(s))
	}
	if f.ActorID > 0 {
		w = append(w, actionlogentry.ActorID(f.ActorID))
	}
	if f.Group != "" {
		w = append(w, actionlogentry.ActionGroup(f.Group))
	}
	if s := strings.TrimSpace(f.Object); s != "" {
		w = append(w, actionlogentry.Or(actionlogentry.ObjectLabelContainsFold(s), actionlogentry.ObjectID(s)))
	}
	if f.ObjectKind != "" {
		w = append(w, actionlogentry.ObjectKind(f.ObjectKind), actionlogentry.ObjectID(f.ObjectID))
	}
	if !f.From.IsZero() {
		w = append(w, actionlogentry.CreatedAtGTE(f.From))
	}
	if !f.To.IsZero() {
		w = append(w, actionlogentry.CreatedAtLT(f.To.AddDate(0, 0, 1)))
	}
	return w
}

// List returns the entries the filter shows, newest first, a page at a
// time, with how many there are.
func (r *Repo) List(ctx context.Context, f Filter, limit, offset int) ([]Entry, int, error) {
	q := r.client.ActionLogEntry.Query().Where(f.where()...)
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting action log entries: %w", err)
	}
	rows, err := q.Order(generated.Desc(actionlogentry.FieldCreatedAt), generated.Desc(actionlogentry.FieldID)).
		Limit(limit).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("listing action log entries: %w", err)
	}
	out := make([]Entry, len(rows))
	for i, row := range rows {
		e := Entry{ID: row.ID, At: row.CreatedAt, Type: row.Type, Group: row.ActionGroup, ActorName: row.ActorName, IP: row.IP,
			Object: modulekit.ActionObject{Kind: row.ObjectKind, ID: row.ObjectID, Label: row.ObjectLabel, Subtype: row.ObjectType, URL: row.ObjectURL}}
		if row.ActorID != nil {
			e.ActorID = *row.ActorID
		}
		if row.Changes != "" {
			// A broken list is shown as none: the entry itself stays.
			_ = json.Unmarshal([]byte(row.Changes), &e.Changes)
		}
		out[i] = e
	}
	return out, total, nil
}

// Count - how many entries the filter shows.
func (r *Repo) Count(ctx context.Context, f Filter) (int, error) {
	n, err := r.client.ActionLogEntry.Query().Where(f.where()...).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting action log entries: %w", err)
	}
	return n, nil
}

// Purge deletes the entries older than before; returns how many.
func (r *Repo) Purge(ctx context.Context, before time.Time) (int, error) {
	n, err := r.client.ActionLogEntry.Delete().Where(actionlogentry.CreatedAtLT(before)).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("purging the action log: %w", err)
	}
	return n, nil
}

// Settings returns the module's settings (the defaults until saved).
func (r *Repo) Settings(ctx context.Context) (Settings, error) {
	row, err := r.client.ActionLogSettings.Get(ctx, 1)
	if generated.IsNotFound(err) {
		return defaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("loading the action log settings: %w", err)
	}
	return Settings{KeepDays: row.KeepDays, LogIP: row.LogIP, GroupsOff: strings.Fields(row.GroupsOff)}, nil
}

// SaveSettings saves the module's settings.
func (r *Repo) SaveSettings(ctx context.Context, s Settings) error {
	off := strings.Join(s.GroupsOff, " ")
	err := r.client.ActionLogSettings.UpdateOneID(1).SetKeepDays(s.KeepDays).SetLogIP(s.LogIP).SetGroupsOff(off).Exec(ctx)
	if generated.IsNotFound(err) {
		err = r.client.ActionLogSettings.Create().SetID(1).SetKeepDays(s.KeepDays).SetLogIP(s.LogIP).SetGroupsOff(off).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the action log settings: %w", err)
	}
	return nil
}

// cut - s within n characters (the column's), a character whole.
func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
