package subscriptions

import (
	"context"
	"fmt"
	"slices"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/textcut"
)

// excerptLength - the runes of a comment an item quotes.
const excerptLength = 160

// describeDeps - what an item's names come from.
type describeDeps struct {
	users interface {
		ByIDs(ctx context.Context, ids []int) (map[int]modulekit.User, error)
	}
	nodes interface {
		ByIDs(ctx context.Context, ids []int) (map[int]modulekit.Node, error)
	}
	comments notifierComments
	terms    interface {
		TermsByIDs(ctx context.Context, ids []int) (map[int]modulekit.TaxonomyTerm, error)
	}
}

// Item - a notification as shown: on the Notifications tab (where the
// comments of one content item are one item: Count, Actors) and in an
// e-mail.
type Item struct {
	Notification
	Actor     string
	Actors    []string
	Count     int
	NodeTitle string
	NodeURL   string
	TermName  string
	Excerpt   string
}

// describe - ns as items, newest first as ns is; group - the unread
// comments of one content item become one item (the tab). A
// notification of content ctx's queries don't see (deleted, or content
// its visitor may not view - entity.VisitorOf) is left out.
func describe(ctx context.Context, d describeDeps, ns []Notification, group bool) ([]Item, error) {
	var userIDs, nodeIDs, termIDs []int
	for _, n := range ns {
		if n.ActorID > 0 {
			userIDs = append(userIDs, n.ActorID)
		}
		nodeIDs = append(nodeIDs, n.NodeID)
		if n.TermID > 0 {
			termIDs = append(termIDs, n.TermID)
		}
	}
	users, err := d.users.ByIDs(ctx, slices.Compact(slices.Sorted(slices.Values(userIDs))))
	if err != nil {
		return nil, fmt.Errorf("the notifications' users: %w", err)
	}
	nodes, err := d.nodes.ByIDs(ctx, slices.Compact(slices.Sorted(slices.Values(nodeIDs))))
	if err != nil {
		return nil, fmt.Errorf("the notifications' content: %w", err)
	}
	terms, err := d.terms.TermsByIDs(ctx, slices.Compact(slices.Sorted(slices.Values(termIDs))))
	if err != nil {
		return nil, fmt.Errorf("the notifications' terms: %w", err)
	}
	var out []Item
	groups := map[int]int{} // node → its unread comments' item
	for _, n := range ns {
		node, ok := nodes[n.NodeID]
		if !ok {
			// Content gone, or one ctx's visitor may not view: no item,
			// none to group into either.
			continue
		}
		actor := users[n.ActorID].Username
		if group && n.Kind == KindComment && !n.Read {
			if i, ok := groups[n.NodeID]; ok {
				it := &out[i]
				it.Count++
				if actor != "" && !slices.Contains(it.Actors, actor) {
					it.Actors = append(it.Actors, actor)
				}
				continue
			}
			groups[n.NodeID] = len(out)
		}
		it := Item{Notification: n, Actor: actor, Count: 1, NodeTitle: node.Title, NodeURL: node.URL(),
			TermName: terms[n.TermID].Name}
		if actor != "" {
			it.Actors = []string{actor}
		}
		if n.CommentID > 0 {
			it.NodeURL += fmt.Sprintf("#comment-%d", n.CommentID)
			if c, err := d.comments.ByID(ctx, n.CommentID); err == nil && c != nil {
				it.Excerpt = textcut.Ellipsis(htmlsanitize.StripToText(c.Content), excerptLength)
			}
		}
		out = append(out, it)
	}
	return out, nil
}
