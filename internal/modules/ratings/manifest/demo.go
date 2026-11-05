package manifest

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/ratings/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit/demokit"
)

// demoFS - the module's demo content by language: demo/<lang>.json.
//
//go:embed demo/*.json
var demoFS embed.FS

// demoContent - the content types rated, the demo users' votes and the
// title of the sidebar's top rated list (none - no list: a demo set has
// its own).
type demoContent struct {
	Types []string   `json:"types"`
	Votes []demoVote `json:"votes"`
	Block string     `json:"block"`
}

// demoVote - the votes for one item: a demo set's node (its key), or the
// blog's article (its place, 1 - the newest); scores by demo user, of 5.
type demoVote struct {
	Node    string         `json:"node"`
	Article int            `json:"article"`
	Scores  map[string]int `json:"scores"`
}

// demoScale - the votes' scale: the settings' default.
const demoScale = 5

// createDemo switches ratings on for the content types, adds the demo
// users' votes and, with a Block title, the top rated list in the
// sidebar (demoTopRated).
func createDemo(ctx context.Context, env demokit.Env) error {
	var content demoContent
	if err := demokit.Content(env, demoFS, &content); err != nil {
		return err
	}
	// The module's own client, in the install transaction.
	c := generated.NewClient(generated.Driver(demokit.Driver(env)))
	for _, t := range content.Types {
		if err := c.RatingContentType.Create().SetContentType(t).Exec(ctx); err != nil {
			return fmt.Errorf("rated type %q: %w", t, err)
		}
	}
	for i, v := range content.Votes {
		nid, err := demoVoteNode(env, v)
		if err != nil {
			return fmt.Errorf("votes %d: %w", i+1, err)
		}
		at := env.Now.Add(-time.Duration(i+1) * time.Hour)
		for name, score := range v.Scores {
			uid, err := env.User(name)
			if err != nil {
				return err
			}
			if score < 1 || score > demoScale {
				return fmt.Errorf("votes %d: %s's score %d", i+1, name, score)
			}
			if err := c.RatingVote.Create().SetNid(nid).SetUID(uid).SetScore(score).SetScale(demoScale).
				SetCreatedAt(at).SetUpdatedAt(at).Exec(ctx); err != nil {
				return err
			}
		}
	}
	if content.Block == "" {
		return nil
	}
	return demokit.PlaceView(ctx, env, demoTopRated(content), demokit.Placement{Region: "sidebar", Title: content.Block, ShowTitle: true, Weight: 15})
}

// demoTopRated - the sidebar's top rated list: the rated types' items
// with votes, the best first (the weighted rating: a few votes don't
// outweigh many), each its title and its stars with the average.
func demoTopRated(content demoContent) demokit.ViewBlock {
	return demokit.ViewBlock{
		Name:     "top_rated",
		Label:    content.Block,
		PageSize: 5,
		Fields: []demokit.ViewBlockField{
			{Builtin: "title", LinkTo: "content"},
			{Builtin: ViewFieldRating, Params: map[string]string{ViewParamLook: ViewLookAverage}},
		},
		Filters: []demokit.ViewBlockFilter{
			{Builtin: "content_type", Operator: "in", Value: strings.Join(content.Types, ",")},
			{Builtin: "status", Operator: "eq", Value: "1"},
			{Builtin: "on_moderation", Operator: "eq", Value: "0"},
			{Builtin: ViewFilterCount, Operator: "not_empty"},
		},
		Sorts: []demokit.ViewBlockSort{
			{Builtin: ViewSortWeighted, Direction: "desc", Params: map[string]string{ViewParamMinVotes: "3"}},
		},
	}
}

// demoVoteNode - the node a vote is for: the set's node, or the blog's
// article.
func demoVoteNode(env demokit.Env, v demoVote) (int, error) {
	if v.Node != "" {
		return env.Node(v.Node)
	}
	if v.Article < 1 || v.Article > len(env.Articles) {
		return 0, fmt.Errorf("no article %d", v.Article)
	}
	return env.Articles[v.Article-1], nil
}
