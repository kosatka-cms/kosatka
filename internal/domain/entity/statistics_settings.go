package entity

import (
	"context"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// StatisticsSettings is the row from statistics_settings (view counting
// toggle, how long the views by day are kept).
type StatisticsSettings struct {
	ViewCountingEnabled bool
	// DailyViewsKeepDays - how many days the views by day are kept (the
	// popular content of a period); 0 - for ever.
	DailyViewsKeepDays int
}

// StatisticsSettingsRepository persists site-wide statistics preferences.
type StatisticsSettingsRepository interface {
	Get(ctx context.Context) (*StatisticsSettings, error)
	Update(ctx context.Context, s *StatisticsSettings) error
}

// NodeCounterRepository updates node_counter view aggregates.
type NodeCounterRepository interface {
	// IncrementView counts a view of the node: its totals and its views
	// of the site's day.
	IncrementView(ctx context.Context, nid int) error
	// DeleteViewDaysBefore drops the views by day older than day
	// (ViewDay).
	DeleteViewDaysBefore(ctx context.Context, day int) error
}

// ViewDay - t's day of the site (its time zone) as the views by day keep
// it: YYYYMMDD.
func ViewDay(t time.Time) int {
	t = t.In(sitetime.Location())
	return t.Year()*10000 + int(t.Month())*100 + t.Day()
}
