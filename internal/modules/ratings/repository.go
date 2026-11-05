package ratings

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/ratings/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/ratings/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/modules/ratings/ent/generated/ratingcontenttype"
	"github.com/kosatka-cms/kosatka/internal/modules/ratings/ent/generated/ratingsettings"
	"github.com/kosatka-cms/kosatka/internal/modules/ratings/ent/generated/ratingvote"
	"github.com/kosatka-cms/kosatka/internal/modules/ratings/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// statsBatch - node IDs per query of Stats (a View may rate every node
// it matches).
const statsBatch = 1000

// Repository - the module's tables: its settings, the rated types, the
// votes (a type's count joins the core's nodes).
// A built-in module, tested with the engine, may read the core's tables
// in its SQL; a module of its own repository goes through the SDK's
// services instead (docs/modules/sdk.md): the core's columns aren't its
// API.
type Repository struct {
	db *sql.DB
}

// NewRepository creates the repository on db.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// client - the module's client for ctx: in its transaction, if any.
func (r *Repository) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// Settings - the module's settings (the defaults without the row).
func (r *Repository) Settings(ctx context.Context) (Settings, error) {
	row, err := r.client(ctx).RatingSettings.Get(ctx, manifest.SettingsRowID)
	if generated.IsNotFound(err) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("loading the ratings' settings: %w", err)
	}
	return Settings{Scale: row.Scale, AllowGuests: row.AllowGuests, AllowChange: row.AllowChange, AllowWithdraw: row.AllowWithdraw}, nil
}

// SaveSettings writes the module's settings (its row made if missing).
func (r *Repository) SaveSettings(ctx context.Context, s Settings) error {
	c := r.client(ctx)
	// Whether the row exists is asked, not taken from the UPDATE's count:
	// MySQL without clientFoundRows counts only the rows it changed.
	exists, err := c.RatingSettings.Query().Where(ratingsettings.ID(manifest.SettingsRowID)).Exist(ctx)
	if err == nil && exists {
		err = c.RatingSettings.UpdateOneID(manifest.SettingsRowID).
			SetScale(s.Scale).SetAllowGuests(s.AllowGuests).SetAllowChange(s.AllowChange).SetAllowWithdraw(s.AllowWithdraw).Exec(ctx)
	} else if err == nil {
		err = c.RatingSettings.Create().SetID(manifest.SettingsRowID).
			SetScale(s.Scale).SetAllowGuests(s.AllowGuests).SetAllowChange(s.AllowChange).SetAllowWithdraw(s.AllowWithdraw).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the ratings' settings: %w", err)
	}
	return nil
}

// RatedTypes - the machine names of the rated content types.
func (r *Repository) RatedTypes(ctx context.Context) ([]string, error) {
	types, err := r.client(ctx).RatingContentType.Query().Order(generated.Asc(ratingcontenttype.FieldContentType)).
		Select(ratingcontenttype.FieldContentType).Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the rated content types: %w", err)
	}
	return types, nil
}

// SetTypeRated switches rating content of contentType on or off (its
// votes stay).
func (r *Repository) SetTypeRated(ctx context.Context, contentType string, on bool) error {
	c := r.client(ctx)
	if !on {
		if _, err := c.RatingContentType.Delete().Where(ratingcontenttype.ContentType(contentType)).Exec(ctx); err != nil {
			return fmt.Errorf("switching rating of %s off: %w", contentType, err)
		}
		return nil
	}
	exists, err := c.RatingContentType.Query().Where(ratingcontenttype.ContentType(contentType)).Exist(ctx)
	if err != nil || exists {
		return err
	}
	if err := c.RatingContentType.Create().SetContentType(contentType).Exec(ctx); err != nil {
		return fmt.Errorf("switching rating of %s on: %w", contentType, err)
	}
	return nil
}

// Stats - the votes of each node among ids that has any.
func (r *Repository) Stats(ctx context.Context, ids []int) (map[int]Stats, error) {
	out := make(map[int]Stats, len(ids))
	for start := 0; start < len(ids); start += statsBatch {
		batch := ids[start:min(start+statsBatch, len(ids))]
		args := make([]any, len(batch))
		for i, id := range batch {
			args[i] = id
		}
		query := modulekit.Rebind(`SELECT nid, COUNT(*), AVG(score * 1.0 / scale) FROM rating_votes WHERE nid IN (` +
			strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",") + `) GROUP BY nid`)
		if err := r.scanStats(ctx, query, args, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repository) scanStats(ctx context.Context, query string, args []any, out map[int]Stats) error {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("loading the nodes' votes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var nid int
		var s Stats
		if err := rows.Scan(&nid, &s.Count, &s.Share); err != nil {
			return fmt.Errorf("reading the nodes' votes: %w", err)
		}
		out[nid] = s
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading the nodes' votes: %w", err)
	}
	return nil
}

// SiteShare - the average share of every vote on the site (0 - none).
func (r *Repository) SiteShare(ctx context.Context) (float64, error) {
	var share sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `SELECT AVG(score * 1.0 / scale) FROM rating_votes`).Scan(&share); err != nil {
		return 0, fmt.Errorf("averaging the site's votes: %w", err)
	}
	return share.Float64, nil
}

// TypeTotals - the votes of contentType's nodes and how many nodes have
// them.
func (r *Repository) TypeTotals(ctx context.Context, contentType string) (votes, nodes int, err error) {
	err = r.db.QueryRowContext(ctx, modulekit.Rebind(`SELECT COUNT(*), COUNT(DISTINCT v.nid) FROM rating_votes v JOIN nodes n ON n.id = v.nid WHERE n.type = ?`), contentType).
		Scan(&votes, &nodes)
	if err != nil {
		return 0, 0, fmt.Errorf("counting the votes of %s: %w", contentType, err)
	}
	return votes, nodes, nil
}

// voterIs - the vote of voter for nid.
func voterIs(nid int, v Voter) predicate.RatingVote {
	if v.UserID > 0 {
		return ratingvote.And(ratingvote.Nid(nid), ratingvote.UID(v.UserID))
	}
	return ratingvote.And(ratingvote.Nid(nid), ratingvote.Guest(v.Guest))
}

// VoteOf - voter's vote for nid; false - none.
func (r *Repository) VoteOf(ctx context.Context, nid int, v Voter) (Vote, bool, error) {
	if !v.Known() {
		return Vote{}, false, nil
	}
	row, err := r.client(ctx).RatingVote.Query().Where(voterIs(nid, v)).Only(ctx)
	if generated.IsNotFound(err) {
		return Vote{}, false, nil
	}
	if err != nil {
		return Vote{}, false, fmt.Errorf("loading a vote for node %d: %w", nid, err)
	}
	return Vote{Score: row.Score, Scale: row.Scale}, true, nil
}

// SaveVote sets voter's vote for nid, replacing theirs.
func (r *Repository) SaveVote(ctx context.Context, nid int, v Voter, vote Vote) error {
	c := r.client(ctx)
	now := time.Now()
	n, err := c.RatingVote.Update().Where(voterIs(nid, v)).SetScore(vote.Score).SetScale(vote.Scale).SetUpdatedAt(now).Save(ctx)
	if err != nil {
		return fmt.Errorf("changing a vote for node %d: %w", nid, err)
	}
	if n > 0 {
		return nil
	}
	create := c.RatingVote.Create().SetNid(nid).SetScore(vote.Score).SetScale(vote.Scale).SetCreatedAt(now).SetUpdatedAt(now)
	if v.UserID > 0 {
		create.SetUID(v.UserID)
	} else {
		create.SetGuest(v.Guest)
	}
	err = create.Exec(ctx)
	// The same voter's second request at once: theirs is there now.
	if generated.IsConstraintError(err) {
		_, err = c.RatingVote.Update().Where(voterIs(nid, v)).SetScore(vote.Score).SetScale(vote.Scale).SetUpdatedAt(now).Save(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving a vote for node %d: %w", nid, err)
	}
	return nil
}

// DeleteVote takes voter's vote for nid back.
func (r *Repository) DeleteVote(ctx context.Context, nid int, v Voter) error {
	if _, err := r.client(ctx).RatingVote.Delete().Where(voterIs(nid, v)).Exec(ctx); err != nil {
		return fmt.Errorf("taking back a vote for node %d: %w", nid, err)
	}
	return nil
}
