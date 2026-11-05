package polls

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/polls/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestVoteErrorMessage - a refused vote tells the voter why; any other
// error isn't the voter's (empty: a 500).
func TestVoteErrorMessage(t *testing.T) {
	ctx := context.Background()
	for err, want := range map[error]string{
		ErrPollVoteForbidden:                       "You can't vote in this poll.",
		fmt.Errorf("vote: %w", ErrPollVoteInvalid): "Choose an answer.",
		ErrPollRevoteNotAllowed:                    "You have already voted in this poll.",
		errors.New("database is gone"):             "",
	} {
		if got := voteErrorMessage(ctx, err); got != want {
			t.Errorf("%v: %q, want %q", err, got, want)
		}
	}
}

// fakeVotes - the PollService's Vote: the votes it took, or err.
type fakeVotes struct {
	PollService
	votes [][]int
	err   error
}

func (f *fakeVotes) Vote(_ context.Context, nodeID, userID int, optionIDs []int) error {
	if f.err != nil {
		return f.err
	}
	f.votes = append(f.votes, append([]int{nodeID, userID}, optionIDs...))
	return nil
}

// voteController - the polls controller with the signed-in user (0 -
// none) and the poll node 5.
func voteController(t *testing.T, userID int, votes *fakeVotes) (*PollController, []*http.Cookie, string) {
	t.Helper()
	testkit.UseEngineRoot(t)
	store, cookies, token := testkit.Session(t)
	deps := modulekit.BaseDeps{
		Auth:      testkit.Auth{UserID: userID},
		Users:     testkit.Users{Known: map[int]*modulekit.User{3: {ID: 3}}},
		Templates: testkit.Templates(),
		Store:     store,
		Logger:    testkit.Logger(),
		Nodes:     testkit.Nodes{Known: map[int]*modulekit.Node{5: {ID: 5, Type: manifest.ContentType, Slug: "polls/best"}}},
	}
	return NewPollController(deps, PollDeps{PollService: votes}), cookies, token
}

// TestVoteSubmit - a signed-in user's vote goes to the poll with the
// chosen options and back to its page; a visitor is sent to sign in; a
// refused vote is a 400, another failure a 500 (an expired form - the
// site's CSRF middleware).
func TestVoteSubmit(t *testing.T) {
	votes := &fakeVotes{}
	c, cookies, token := voteController(t, 3, votes)
	vote := func(ctl *PollController, form url.Values) *httptest.ResponseRecorder {
		req := testkit.PostForm("/node/5/poll/vote", form, cookies)
		req.SetPathValue("id", "5")
		rec := httptest.NewRecorder()
		ctl.VoteSubmit(rec, req)
		return rec
	}
	rec := vote(c, url.Values{"csrf_token": {token}, "options": {"7", "9", "x"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/polls/best" {
		t.Errorf("vote: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if len(votes.votes) != 1 || fmt.Sprint(votes.votes[0]) != "[5 3 7 9]" {
		t.Errorf("votes: %v", votes.votes)
	}

	anon, _, _ := voteController(t, 0, votes)
	if rec := vote(anon, url.Values{"csrf_token": {token}, "options": {"7"}}); rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
		t.Errorf("visitor: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	votes.err = ErrPollRevoteNotAllowed
	if rec := vote(c, url.Values{"csrf_token": {token}, "options": {"7"}}); rec.Code != http.StatusBadRequest {
		t.Errorf("refused: %d", rec.Code)
	}
	votes.err = errors.New("db down")
	if rec := vote(c, url.Values{"csrf_token": {token}, "options": {"7"}}); rec.Code != http.StatusInternalServerError {
		t.Errorf("failure: %d", rec.Code)
	}
	if len(votes.votes) != 1 {
		t.Errorf("votes after the refused ones: %v", votes.votes)
	}
}

// fakePoll - the PollService's ByNodeID: the one poll.
type fakePoll struct {
	PollService
	poll Poll
}

func (f fakePoll) ByNodeID(context.Context, int) (*Poll, error) { return &f.poll, nil }

// TestPollEmbedHidden - the "Poll" block's poll of unpublished content or
// content on moderation is not found for a visitor who may not see it
// (AuthService.CanViewNode).
func TestPollEmbedHidden(t *testing.T) {
	c, _, _ := voteController(t, 0, nil)
	for _, n := range []modulekit.Node{
		{ID: 5, Type: manifest.ContentType, Status: 1, OnModeration: 1},
		{ID: 5, Type: manifest.ContentType},
	} {
		c.pollService = fakePoll{poll: Poll{Node: n}}
		req := httptest.NewRequest(http.MethodGet, "/node/5/poll/embed", nil)
		req.SetPathValue("id", "5")
		rec := httptest.NewRecorder()
		c.Embed(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, on moderation %d: %d", n.Status, n.OnModeration, rec.Code)
		}
	}
}

// TestPollNodeSettings - the publishing, the author, the date, the
// address and the comments' mode come from the form for who administers
// content only: an author's new poll is published under them with a
// generated address, an author's edit keeps the poll's own.
func TestPollNodeSettings(t *testing.T) {
	controller := func(admin bool) *PollController {
		return NewPollController(modulekit.BaseDeps{Auth: testkit.Auth{UserID: 3, Permissions: map[string]bool{modulekit.PermAdministerContent: admin}}, Logger: testkit.Logger()}, PollDeps{})
	}
	sent := url.Values{"created_by": {"1"}, "created_at": {"2001-02-03T04:05"}}
	r := httptest.NewRequest(http.MethodPost, "/create/poll", nil)
	r.Form = sent
	user := &modulekit.User{ID: 3}
	pf := &PollForm{Status: 0, OnModeration: 1}
	author := func() *Poll {
		return &Poll{Node: modulekit.Node{Slug: "elsewhere", CommentMode: modulekit.CommentModeHidden, CreatedBy: *user}}
	}

	p := author()
	if admin := controller(false).applyPollNodeSettings(r, user, pf, p, nil); admin || p.Node.Status != 1 || p.Node.OnModeration != 0 ||
		p.Node.Slug != "" || p.Node.CommentMode != "" || p.Node.CreatedBy.ID != 3 || !p.Node.CreatedAt.IsZero() {
		t.Errorf("an author's new poll: %+v", p.Node)
	}
	existing := &Poll{Node: modulekit.Node{Slug: "poll/ours", Status: 1, CommentMode: modulekit.CommentModeOpen}}
	p = author()
	if controller(false).applyPollNodeSettings(r, user, pf, p, existing); p.Node.Slug != "poll/ours" || p.Node.Status != 1 || p.Node.CommentMode != modulekit.CommentModeOpen {
		t.Errorf("an author's edit: %+v", p.Node)
	}
	p = author()
	if admin := controller(true).applyPollNodeSettings(r, user, pf, p, nil); !admin || p.Node.Status != 0 || p.Node.OnModeration != 1 || p.Node.CreatedBy.ID != 1 || p.Node.CreatedAt.Year() != 2001 {
		t.Errorf("an administrator's: %+v", p.Node)
	}
}
