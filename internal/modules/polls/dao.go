package polls

import (
	"sort"

	"github.com/kosatka-cms/kosatka/internal/modules/polls/ent/generated"
)

// ToPollEntity - a poll row with its options (its node - the core's -
// set by the repository).
func ToPollEntity(p *generated.Poll) *Poll {
	if p == nil {
		return nil
	}
	out := &Poll{
		ID:            p.ID,
		Description:   p.Description,
		MaxAnswers:    p.MaxAnswers,
		AvailableFrom: p.AvailableFrom,
		AvailableTo:   p.AvailableTo,
		ResultsOnly:   p.ResultsOnly,
		AllowRevote:   p.AllowRevote,
	}
	if len(p.Edges.Options) > 0 {
		out.Options = make(PollOptions, 0, len(p.Edges.Options))
		for _, o := range p.Edges.Options {
			out.Options = append(out.Options, PollOption{
				ID:     o.ID,
				PollID: o.PollID,
				Label:  o.Label,
				Weight: o.Weight,
			})
		}
		sort.Slice(out.Options, func(i, j int) bool {
			if out.Options[i].Weight != out.Options[j].Weight {
				return out.Options[i].Weight < out.Options[j].Weight
			}
			return out.Options[i].ID < out.Options[j].ID
		})
	}
	return out
}
