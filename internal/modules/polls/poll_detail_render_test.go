package polls

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestPollDetailCommentMode - a poll with hidden comments has no comments
// section; an open one has it.
func TestPollDetailCommentMode(t *testing.T) {
	testkit.UseEngineRoot(t)
	render := func(mode modulekit.CommentMode) string {
		p := Poll{Node: modulekit.Node{ID: 3, Slug: "poll/best", Title: "Best?", Status: 1, CommentMode: mode}}
		return testkit.RenderPageContent(t, "poll/detail.html", map[string]any{"item": &p, "pollView": &PollView{Poll: p}})
	}
	if out := render(modulekit.CommentModeHidden); strings.Contains(out, `id="comments"`) {
		t.Error("hidden comments must not render")
	}
	if out := render(modulekit.CommentModeOpen); !strings.Contains(out, `id="comments"`) {
		t.Error("open comments must render")
	}
}

// TestPollPagesNodeActions - the poll and its vote log take the node's
// actions and status from the engine's parts (partials/node_parts.html);
// the poll's own links come in through node_actions_polls.
func TestPollPagesNodeActions(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.RegisterModules()
	p := Poll{Node: modulekit.Node{ID: 3, Slug: "poll/best", Title: "Best?", Status: 1, OnModeration: 1, CommentMode: modulekit.CommentModeHidden}}
	out := testkit.RenderPageContent(t, "poll/detail.html", map[string]any{"item": &p, "pollView": &PollView{Poll: p}, "canEdit": true, "canDelete": true, "canViewPollVotes": true})
	for _, want := range []string{`class="moderation-notice"`, "node-on-moderation", `href="/node/3/edit"`, `href="/node/3/poll/votes"`, `action="/node/3/unpublish"`, `href="/node/3/delete"`} {
		if !strings.Contains(out, want) {
			t.Errorf("poll: no %q", want)
		}
	}
	// A moderator who may only see the votes still gets the link.
	if out := testkit.RenderPageContent(t, "poll/detail.html", map[string]any{"item": &p, "pollView": &PollView{Poll: p}, "canViewPollVotes": true}); !strings.Contains(out, `href="/node/3/poll/votes"`) || strings.Contains(out, "/edit") {
		t.Errorf("votes-only moderator: %s", out)
	}
	out = testkit.RenderPageContent(t, "poll/votes.html", map[string]any{"item": &p, "poll": &p, "canEdit": true, "canViewPollVotes": true, "pollVotesPage": true})
	if !strings.Contains(out, `href="/poll/best"`) || strings.Contains(out, "/poll/votes") || !strings.Contains(out, `href="/node/3/edit"`) {
		t.Errorf("vote log: %s", out)
	}
}

// TestPollBlockEmbed - the "Poll" block's poll (poll_block): the question
// linking the poll's page, the results and the vote form with the
// visitor's token; after voting, no form but the note.
func TestPollBlockEmbed(t *testing.T) {
	testkit.UseEngineRoot(t)
	p := Poll{Node: modulekit.Node{ID: 3, Slug: "poll/best", Title: "Best?", Status: 1}, MaxAnswers: 1}
	view := &PollView{Poll: p, TotalVotes: 2, CanVote: true, ShowForm: true, OptionCounts: []PollOptionCount{
		{PollOption: PollOption{ID: 7, Label: "Foam"}, Count: 2, Percent: 100},
		{PollOption: PollOption{ID: 8, Label: "Deep"}},
	}}
	render := func(data map[string]any) string {
		t.Helper()
		return testkit.RenderNamed(t, "poll_block", data, "poll/detail.html")
	}
	out := render(map[string]any{"pollView": view, "CSRFToken": "tok"})
	for _, want := range []string{`class="poll-detail poll-block-content"`, `<a href="/poll/best">Best?</a>`, "Foam", "100.0%", `action="/node/3/poll/vote"`, `data-poll-node-id="3"`, `value="tok"`, `value="8"`} {
		if !strings.Contains(out, want) {
			t.Errorf("poll block: no %q in %s", want, out)
		}
	}
	view.ShowForm = false
	out = render(map[string]any{"pollView": view, "currentUser": &modulekit.User{ID: 1}})
	if strings.Contains(out, "poll-vote-form") || !strings.Contains(out, "already voted") {
		t.Errorf("voted: %s", out)
	}
}
