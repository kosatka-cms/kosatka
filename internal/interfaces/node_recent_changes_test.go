package interfaces

import (
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// TestByDay - the revisions by their day in the zone (a save late in the
// evening by UTC is the next day east of it), today and yesterday marked,
// each with its time of day.
func TestByDay(t *testing.T) {
	loc := time.FixedZone("MSK", 3*3600)
	now := time.Date(2026, 10, 25, 20, 0, 0, 0, time.UTC)
	row := func(at time.Time) service.RecentRow {
		return service.RecentRow{RecentRevision: entity.RecentRevision{NodeRevision: entity.NodeRevision{CreatedAt: at}}}
	}
	days := byDay([]service.RecentRow{
		row(time.Date(2026, 10, 25, 22, 30, 0, 0, time.UTC)), // the 26th in MSK
		row(time.Date(2026, 10, 25, 10, 5, 0, 0, time.UTC)),
		row(time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)),
		row(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)),
	}, loc, now)
	if len(days) != 4 {
		t.Fatalf("days: %+v", days)
	}
	if d := days[0]; d.Today || d.Day.Day() != 26 || d.Rows[0].Time != "01:30" {
		t.Errorf("the next day in MSK: %+v", d)
	}
	if d := days[1]; !d.Today || d.Rows[0].Time != "13:05" {
		t.Errorf("today: %+v", d)
	}
	if !days[2].Yesterday || days[3].Today || days[3].Yesterday {
		t.Errorf("yesterday and earlier: %+v", days)
	}
}
