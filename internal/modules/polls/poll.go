package polls

import (
	"context"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// Poll - a poll, a node of the poll content type (its question is the
// node's title): its description, how many answers a vote may choose,
// when it is open, and its options.
type Poll struct {
	ID            int
	Node          modulekit.Node
	Description   string
	MaxAnswers    int
	AvailableFrom *time.Time
	AvailableTo   *time.Time
	ResultsOnly   bool
	AllowRevote   bool // false - one vote per user, no changing it
	Options       PollOptions
}

// PollOptions - a poll's answer options, in their order.
type PollOptions []PollOption

// PollOption - an answer option of a poll: its label and its place among
// the options (Weight).
type PollOption struct {
	ID     int
	PollID int
	Label  string
	Weight int
}

// PollOptionCount is an option with aggregated vote count for display.
type PollOptionCount struct {
	PollOption
	Count    int
	Percent  float64 // 0-100 when TotalVotes > 0
	Selected bool    // current user voted for this option
}

// PollVoteAuditRow is one row of the vote log (for admins/moderators).
type PollVoteAuditRow struct {
	UserID      int
	Username    string
	UserSlug    string
	OptionLabel string
	VotedAt     time.Time
}

// PollView aggregates data for the poll detail page.
type PollView struct {
	Poll         Poll
	OptionCounts []PollOptionCount
	TotalVotes   int
	CanVote      bool
	ShowForm     bool
	BeforeStart  bool
	AfterEnd     bool
}

// PollRepository - the store of polls: their nodes, options and votes.
type PollRepository interface {
	ByNodeID(ctx context.Context, nodeID int) (*Poll, error)
	BySlug(ctx context.Context, slug string) (*Poll, error)
	Create(ctx context.Context, p *Poll) (*Poll, error)
	Update(ctx context.Context, p *Poll) (*Poll, error)
	VoteCounts(ctx context.Context, pollID int) (map[int]int, int, error)
	UserOptionVotes(ctx context.Context, pollID, userID int) ([]int, error)
	SaveUserVotes(ctx context.Context, pollID, userID int, optionIDs []int, revote bool) (bool, error)
	VoteAuditRows(ctx context.Context, pollID int) ([]PollVoteAuditRow, error)
	BlockTargets(ctx context.Context) ([]modulekit.PluginBlockTarget, error)
}
