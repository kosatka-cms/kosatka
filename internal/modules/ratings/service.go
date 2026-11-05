package ratings

import (
	"context"
	"fmt"
	"slices"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Service - voting and what a page shows of a node's rating.
type Service struct {
	repo *Repository
}

// NewService creates the service on repo.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// Settings - the module's settings.
func (s *Service) Settings(ctx context.Context) (Settings, error) { return s.repo.Settings(ctx) }

// SaveSettings writes the module's settings (a scale out of range - the
// default).
func (s *Service) SaveSettings(ctx context.Context, st Settings) error {
	if !ValidScale(st.Scale) {
		st.Scale = DefaultScale
	}
	return s.repo.SaveSettings(ctx, st)
}

// RatedTypes - the machine names of the rated content types.
func (s *Service) RatedTypes(ctx context.Context) ([]string, error) { return s.repo.RatedTypes(ctx) }

// Rated - whether content of nodeType is rated.
func (s *Service) Rated(ctx context.Context, nodeType string) (bool, error) {
	types, err := s.repo.RatedTypes(ctx)
	return slices.Contains(types, nodeType), err
}

// SetRatedTypes rates exactly the types of on among all (the others of
// all switched off).
func (s *Service) SetRatedTypes(ctx context.Context, all []string, on []string) error {
	for _, t := range all {
		if err := s.repo.SetTypeRated(ctx, t, slices.Contains(on, t)); err != nil {
			return err
		}
	}
	return nil
}

// SetTypeRated switches rating of one content type on or off (its votes
// stay).
func (s *Service) SetTypeRated(ctx context.Context, contentType string, on bool) error {
	return s.repo.SetTypeRated(ctx, contentType, on)
}

// TypeTotals - the votes of contentType's content and how many items have
// them.
func (s *Service) TypeTotals(ctx context.Context, contentType string) (votes, nodes int, err error) {
	return s.repo.TypeTotals(ctx, contentType)
}

// Vote gives voter's score (1 to the scale) to node, replacing their
// vote while votes may change (the same score again is no change).
func (s *Service) Vote(ctx context.Context, node modulekit.Node, v Voter, score int) error {
	st, err := s.ratedSettings(ctx, node, v)
	if err != nil {
		return err
	}
	if score < 1 || score > st.Scale {
		return ErrScore
	}
	prev, found, err := s.repo.VoteOf(ctx, node.ID, v)
	if err != nil {
		return err
	}
	if found && !st.AllowChange {
		if prev.On(st.Scale) == score {
			return nil
		}
		return ErrFinal
	}
	return s.repo.SaveVote(ctx, node.ID, v, Vote{Score: score, Scale: st.Scale})
}

// Withdraw takes voter's vote for node back, while votes may be taken
// back.
func (s *Service) Withdraw(ctx context.Context, node modulekit.Node, v Voter) error {
	st, err := s.ratedSettings(ctx, node, v)
	if err != nil {
		return err
	}
	if !st.AllowWithdraw {
		return ErrWithdraw
	}
	return s.repo.DeleteVote(ctx, node.ID, v)
}

// ratedSettings - the settings, when node is rated and the voter known.
func (s *Service) ratedSettings(ctx context.Context, node modulekit.Node, v Voter) (Settings, error) {
	if !v.Known() {
		return Settings{}, ErrNotRated
	}
	rated, err := s.Rated(ctx, string(node.Type))
	if err != nil {
		return Settings{}, err
	}
	if !rated {
		return Settings{}, ErrNotRated
	}
	return s.repo.Settings(ctx)
}

// Widget - what the node_rating block shows: the average and the count,
// the visitor's own vote and what they may do.
type Widget struct {
	NodeID int
	// NodeURL - the node's address (the log in link's destination).
	NodeURL string
	Scale   int
	Count   int
	// Average - the average on the scale, formatted ("4.6"); Fill - the
	// share of the stars filled, in percent.
	Average string
	Fill    float64
	// Stars - 1 to Scale, the vote buttons.
	Stars []int
	// Mine - the visitor's vote on the scale (0 - none).
	Mine int
	// CanVote - the visitor may rate (and hasn't voted, or may change
	// it); CanWithdraw - may take theirs back; LogIn - a guest who may
	// not rate, offered to log in.
	CanVote     bool
	CanWithdraw bool
	LogIn       bool
}

// Summary - the average and count of a node, as a View's field shows
// them.
type Summary struct {
	Scale   int
	Count   int
	Average string
	Fill    float64
	// Look - what the field shows (lookFull, lookAverage, lookCount).
	Look string
}

// MayRate - whether the visitor may rate: a user with the permission
// (userAllowed), a guest when guests may.
func (s *Service) MayRate(ctx context.Context, guest, userAllowed bool) (bool, error) {
	if !guest {
		return userAllowed, nil
	}
	st, err := s.repo.Settings(ctx)
	return st.AllowGuests, err
}

// WidgetFor - node's widget for the visitor (v, may rate: allowed; logIn -
// a guest who may not, offered to log in); false - its type isn't
// rated.
func (s *Service) WidgetFor(ctx context.Context, node modulekit.Node, v Voter, allowed, logIn bool) (Widget, bool, error) {
	rated, err := s.Rated(ctx, string(node.Type))
	if err != nil || !rated {
		return Widget{}, false, err
	}
	st, err := s.repo.Settings(ctx)
	if err != nil {
		return Widget{}, false, err
	}
	stats, err := s.repo.Stats(ctx, []int{node.ID})
	if err != nil {
		return Widget{}, false, err
	}
	w := Widget{NodeID: node.ID, Scale: st.Scale, LogIn: logIn}
	w.Count, w.Average, w.Fill = summaryOf(ctx, stats[node.ID], st.Scale)
	for i := 1; i <= st.Scale; i++ {
		w.Stars = append(w.Stars, i)
	}
	if allowed {
		mine, found, err := s.repo.VoteOf(ctx, node.ID, v)
		if err != nil {
			return Widget{}, false, err
		}
		if found {
			w.Mine = mine.On(st.Scale)
		}
		w.CanVote = !found || st.AllowChange
		w.CanWithdraw = found && st.AllowWithdraw
	}
	return w, true, nil
}

// summaryOf - the count, the formatted average and the fill of stats.
func summaryOf(ctx context.Context, stats Stats, scale int) (count int, average string, fill float64) {
	if stats.Count == 0 {
		return 0, "", 0
	}
	return stats.Count, formatAverage(stats.Average(scale), decimalSeparator(ctx)), stats.Share * 100
}

// decimalSeparator - the site language's (a catalog's translation).
func decimalSeparator(ctx context.Context) string {
	return i18n.TC(ctx, "decimal separator", ".")
}

// Summaries - the summary of each node among ids that has votes.
func (s *Service) Summaries(ctx context.Context, ids []int, look string) (map[int]Summary, error) {
	st, err := s.repo.Settings(ctx)
	if err != nil {
		return nil, err
	}
	stats, err := s.repo.Stats(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int]Summary, len(stats))
	for id, n := range stats {
		sum := Summary{Scale: st.Scale, Look: look}
		sum.Count, sum.Average, sum.Fill = summaryOf(ctx, n, st.Scale)
		out[id] = sum
	}
	return out, nil
}

// Averages - the average on the scale of each node among ids that has
// votes.
func (s *Service) Averages(ctx context.Context, ids []int) (map[int]float64, error) {
	st, err := s.repo.Settings(ctx)
	if err != nil {
		return nil, err
	}
	stats, err := s.repo.Stats(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int]float64, len(stats))
	for id, n := range stats {
		out[id] = n.Average(st.Scale)
	}
	return out, nil
}

// Counts - the votes of every node among ids (0 - none).
func (s *Service) Counts(ctx context.Context, ids []int) (map[int]float64, error) {
	stats, err := s.repo.Stats(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int]float64, len(ids))
	for _, id := range ids {
		out[id] = float64(stats[id].Count)
	}
	return out, nil
}

// WeightedAverages - the weighted rating on the scale (Weighted) of each
// node among ids that has votes; minVotes - the votes a node needs to
// count fully.
func (s *Service) WeightedAverages(ctx context.Context, ids []int, minVotes int) (map[int]float64, error) {
	st, err := s.repo.Settings(ctx)
	if err != nil {
		return nil, err
	}
	stats, err := s.repo.Stats(ctx, ids)
	if err != nil {
		return nil, err
	}
	site, err := s.repo.SiteShare(ctx)
	if err != nil {
		return nil, fmt.Errorf("the weighted rating: %w", err)
	}
	out := make(map[int]float64, len(stats))
	for id, n := range stats {
		out[id] = Weighted(n, site, minVotes) * float64(st.Scale)
	}
	return out, nil
}
