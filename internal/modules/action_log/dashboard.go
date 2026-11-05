package actionlog

import (
	"context"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// dashboardSize - the entries of the Overview's block.
const dashboardSize = 5

// dashboardRows - the newest entries for the admin's Overview
// (modulekit.AddDashboardBlock): each with its action as named and its
// tone.
func dashboardRows(repo *Repo) func(ctx context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
		entries, _, err := repo.List(ctx, Filter{}, dashboardSize, 0)
		if err != nil {
			return nil, err
		}
		rows := make([]row, len(entries))
		for i, e := range entries {
			rows[i] = row{Entry: e, Action: e.Type}
			if t, ok := modulekit.ActionTypeOf(e.Type); ok {
				rows[i].Action, rows[i].Tone = i18n.T(ctx, t.Label), t.Tone
			}
		}
		return rows, nil
	}
}
