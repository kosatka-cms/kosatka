package entity

import "time"

// FeedNodeFilter - the materials a feed takes: published, not on
// moderation, of Types, tagged with any of TermIDs (none - any), created
// at or after Since (zero - any time), newest first, at most Limit.
type FeedNodeFilter struct {
	Types   []string
	TermIDs []int
	Since   time.Time
	Limit   int
}
