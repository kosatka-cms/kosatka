package admin

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestPickableViews - a menu link's Views pages: the enabled ones, and a
// disabled one only when the link points to it already.
func TestPickableViews(t *testing.T) {
	views := entity.ViewDisplayDefs{{ID: 1, Status: 1}, {ID: 2, Status: 0}, {ID: 3, Status: 1}}
	ids := func(vs entity.ViewDisplayDefs) []int {
		var out []int
		for _, v := range vs {
			out = append(out, v.ID)
		}
		return out
	}
	if got := ids(pickableViews(views, 0)); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Errorf("a new link: %v, want the enabled 1 and 3", got)
	}
	if got := ids(pickableViews(views, 2)); len(got) != 3 || got[1] != 2 {
		t.Errorf("a link to the disabled 2: %v, want it kept in place", got)
	}
}
