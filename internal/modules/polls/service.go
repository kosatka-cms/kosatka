package polls

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/polls/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

func pollOptionPercent(count, total int) float64 {
	if total <= 0 || count <= 0 {
		return 0
	}
	return 100.0 * float64(count) / float64(total)
}

// PollService - the polls: their pages' data and the votes in them.
type PollService interface {
	ByNodeID(ctx context.Context, nodeID int) (*Poll, error)
	BySlug(ctx context.Context, slug string) (*Poll, error)
	Create(ctx context.Context, p *Poll) (*Poll, error)
	Update(ctx context.Context, p *Poll) (*Poll, error)
	BuildView(ctx context.Context, p *Poll, userID int, loggedIn bool) (*PollView, error)
	Vote(ctx context.Context, nodeID, userID int, optionIDs []int) error
	VoteAudit(ctx context.Context, nodeID int) (*Poll, []PollVoteAuditRow, error)
	// BlockTargets - the polls the "Poll" block can show (its placement
	// page): node ID → question.
	BlockTargets(ctx context.Context) ([]modulekit.PluginBlockTarget, error)
}

// DefaultPollService - PollService on a PollRepository.
type DefaultPollService struct {
	repository PollRepository
	nodes      modulekit.NodesService
	logger     *slog.Logger
}

// NewPollService - the polls service on repository; nodes makes a new
// poll's address from its question when it has none (GenerateUniqueSlug).
func NewPollService(
	repository PollRepository,
	nodes modulekit.NodesService,
	baseLogger *slog.Logger,
) PollService {
	return &DefaultPollService{
		repository: repository,
		nodes:      nodes,
		logger:     baseLogger.With("component", "poll.service"),
	}
}

// ByNodeID - the node's poll with its options and its node.
func (s *DefaultPollService) ByNodeID(ctx context.Context, nodeID int) (*Poll, error) {
	return s.repository.ByNodeID(ctx, nodeID)
}

// BlockTargets - see PollService.BlockTargets.
func (s *DefaultPollService) BlockTargets(ctx context.Context) ([]modulekit.PluginBlockTarget, error) {
	return s.repository.BlockTargets(ctx)
}

// BySlug - the poll whose node has the address slug (a node of another
// type - not found).
func (s *DefaultPollService) BySlug(ctx context.Context, slug string) (*Poll, error) {
	p, err := s.repository.BySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Create stores a new poll (a slug from its title when it has none); the
// action log gets "Content created".
func (s *DefaultPollService) Create(ctx context.Context, p *Poll) (*Poll, error) {
	if p.Node.Slug == "" {
		slug, err := s.nodes.GenerateUniqueSlug(ctx, p.Node.Title, manifest.ContentType)
		if err != nil {
			return nil, err
		}
		p.Node.Slug = slug
	}
	if p.MaxAnswers < 1 {
		p.MaxAnswers = 1
	}
	created, err := s.repository.Create(ctx, p)
	if err != nil {
		return nil, err
	}
	if saved, err := s.repository.ByNodeID(ctx, created.Node.ID); err == nil {
		modulekit.RecordNodeCreated(ctx, saved.Node)
	} else {
		modulekit.LogFallback(s.logger, err, "the created poll for the action log", "nodeID", created.Node.ID)
	}
	return created, nil
}

// Update stores the poll (an empty slug keeps the old one); what it
// changed of the content (status, author, address) goes to the action log.
func (s *DefaultPollService) Update(ctx context.Context, p *Poll) (*Poll, error) {
	if p.MaxAnswers < 1 {
		p.MaxAnswers = 1
	}
	prev, err := s.repository.ByNodeID(ctx, p.Node.ID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.Node.Slug) == "" {
		p.Node.Slug = prev.Node.Slug
	}
	updated, err := s.repository.Update(ctx, p)
	if err != nil {
		return nil, err
	}
	if saved, err := s.repository.ByNodeID(ctx, p.Node.ID); err == nil {
		modulekit.RecordNodeSaved(ctx, prev.Node, saved.Node)
	} else {
		modulekit.LogFallback(s.logger, err, "the saved poll for the action log", "nodeID", p.Node.ID)
	}
	return updated, nil
}

// BuildView - the poll's page data for the user (loggedIn false - a
// visitor): the votes per option and in all, the user's choices, whether
// they can vote and get the form, whether the poll is yet to open or over.
func (s *DefaultPollService) BuildView(ctx context.Context, p *Poll, userID int, loggedIn bool) (*PollView, error) {
	counts, total, err := s.repository.VoteCounts(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	var userOpts []int
	if loggedIn && userID > 0 {
		userOpts, err = s.repository.UserOptionVotes(ctx, p.ID, userID)
		if err != nil {
			return nil, err
		}
	}

	selected := make(map[int]struct{})
	for _, id := range userOpts {
		selected[id] = struct{}{}
	}
	optionCounts := make([]PollOptionCount, 0, len(p.Options))
	for _, o := range p.Options {
		_, isSel := selected[o.ID]
		c := counts[o.ID]
		optionCounts = append(optionCounts, PollOptionCount{
			PollOption: o,
			Count:      c,
			Percent:    pollOptionPercent(c, total),
			Selected:   isSel,
		})
	}

	now := time.Now()
	beforeStart := p.AvailableFrom != nil && now.Before(*p.AvailableFrom)
	afterEnd := p.AvailableTo != nil && !p.AvailableTo.IsZero() && now.After(*p.AvailableTo)
	inWindow := !beforeStart && !afterEnd

	published := p.Node.IsPublic()
	canVote := loggedIn && !p.ResultsOnly && inWindow && published
	alreadyVoted := len(userOpts) > 0
	showForm := canVote && (p.AllowRevote || !alreadyVoted)

	view := &PollView{
		Poll:         *p,
		OptionCounts: optionCounts,
		TotalVotes:   total,
		CanVote:      canVote,
		ShowForm:     showForm,
		BeforeStart:  beforeStart,
		AfterEnd:     afterEnd,
	}
	return view, nil
}

// The refusals of a vote (Vote).
var (
	// ErrPollVoteForbidden - no user, a poll of results only, outside its
	// dates or not public.
	ErrPollVoteForbidden = errors.New("poll vote not allowed")
	// ErrPollVoteInvalid - no options, more than MaxAnswers, one twice or one
	// not of the poll.
	ErrPollVoteInvalid = errors.New("poll vote invalid options")
	// ErrPollRevoteNotAllowed - the user has voted and the poll doesn't let a
	// vote change.
	ErrPollRevoteNotAllowed = errors.New("poll revote not allowed")
)

// Vote stores the user's choice of options in the node's poll instead of
// their earlier one; a refused vote is ErrPollVoteForbidden,
// ErrPollVoteInvalid or ErrPollRevoteNotAllowed.
func (s *DefaultPollService) Vote(ctx context.Context, nodeID, userID int, optionIDs []int) error {
	if userID <= 0 {
		return ErrPollVoteForbidden
	}
	p, err := s.repository.ByNodeID(ctx, nodeID)
	if err != nil {
		return err
	}
	now := time.Now()
	if p.ResultsOnly {
		return ErrPollVoteForbidden
	}
	if p.AvailableFrom != nil && now.Before(*p.AvailableFrom) {
		return ErrPollVoteForbidden
	}
	if p.AvailableTo != nil && !p.AvailableTo.IsZero() && now.After(*p.AvailableTo) {
		return ErrPollVoteForbidden
	}
	if !p.Node.IsPublic() {
		return ErrPollVoteForbidden
	}

	if len(optionIDs) == 0 {
		return ErrPollVoteInvalid
	}
	if len(optionIDs) > p.MaxAnswers {
		return ErrPollVoteInvalid
	}
	seen := make(map[int]struct{})
	for _, id := range optionIDs {
		if _, ok := seen[id]; ok {
			return ErrPollVoteInvalid
		}
		seen[id] = struct{}{}
		ok := false
		for _, o := range p.Options {
			if o.ID == id {
				ok = true
				break
			}
		}
		if !ok {
			return ErrPollVoteInvalid
		}
	}

	saved, err := s.repository.SaveUserVotes(ctx, p.ID, userID, optionIDs, p.AllowRevote)
	if err != nil {
		return err
	}
	if !saved {
		return ErrPollRevoteNotAllowed
	}
	return nil
}

// VoteAudit - the node's poll and its votes one by one, newest first: who
// chose which option and when.
func (s *DefaultPollService) VoteAudit(ctx context.Context, nodeID int) (*Poll, []PollVoteAuditRow, error) {
	p, err := s.repository.ByNodeID(ctx, nodeID)
	if err != nil {
		return nil, nil, err
	}
	rows, err := s.repository.VoteAuditRows(ctx, p.ID)
	if err != nil {
		return nil, nil, err
	}
	return p, rows, nil
}
