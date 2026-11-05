package manifest

import (
	"context"
	"embed"
	"fmt"
	"strconv"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/polls/ent"
	"github.com/kosatka-cms/kosatka/internal/modules/polls/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit/demokit"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

// demoFS - the module's demo content by language: demo/<lang>.json.
//
//go:embed demo/*.json
var demoFS embed.FS

// demoContent - the poll: its title, description (Basic HTML), options,
// the demo users' votes (an option's index by user), comments, the title
// of its link in the footer menu and of its block.
type demoContent struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Options     []string          `json:"options"`
	Votes       map[string]int    `json:"votes"`
	Comments    []demokit.Comment `json:"comments"`
	Menu        string            `json:"menu"`
	Block       string            `json:"block"`
}

// demoPollAge - how long ago the poll opened.
const demoPollAge = 30 * time.Hour

// RegisterSteps registers the module's own ent schema (its tables and
// migration files) and its demo step (manifests.Register).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
	demokit.Register(Name, createDemo)
}

// createDemo adds the poll with the demo users' votes and comments, its
// block in the sidebar of the front page (after the search) and a link to
// it in the footer menu.
func createDemo(ctx context.Context, env demokit.Env) error {
	var content demoContent
	if err := demokit.Content(env, demoFS, &content); err != nil {
		return err
	}
	at := env.Now.Add(-demoPollAge)
	nodeID, err := demokit.CreateNode(ctx, env, demokit.Node{Type: string(ContentType), Title: content.Title, Author: env.CreatedBy, Comments: manifestkit.CommentModeOpen, At: at, Views: 420})
	if err != nil {
		return err
	}
	// The module's own client, in the install transaction.
	client := generated.NewClient(generated.Driver(demokit.Driver(env)))
	poll, err := client.Poll.Create().SetNid(nodeID).SetDescription(content.Description).SetMaxAnswers(1).Save(ctx)
	if err != nil {
		return err
	}
	options := make([]int, len(content.Options))
	for i, label := range content.Options {
		o, err := client.PollOption.Create().SetPollID(poll.ID).SetLabel(label).SetWeight(i).Save(ctx)
		if err != nil {
			return fmt.Errorf("option %q: %w", label, err)
		}
		options[i] = o.ID
	}
	for name, i := range content.Votes {
		user, err := env.User(name)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(options) {
			return fmt.Errorf("vote of %s: no option %d", name, i)
		}
		if err := client.PollVote.Create().SetPollID(poll.ID).SetUserID(user).SetOptionID(options[i]).SetCreatedAt(at.Add(time.Hour)).Exec(ctx); err != nil {
			return err
		}
	}
	if _, _, err := demokit.CreateComments(ctx, env, nodeID, content.Comments, at); err != nil {
		return err
	}
	if err := demokit.PlacePlugin(ctx, env, PluginBlockPoll, map[string]string{manifestkit.PluginBlockTargetKey: strconv.Itoa(nodeID)},
		demokit.Placement{Region: "sidebar", Title: content.Block, Weight: 5, Visibility: manifestkit.BlockVisibilityHomepage}); err != nil {
		return err
	}
	return demokit.MenuLink(ctx, env, env.FooterMenu, content.Menu, "", nodeID, 10)
}
