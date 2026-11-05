package subscriptions

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
)

type describeUsers map[int]modulekit.User

func (u describeUsers) ByIDs(context.Context, []int) (map[int]modulekit.User, error) { return u, nil }

type describeNodes map[int]modulekit.Node

func (n describeNodes) ByIDs(context.Context, []int) (map[int]modulekit.Node, error) { return n, nil }

type describeTerms struct{}

func (describeTerms) TermsByIDs(context.Context, []int) (map[int]modulekit.TaxonomyTerm, error) {
	return nil, nil
}

// TestDescribeHiddenNode - unread comments on content the visitor may
// not view leave no item and no group to add to; the visible content's
// comments are one item.
func TestDescribeHiddenNode(t *testing.T) {
	d := describeDeps{
		users: describeUsers{7: {ID: 7, Username: "ann"}},
		nodes: describeNodes{2: {ID: 2, Title: "Shown"}},
		terms: describeTerms{},
	}
	ns := []Notification{
		{ID: 1, Kind: KindComment, NodeID: 1, ActorID: 7},
		{ID: 2, Kind: KindComment, NodeID: 1, ActorID: 7},
		{ID: 3, Kind: KindComment, NodeID: 2, ActorID: 7},
		{ID: 4, Kind: KindComment, NodeID: 2, ActorID: 7},
	}
	items, err := describe(context.Background(), d, ns, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].NodeTitle != "Shown" || items[0].Count != 2 {
		t.Fatalf("items: %+v", items)
	}
}
