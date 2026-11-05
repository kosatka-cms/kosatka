package polls

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/modules/polls/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/polls/ent/generated/poll"
	"github.com/kosatka-cms/kosatka/internal/modules/polls/ent/generated/polloption"
	"github.com/kosatka-cms/kosatka/internal/modules/polls/ent/generated/pollvote"
	"github.com/kosatka-cms/kosatka/internal/modules/polls/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// NewPollRepository - the polls on db, on the module's own ent schema
// (ent/); a poll's node and its voters are the core's (nodes, users).
func NewPollRepository(db *sql.DB, nodes modulekit.NodesRepository, users modulekit.UserRepository) *EntPollRepository {
	return &EntPollRepository{db: db, nodes: nodes, users: users}
}

// EntPollRepository - the polls: their rows in the module's tables, each
// one's node in the core's, written together in one transaction
// (modulekit.InTx: the core's repository and the module's client).
type EntPollRepository struct {
	db    *sql.DB
	nodes modulekit.NodesRepository
	users modulekit.UserRepository
}

// client - the module's client for ctx: in its transaction, if any.
func (r EntPollRepository) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// ByNodeID returns the node's poll with its options in their order, its
// node and the node's page SEO.
func (r EntPollRepository) ByNodeID(ctx context.Context, nodeID int) (*Poll, error) {
	res, err := r.client(ctx).Poll.Query().
		Where(poll.Nid(nodeID)).
		WithOptions(func(oq *generated.PollOptionQuery) {
			oq.Order(generated.Asc(polloption.FieldWeight), generated.Asc(polloption.FieldID))
		}).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("poll with node ID %d: %w", nodeID, err)
	}
	p := ToPollEntity(res)
	node, err := r.nodes.ByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("the node of poll %d: %w", res.ID, err)
	}
	if node.PageSEO, err = r.nodes.PageSEO(ctx, nodeID); err != nil {
		return nil, fmt.Errorf("the page SEO of poll %d: %w", res.ID, err)
	}
	p.Node = *node
	return p, nil
}

// BlockTargets - the polls, the newest first: node ID → question.
func (r EntPollRepository) BlockTargets(ctx context.Context) ([]modulekit.PluginBlockTarget, error) {
	nodes, err := r.nodes.ByType(ctx, string(manifest.ContentType))
	if err != nil {
		return nil, err
	}
	out := make([]modulekit.PluginBlockTarget, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, modulekit.PluginBlockTarget{Value: strconv.Itoa(n.ID), Label: n.Title})
	}
	return out, nil
}

// BySlug - the poll whose node has the address slug.
func (r EntPollRepository) BySlug(ctx context.Context, slug string) (*Poll, error) {
	node, err := r.nodes.BySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("poll with slug %s: %w", slug, err)
	}
	if node.Type != manifest.ContentType {
		return nil, fmt.Errorf("%w: poll with slug %s", modulekit.ErrEntityNotFound, slug)
	}
	return r.ByNodeID(ctx, node.ID)
}

// Create stores the poll's node, poll row and options in one transaction.
func (r EntPollRepository) Create(ctx context.Context, p *Poll) (*Poll, error) {
	var nodeID int
	err := modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		n := p.Node
		n.Type = manifest.ContentType
		var err error
		if nodeID, err = r.nodes.CreateRow(ctx, n); err != nil {
			return err
		}
		client := r.client(ctx)
		pollCreate := client.Poll.Create().
			SetNid(nodeID).
			SetDescription(p.Description).
			SetMaxAnswers(p.MaxAnswers).
			SetResultsOnly(p.ResultsOnly).
			SetAllowRevote(p.AllowRevote)
		if p.AvailableFrom != nil {
			pollCreate = pollCreate.SetAvailableFrom(*p.AvailableFrom)
		}
		if p.AvailableTo != nil {
			pollCreate = pollCreate.SetAvailableTo(*p.AvailableTo)
		}
		createdPoll, err := pollCreate.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create poll: %w", err)
		}
		for i, opt := range p.Options {
			if _, err := client.PollOption.Create().
				SetPollID(createdPoll.ID).
				SetLabel(opt.Label).
				SetWeight(i).
				Save(ctx); err != nil {
				return fmt.Errorf("failed to create poll option: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.ByNodeID(ctx, nodeID)
}

// Update stores the poll row, its node and its options in one
// transaction (an option with votes is kept even when removed).
func (r EntPollRepository) Update(ctx context.Context, p *Poll) (*Poll, error) {
	err := modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		client := r.client(ctx)
		existing, err := client.Poll.Query().Where(poll.Nid(p.Node.ID)).Only(ctx)
		if err != nil {
			return err
		}

		u := client.Poll.UpdateOneID(existing.ID).
			SetDescription(p.Description).
			SetMaxAnswers(p.MaxAnswers).
			SetResultsOnly(p.ResultsOnly).
			SetAllowRevote(p.AllowRevote).
			ClearAvailableFrom().
			ClearAvailableTo()
		if p.AvailableFrom != nil {
			u = u.SetAvailableFrom(*p.AvailableFrom)
		}
		if p.AvailableTo != nil {
			u = u.SetAvailableTo(*p.AvailableTo)
		}
		if _, err = u.Save(ctx); err != nil {
			return err
		}
		if err := r.nodes.UpdateRow(ctx, p.Node); err != nil {
			return err
		}

		existingOpts, err := client.PollOption.Query().
			Where(polloption.PollID(existing.ID)).
			All(ctx)
		if err != nil {
			return err
		}
		submitted := make(map[int]struct{})
		for _, opt := range p.Options {
			if opt.ID > 0 {
				submitted[opt.ID] = struct{}{}
			}
		}
		for _, ex := range existingOpts {
			if _, ok := submitted[ex.ID]; ok {
				continue
			}
			n, err := client.PollVote.Query().Where(pollvote.OptionID(ex.ID)).Count(ctx)
			if err != nil {
				return err
			}
			if n > 0 {
				continue
			}
			if err := client.PollOption.DeleteOneID(ex.ID).Exec(ctx); err != nil {
				return err
			}
		}

		for i, opt := range p.Options {
			w := i
			if opt.ID > 0 {
				// This poll's option only: the IDs come from the form.
				_, err = client.PollOption.Update().
					Where(polloption.ID(opt.ID), polloption.PollID(existing.ID)).
					SetLabel(opt.Label).
					SetWeight(w).
					Save(ctx)
				if err != nil {
					return err
				}
			} else {
				_, err = client.PollOption.Create().
					SetPollID(existing.ID).
					SetLabel(opt.Label).
					SetWeight(w).
					Save(ctx)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.ByNodeID(ctx, p.Node.ID)
}

// VoteCounts - the poll's votes per option and in all, counted by the
// database (a poll may have many).
func (r EntPollRepository) VoteCounts(ctx context.Context, pollID int) (map[int]int, int, error) {
	var rows []struct {
		OptionID int `json:"option_id"`
		Count    int `json:"count"`
	}
	if err := r.client(ctx).PollVote.Query().
		Where(pollvote.PollID(pollID)).
		GroupBy(pollvote.FieldOptionID).
		Aggregate(generated.Count()).
		Scan(ctx, &rows); err != nil {
		return nil, 0, err
	}
	counts := make(map[int]int, len(rows))
	total := 0
	for _, row := range rows {
		counts[row.OptionID] = row.Count
		total += row.Count
	}
	return counts, total, nil
}

// UserOptionVotes returns the options the user voted for in the poll
// (none - not voted).
func (r EntPollRepository) UserOptionVotes(ctx context.Context, pollID, userID int) ([]int, error) {
	rows, err := r.client(ctx).PollVote.Query().
		Where(pollvote.PollID(pollID), pollvote.UserID(userID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.OptionID)
	}
	return out, nil
}

// SaveUserVotes stores the user's votes in the poll, in one transaction
// holding the poll's row (lockPoll): with revote, instead of their earlier
// ones; without it, only if they have none yet (saved false - they voted
// already). Two votes of one user at once don't both pass the check, nor
// leave both sets.
func (r EntPollRepository) SaveUserVotes(ctx context.Context, pollID, userID int, optionIDs []int, revote bool) (saved bool, err error) {
	err = modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		if err := r.lockPoll(ctx, pollID); err != nil {
			return err
		}
		client := r.client(ctx)
		if !revote {
			voted, err := client.PollVote.Query().Where(pollvote.PollID(pollID), pollvote.UserID(userID)).Exist(ctx)
			if err != nil || voted {
				return err
			}
		}
		if _, err := client.PollVote.Delete().
			Where(pollvote.PollID(pollID), pollvote.UserID(userID)).
			Exec(ctx); err != nil {
			return err
		}
		now := time.Now()
		for _, oid := range optionIDs {
			if err := client.PollVote.Create().
				SetPollID(pollID).
				SetUserID(userID).
				SetOptionID(oid).
				SetCreatedAt(now).
				Exec(ctx); err != nil {
				return err
			}
		}
		saved = true
		return nil
	})
	return saved, err
}

// lockPoll locks the poll's row until ctx's transaction ends (SELECT … FOR
// UPDATE, MySQL and PostgreSQL alike): the votes in it are saved one at a
// time.
func (r EntPollRepository) lockPoll(ctx context.Context, pollID int) error {
	var rows entsql.Rows
	if err := modulekit.Driver(ctx, r.db).Query(ctx, modulekit.Rebind("SELECT id FROM polls WHERE id = ? FOR UPDATE"), []any{pollID}, &rows); err != nil {
		return fmt.Errorf("locking poll %d: %w", pollID, err)
	}
	return rows.Close()
}

// VoteAuditRows returns the poll's votes, newest first, with their
// voters' names and the options' labels.
func (r EntPollRepository) VoteAuditRows(ctx context.Context, pollID int) ([]PollVoteAuditRow, error) {
	rows, err := r.client(ctx).PollVote.Query().
		Where(pollvote.PollID(pollID)).
		WithPollOption().
		Order(generated.Desc(pollvote.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, v := range rows {
		ids = append(ids, v.UserID)
	}
	voters, err := r.users.ByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("the voters of poll %d: %w", pollID, err)
	}
	out := make([]PollVoteAuditRow, 0, len(rows))
	for _, v := range rows {
		row := PollVoteAuditRow{
			UserID:  v.UserID,
			VotedAt: v.CreatedAt,
		}
		if u, ok := voters[v.UserID]; ok {
			row.Username = u.Username
			row.UserSlug = u.Slug
		}
		if v.Edges.PollOption != nil {
			row.OptionLabel = v.Edges.PollOption.Label
		}
		out = append(out, row)
	}
	return out, nil
}

var _ PollRepository = (*EntPollRepository)(nil)
