package entity

import (
	"context"
	"time"
)

// DashboardStats - the figures of the admin's Overview: the published
// content, the comments and the users, all and since the start of the
// recent period; the content's views, today and all.
type DashboardStats struct {
	Published, PublishedSince int
	Comments, CommentsSince   int
	Users, UsersSince         int
	ViewsToday, ViewsTotal    int
}

// PopularNode - a published content item with its views in a period
// (Views) and all its views (Total).
type PopularNode struct {
	Node  Node
	Views int
	Total int
}

// PopularPeriod - the period the popular content is counted over.
type PopularPeriod string

const (
	// PopularPeriodToday - the site's day (node_counter.daycount).
	PopularPeriodToday PopularPeriod = "today"
	// PopularPeriodWeek - the last 7 days, today's included (the views
	// by day).
	PopularPeriodWeek PopularPeriod = "week"
	// PopularPeriodMonth - the last 30 days, today's included.
	PopularPeriodMonth PopularPeriod = "month"
	// PopularPeriodAll - for ever (node_counter.totalcount).
	PopularPeriodAll PopularPeriod = "all"
)

// PopularPeriods - the periods in the order the report offers them.
var PopularPeriods = []PopularPeriod{PopularPeriodToday, PopularPeriodWeek, PopularPeriodMonth, PopularPeriodAll}

// ParsePopularPeriod - p if it's a period, else PopularPeriodToday.
func ParsePopularPeriod(p string) PopularPeriod {
	for _, known := range PopularPeriods {
		if string(known) == p {
			return known
		}
	}
	return PopularPeriodToday
}

// Days - how many days of the views by day the period sums, today's
// included; 0 - it isn't counted from them (today, for ever).
func (p PopularPeriod) Days() int {
	switch p {
	case PopularPeriodWeek:
		return 7
	case PopularPeriodMonth:
		return 30
	}
	return 0
}

// PopularQuery - which popular content: of Period, counted from Today
// (the start of the site's day), of the content type Type ("" - of any),
// a page of it (Limit, Offset).
type PopularQuery struct {
	Period        PopularPeriod
	Today         time.Time
	Type          string
	Limit, Offset int
}

// DashboardRepository - what the admin's Overview counts.
type DashboardRepository interface {
	// Stats - the figures; since - the start of the recent period,
	// today - of the site's day (a view counts today from it).
	Stats(ctx context.Context, since, today time.Time) (DashboardStats, error)
	// OnModeration - how many content items wait on moderation (the
	// "Pending moderation" menu item's count).
	OnModeration(ctx context.Context) (int, error)
	// Popular - the published content viewed most in q's period, most
	// first, a page of it; and how many items there are in all.
	Popular(ctx context.Context, q PopularQuery) ([]PopularNode, int, error)
}
