package ratings

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/modules/ratings/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The module's criteria in Views (their keys; manifest's, which its demo
// uses too).
const (
	keyWeighted = manifest.ViewSortWeighted
	keyAverage  = manifest.Name + ".average"
	keyCount    = manifest.ViewFilterCount
	keyRating   = manifest.ViewFieldRating
)

// The params of the criteria.
const (
	// paramMinVotes - the weighted rating's threshold: the votes a node
	// needs for its own average to count fully.
	paramMinVotes = manifest.ViewParamMinVotes
	// paramLook - what the rating field shows.
	paramLook = manifest.ViewParamLook
)

// The looks of the rating field.
const (
	lookFull    = "full"
	lookAverage = manifest.ViewLookAverage
	lookCount   = "count"
)

// registerViews adds the module's sorts (the weighted rating, the
// average, the number of votes), filters (the average, the number of
// votes) and field (the rating) to content Views.
func registerViews(s *Service, render func(w *bytes.Buffer, block string, data any) error) {
	minVotes := modulekit.ViewParam{Key: paramMinVotes, Label: i18n.N("Votes for full weight"), Type: modulekit.PluginBlockSettingNumber, Default: "5", Min: 1, Max: 10000}
	modulekit.AddViewSort(modulekit.ViewSort{Module: manifest.Name, Key: keyWeighted, Label: i18n.N("Rating (weighted)"), Params: []modulekit.ViewParam{minVotes},
		Values: func(ctx context.Context, ids []int, params map[string]string) (map[int]float64, error) {
			n, _ := strconv.Atoi(params[paramMinVotes])
			return s.WeightedAverages(ctx, ids, n)
		}})
	modulekit.AddViewSort(modulekit.ViewSort{Module: manifest.Name, Key: keyAverage, Label: i18n.N("Rating: average"),
		Values: func(ctx context.Context, ids []int, _ map[string]string) (map[int]float64, error) {
			return s.Averages(ctx, ids)
		}})
	modulekit.AddViewSort(modulekit.ViewSort{Module: manifest.Name, Key: keyCount, Label: i18n.N("Rating: number of votes"),
		Values: func(ctx context.Context, ids []int, _ map[string]string) (map[int]float64, error) {
			return s.Counts(ctx, ids)
		}})

	modulekit.AddViewFilter(modulekit.ViewFilter{Module: manifest.Name, Key: keyAverage, Label: i18n.N("Rating: average"), Values: s.Averages})
	modulekit.AddViewFilter(modulekit.ViewFilter{Module: manifest.Name, Key: keyCount, Label: i18n.N("Rating: number of votes"), Values: s.Counts})

	look := modulekit.ViewParam{Key: paramLook, Label: i18n.N("What it shows"), Type: modulekit.PluginBlockSettingSelect, Default: lookFull,
		Options: []modulekit.PluginBlockTarget{
			{Value: lookFull, Label: i18n.N("Stars, average and number of votes")},
			{Value: lookAverage, Label: i18n.N("The average only")},
			{Value: lookCount, Label: i18n.N("The number of votes only")},
		}}
	modulekit.AddViewField(modulekit.ViewField{Module: manifest.Name, Key: keyRating, Label: i18n.N("Rating"), Params: []modulekit.ViewParam{look},
		Render: func(ctx context.Context, ids []int, params map[string]string) (map[int]template.HTML, error) {
			sums, err := s.Summaries(ctx, ids, params[paramLook])
			if err != nil {
				return nil, err
			}
			out := make(map[int]template.HTML, len(sums))
			var buf bytes.Buffer
			for id, sum := range sums {
				buf.Reset()
				if err := render(&buf, "ratings_summary", sum); err != nil {
					return nil, fmt.Errorf("rendering the rating of node %d: %w", id, err)
				}
				out[id] = template.HTML(buf.String()) // #nosec G203 -- the module's own template, escaped by it
			}
			return out, nil
		}})
}
