package complaints

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/complaints/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/complaints/ent/generated/complaint"
	"github.com/kosatka-cms/kosatka/internal/modules/complaints/ent/generated/complaintsettings"
	"github.com/kosatka-cms/kosatka/internal/modules/complaints/ent/generated/complainttarget"
	"github.com/kosatka-cms/kosatka/internal/modules/complaints/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/modules/complaints/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// notesShown - the newest notes a target's row shows.
const notesShown = 3

// Repo - the module's tables: the targets, the complaints, the settings.
type Repo struct{ db *sql.DB }

// NewRepo - the module's tables on db.
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

// client - the module's ent client: the context's transaction
// (modulekit.InTx), else db.
func (r *Repo) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// Settings - the module's settings (the defaults while its row is
// missing).
func (r *Repo) Settings(ctx context.Context) (Settings, error) {
	row, err := r.client(ctx).ComplaintSettings.Get(ctx, manifest.SettingsRowID)
	if generated.IsNotFound(err) {
		return Settings{Comments: true, HideAfter: 3}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("loading the complaints' settings: %w", err)
	}
	return Settings{Comments: row.Comments, NodeTypes: row.NodeTypes, Reasons: row.Reasons, HideAfter: row.HideAfter}, nil
}

// SaveSettings stores the module's settings (the row made on the first
// save).
func (r *Repo) SaveSettings(ctx context.Context, s Settings) error {
	c := r.client(ctx)
	n, err := c.ComplaintSettings.Update().Where(complaintsettingsID()).
		SetComments(s.Comments).SetNodeTypes(s.NodeTypes).SetReasons(s.Reasons).SetHideAfter(max(s.HideAfter, 0)).Save(ctx)
	if err == nil && n == 0 {
		err = c.ComplaintSettings.Create().SetID(manifest.SettingsRowID).
			SetComments(s.Comments).SetNodeTypes(s.NodeTypes).SetReasons(s.Reasons).SetHideAfter(max(s.HideAfter, 0)).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the complaints' settings: %w", err)
	}
	return nil
}

// Add records userID's complaint about t's object (Kind, ObjectID,
// NodeID, AuthorID): its target made, or a resolved one back in the
// queue (its Hidden kept). added false - the visitor complained already:
// nothing changes. fresh - the target is new to the queue (made or back).
func (r *Repo) Add(ctx context.Context, t Target, userID int, reason, note string, now time.Time) (target Target, added, fresh bool, err error) {
	err = modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		c := r.client(ctx)
		row, err := c.ComplaintTarget.Query().
			Where(complainttarget.Kind(t.Kind), complainttarget.ObjectID(t.ObjectID)).Only(ctx)
		switch {
		case generated.IsNotFound(err):
			row, err = c.ComplaintTarget.Create().SetKind(t.Kind).SetObjectID(t.ObjectID).SetNodeID(t.NodeID).
				SetAuthorID(t.AuthorID).SetLastAt(now).SetCreatedAt(now).Save(ctx)
			if err != nil {
				return fmt.Errorf("adding the complaints' target: %w", err)
			}
			fresh = true
		case err != nil:
			return fmt.Errorf("loading the complaints' target: %w", err)
		}
		exists, err := c.Complaint.Query().Where(complaint.TargetID(row.ID), complaint.UserID(userID)).Exist(ctx)
		if err != nil {
			return fmt.Errorf("checking an earlier complaint: %w", err)
		}
		if exists {
			target = toTarget(row)
			return nil
		}
		if err := c.Complaint.Create().SetTargetID(row.ID).SetUserID(userID).SetReason(reason).
			SetComment(note).SetCreatedAt(now).Exec(ctx); err != nil {
			return fmt.Errorf("adding a complaint: %w", err)
		}
		u := c.ComplaintTarget.UpdateOneID(row.ID).SetLastAt(now)
		if row.Status == StatusResolved {
			u = u.SetStatus(StatusNew).SetResolution("").SetResolvedBy(0)
			fresh = true
		}
		if row, err = u.Save(ctx); err != nil {
			return fmt.Errorf("updating the complaints' target: %w", err)
		}
		target, added = toTarget(row), true
		return nil
	})
	return target, added, fresh, err
}

// CountSince - the complaints about targetID since t (zero - all).
func (r *Repo) CountSince(ctx context.Context, targetID int, since time.Time) (int, error) {
	q := r.client(ctx).Complaint.Query().Where(complaint.TargetID(targetID))
	if !since.IsZero() {
		q = q.Where(complaint.CreatedAtGT(since))
	}
	n, err := q.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting the complaints: %w", err)
	}
	return n, nil
}

// SetHidden marks whether the complaints hid the target's object.
func (r *Repo) SetHidden(ctx context.Context, id int, hidden bool) error {
	if err := r.client(ctx).ComplaintTarget.UpdateOneID(id).SetHidden(hidden).Exec(ctx); err != nil {
		return fmt.Errorf("marking the complaints' target %d hidden: %w", id, err)
	}
	return nil
}

// Resolve takes the target out of the queue: what by did, when.
func (r *Repo) Resolve(ctx context.Context, id int, resolution string, by int, at time.Time) error {
	err := r.client(ctx).ComplaintTarget.UpdateOneID(id).SetStatus(StatusResolved).SetResolution(resolution).
		SetResolvedBy(by).SetResolvedAt(at).SetHidden(false).Exec(ctx)
	if err != nil {
		return fmt.Errorf("resolving the complaints' target %d: %w", id, err)
	}
	return nil
}

// TargetByID - the target id with its complaints' count, reasons and
// notes (a not-found error - none).
func (r *Repo) TargetByID(ctx context.Context, id int) (*Target, error) {
	row, err := r.client(ctx).ComplaintTarget.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading the complaints' target %d: %w", id, notFound(err))
	}
	targets := []Target{toTarget(row)}
	if err := r.fill(ctx, targets); err != nil {
		return nil, err
	}
	return &targets[0], nil
}

// List - the targets of status (StatusNew - the queue, newest complaint
// first), a page of them, and how many there are.
func (r *Repo) List(ctx context.Context, status string, limit, offset int) ([]Target, int, error) {
	q := r.client(ctx).ComplaintTarget.Query().Where(complainttarget.Status(status))
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting the complaints' targets: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}
	rows, err := q.Order(generated.Desc(complainttarget.FieldLastAt), generated.Desc(complainttarget.FieldID)).
		Limit(limit).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("listing the complaints' targets: %w", err)
	}
	out := make([]Target, len(rows))
	for i, row := range rows {
		out[i] = toTarget(row)
	}
	return out, total, r.fill(ctx, out)
}

// DeleteByNode removes the targets of a content item and of its comments
// (the item deleted), their complaints with them.
func (r *Repo) DeleteByNode(ctx context.Context, nodeID int) error {
	if _, err := r.client(ctx).ComplaintTarget.Delete().Where(complainttarget.NodeID(nodeID)).Exec(ctx); err != nil {
		return fmt.Errorf("deleting the complaints about content %d: %w", nodeID, err)
	}
	return nil
}

// CountNew - the targets waiting in the queue (the admin menu's number).
func (r *Repo) CountNew(ctx context.Context) (int, error) {
	n, err := r.client(ctx).ComplaintTarget.Query().Where(complainttarget.Status(StatusNew)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting the complaints' queue: %w", err)
	}
	return n, nil
}

// fill sets targets' complaints' counts, reasons (most first) and newest
// notes.
func (r *Repo) fill(ctx context.Context, targets []Target) error {
	if len(targets) == 0 {
		return nil
	}
	ids := make([]int, len(targets))
	at := make(map[int]int, len(targets))
	for i, t := range targets {
		ids[i], at[t.ID] = t.ID, i
	}
	rows, err := r.client(ctx).Complaint.Query().Where(complaint.TargetIDIn(ids...)).
		Order(generated.Desc(complaint.FieldCreatedAt), generated.Desc(complaint.FieldID)).All(ctx)
	if err != nil {
		return fmt.Errorf("loading the complaints: %w", err)
	}
	reasons := make(map[int]map[string]int, len(targets))
	for _, c := range rows {
		t := &targets[at[c.TargetID]]
		t.Count++
		if reasons[c.TargetID] == nil {
			reasons[c.TargetID] = map[string]int{}
		}
		reasons[c.TargetID][c.Reason]++
		if c.Comment != "" && len(t.Notes) < notesShown {
			t.Notes = append(t.Notes, Note{UserID: c.UserID, Text: c.Comment, At: c.CreatedAt})
		}
	}
	for id, counts := range reasons {
		t := &targets[at[id]]
		for reason, n := range counts {
			t.Reasons = append(t.Reasons, ReasonCount{Reason: reason, Count: n})
		}
		sort.Slice(t.Reasons, func(i, j int) bool {
			if t.Reasons[i].Count != t.Reasons[j].Count {
				return t.Reasons[i].Count > t.Reasons[j].Count
			}
			return t.Reasons[i].Reason < t.Reasons[j].Reason
		})
	}
	return nil
}

// toTarget - a target row as the module's Target (no counts yet).
func toTarget(row *generated.ComplaintTarget) Target {
	return Target{ID: row.ID, Kind: row.Kind, ObjectID: row.ObjectID, NodeID: row.NodeID, AuthorID: row.AuthorID,
		Status: row.Status, Hidden: row.Hidden, Resolution: row.Resolution, ResolvedBy: row.ResolvedBy,
		ResolvedAt: row.ResolvedAt, LastAt: row.LastAt, CreatedAt: row.CreatedAt}
}

// notFound - err as the core's not-found error when ent found nothing.
func notFound(err error) error {
	if generated.IsNotFound(err) {
		return fmt.Errorf("%w: %w", modulekit.ErrEntityNotFound, err)
	}
	return err
}

// complaintsettingsID - the settings row's condition.
func complaintsettingsID() predicate.ComplaintSettings {
	return complaintsettings.ID(manifest.SettingsRowID)
}
