// Package ratings - star ratings of content: a voter's vote for a content
// item, its average and count on the item's page (the node_rating block)
// and in Views (the module's sorts, filters and field). Which types are
// rated, the scale and whether a vote may change are the module's
// settings; who may vote - the "rate content" permission, and guests when
// the settings let them.
package ratings

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// The scale a site may set (Settings.Scale).
const (
	MinScale     = 3
	MaxScale     = 10
	DefaultScale = 5
)

// Settings - the module's settings.
type Settings struct {
	// Scale - the stars of a vote. Votes keep the scale they were given
	// on: changing it keeps each vote's share (4 of 5 is 8 of 10).
	Scale int
	// AllowGuests - guests may rate (each by their session), besides the
	// users with the "rate content" permission.
	AllowGuests   bool
	AllowChange   bool
	AllowWithdraw bool
}

// DefaultSettings - the settings of a new install.
func DefaultSettings() Settings {
	return Settings{Scale: DefaultScale, AllowChange: true, AllowWithdraw: true}
}

// ValidScale - whether n is a scale the site may set.
func ValidScale(n int) bool { return n >= MinScale && n <= MaxScale }

// Stats - a node's votes: how many, and their average as a share of the
// scale (0 to 1), so the site's scale may change.
type Stats struct {
	Count int
	Share float64
}

// Average - the average on scale.
func (s Stats) Average(scale int) float64 { return s.Share * float64(scale) }

// Weighted - the share a top list ranks by: the node's average pulled to
// the site's (siteShare) while it has fewer votes than minVotes - two
// fives don't outrank forty votes of 4.8. A Bayesian average:
// (v·R + m·C) / (v + m).
func Weighted(s Stats, siteShare float64, minVotes int) float64 {
	v, m := float64(s.Count), float64(max(minVotes, 0))
	if v+m == 0 {
		return 0
	}
	return (v*s.Share + m*siteShare) / (v + m)
}

// Voter - who votes: a user, or a guest by their key (Guest; the hash of
// their browser's cookie).
type Voter struct {
	UserID int
	Guest  string
}

// Known - whether the voter can be told apart (a user, or a guest with a
// key).
func (v Voter) Known() bool { return v.UserID > 0 || v.Guest != "" }

// Vote - a voter's vote: score stars of the scale it was given on.
type Vote struct {
	Score int
	Scale int
}

// On - the vote on scale, rounded to a whole star (at least 1).
func (v Vote) On(scale int) int {
	if v.Scale <= 0 {
		return 0
	}
	return max(1, int(math.Round(float64(v.Score)*float64(scale)/float64(v.Scale))))
}

// The errors of voting (Service.Vote/Withdraw).
var (
	// ErrNotRated - the node's type isn't rated, or the node can't be seen.
	ErrNotRated = errors.New("this content is not rated")
	// ErrScore - not a score of the scale.
	ErrScore = errors.New("not a score of the scale")
	// ErrFinal - the voter has voted and votes can't change.
	ErrFinal = errors.New("a vote can't be changed")
	// ErrWithdraw - votes can't be taken back.
	ErrWithdraw = errors.New("a vote can't be taken back")
)

// formatAverage - v with one decimal, its separator sep ("4.6", "4,6").
func formatAverage(v float64, sep string) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', 1, 64), ".", sep, 1)
}
