package actionlog

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// TestKindLabel - a content's type by its label, a module's type too (the
// labels are the site's and the modules' types); a type gone since - its
// machine name; the core's kinds by name, a module's own kind - none.
func TestKindLabel(t *testing.T) {
	c := &Controller{}
	labels := map[string]string{"article": "Article", "forum": "Forum topic"}
	for _, tc := range []struct {
		o    modulekit.ActionObject
		want string
	}{
		{modulekit.ActionObject{Kind: "node", Subtype: "forum"}, "Forum topic"},
		{modulekit.ActionObject{Kind: "node", Subtype: "article"}, "Article"},
		{modulekit.ActionObject{Kind: "node", Subtype: "gone"}, "gone"},
		{modulekit.ActionObject{Kind: "user"}, "User"},
		{modulekit.ActionObject{Kind: "gallery"}, ""},
	} {
		if got := c.kindLabel(context.Background(), tc.o, labels); got != tc.want {
			t.Errorf("%+v: %q, want %q", tc.o, got, tc.want)
		}
	}
}
