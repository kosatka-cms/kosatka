package subscriptions

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// notifierNodes / notifierComments / notifierTerms - what the notifier
// reads of the core.
type notifierNodes interface {
	ByID(ctx context.Context, nodeID int) (*modulekit.Node, error)
}

type notifierComments interface {
	ByID(ctx context.Context, id int) (*modulekit.Comment, error)
}

type notifierTerms interface {
	FieldTermIDsForNode(ctx context.Context, nodeID int) ([]int, error)
}

// Notifier turns the core's events into notifications: a comment posted
// (CommentPosted) and a content item becoming public (NodePublished).
type Notifier struct {
	repo     *Repo
	nodes    notifierNodes
	comments notifierComments
	terms    notifierTerms
	now      func() time.Time
}

// NewNotifier - the notifier writing to repo; nodes, comments and terms
// are the core's, read for the events.
func NewNotifier(repo *Repo, nodes notifierNodes, comments notifierComments, terms notifierTerms) *Notifier {
	return &Notifier{repo: repo, nodes: nodes, comments: comments, terms: terms, now: time.Now}
}

// CommentPosted tells of comment c: the author of the comment it replies
// to (unless they turned the replies off), then the content's followers;
// nobody twice, nor its own author, who then follows the content (unless
// they turned that off or unsubscribed from it). Only of public content:
// the others can't see it.
func (n *Notifier) CommentPosted(ctx context.Context, c modulekit.Comment) error {
	node, err := n.nodes.ByID(ctx, c.Nid)
	if err != nil {
		return fmt.Errorf("loading node %d: %w", c.Nid, err)
	}
	author := c.CreatedBy
	settings, err := n.repo.UserSettings(ctx, []int{author})
	if err != nil {
		return err
	}
	if settings[author].AutoCommented {
		if err := n.repo.AutoSubscribe(ctx, author, Target{NodeID: node.ID}); err != nil {
			return err
		}
	}
	if !node.IsPublic() {
		return nil
	}
	told := map[int]bool{author: true}
	var out []Notification
	add := func(userID int, kind string) {
		told[userID] = true
		out = append(out, Notification{UserID: userID, Kind: kind, NodeID: node.ID, CommentID: c.ID, ActorID: author, CreatedAt: n.now()})
	}
	if c.ParentID != nil && *c.ParentID > 0 {
		parent, err := n.comments.ByID(ctx, *c.ParentID)
		if err != nil {
			return fmt.Errorf("loading comment %d: %w", *c.ParentID, err)
		}
		if p := parent.CreatedBy; p > 0 && !told[p] {
			s, err := n.repo.UserSettings(ctx, []int{p})
			if err != nil {
				return err
			}
			if s[p].Replies {
				add(p, KindReply)
			}
		}
	}
	followers, err := n.repo.Followers(ctx, Target{NodeID: node.ID})
	if err != nil {
		return err
	}
	for userID := range followers {
		if !told[userID] {
			add(userID, KindComment)
		}
	}
	return n.store(ctx, out)
}

// NodePublished tells of content node becoming public: its author
// follows its comments (unless they turned that off), and the followers
// of its terms are told of it, each once (published again, it tells
// nobody twice), its author not.
func (n *Notifier) NodePublished(ctx context.Context, node modulekit.Node) error {
	author := node.CreatedBy.ID
	if author > 0 {
		settings, err := n.repo.UserSettings(ctx, []int{author})
		if err != nil {
			return err
		}
		if settings[author].AutoOwn {
			if err := n.repo.AutoSubscribe(ctx, author, Target{NodeID: node.ID}); err != nil {
				return err
			}
		}
	}
	termIDs, err := n.terms.FieldTermIDsForNode(ctx, node.ID)
	if err != nil {
		return fmt.Errorf("the terms of node %d: %w", node.ID, err)
	}
	followers, err := n.repo.TermFollowers(ctx, termIDs)
	if err != nil {
		return err
	}
	delete(followers, author)
	ids := make([]int, 0, len(followers))
	for id := range followers {
		ids = append(ids, id)
	}
	told, err := n.repo.ToldOfContent(ctx, node.ID, ids)
	if err != nil {
		return err
	}
	var out []Notification
	for userID, s := range followers {
		if !told[userID] {
			out = append(out, Notification{UserID: userID, Kind: KindContent, NodeID: node.ID, ActorID: author, TermID: s.TermID, CreatedAt: n.now()})
		}
	}
	return n.store(ctx, out)
}

// store saves ns, to be e-mailed to whoever doesn't turn the e-mails off.
func (n *Notifier) store(ctx context.Context, ns []Notification) error {
	if len(ns) == 0 {
		return nil
	}
	ids := make([]int, 0, len(ns))
	for _, x := range ns {
		ids = append(ids, x.UserID)
	}
	settings, err := n.repo.UserSettings(ctx, ids)
	if err != nil {
		return err
	}
	mailed := make(map[int]bool, len(settings))
	for id, s := range settings {
		mailed[id] = s.Mail != MailNever
	}
	return n.repo.AddNotifications(ctx, ns, mailed)
}
