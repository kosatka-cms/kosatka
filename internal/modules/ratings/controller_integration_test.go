//go:build dbintegration

package ratings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/ratings/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestVoteHandler - a user with the permission votes (a script gets the
// new widget, a form goes back to the page); a guest only while guests
// may, and is then known by their session (an expired form - the site's
// CSRF middleware).
func TestVoteHandler(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		s := NewService(NewRepository(db.DB))
		userID := insertRow(t, db, "INSERT INTO users (username, slug, password_hash) VALUES (?, ?, ?)", "ann", "ann", "")
		nodeID := insertRow(t, db, "INSERT INTO nodes (type, title, slug, status) VALUES (?, ?, ?, ?)", "article", "Sea", "article/sea", 1)
		if err := s.SetTypeRated(ctx, "article", true); err != nil {
			t.Fatal(err)
		}
		store, cookies, token := testkit.Session(t)
		controller := func(uid int, perms map[string]bool) *Controller {
			return NewController(modulekit.BaseDeps{
				Auth:      testkit.Auth{UserID: uid, Permissions: perms},
				Users:     testkit.Users{Known: map[int]*modulekit.User{userID: {ID: userID}}},
				Templates: testkit.Templates(),
				Store:     store,
				Logger:    testkit.Logger(),
			}, s, testkit.Nodes{Known: map[int]*modulekit.Node{nodeID: {ID: nodeID, Type: "article", Slug: "article/sea", Status: 1}}}, nil)
		}
		post := func(c *Controller, form url.Values, script bool) *httptest.ResponseRecorder {
			req := testkit.PostForm("/node/"+strconv.Itoa(nodeID)+"/rating", form, cookies)
			req.SetPathValue("id", strconv.Itoa(nodeID))
			if script {
				req.Header.Set("X-Requested-With", "XMLHttpRequest")
			}
			rec := httptest.NewRecorder()
			c.Vote(rec, req)
			return rec
		}

		rate := map[string]bool{manifest.PermRate: true}
		rec := post(controller(userID, rate), url.Values{"csrf_token": {token}, "score": {"4"}}, true)
		var body struct{ HTML string }
		if err := json.Unmarshal(rec.Body.Bytes(), &body); rec.Code != http.StatusOK || err != nil || !strings.Contains(body.HTML, `aria-pressed="true"`) {
			t.Fatalf("a user's vote: %d %s", rec.Code, rec.Body.String())
		}
		if rec := post(controller(userID, rate), url.Values{"csrf_token": {token}, "score": {"5"}}, false); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/article/sea" {
			t.Errorf("a form's vote: %d %s", rec.Code, rec.Header().Get("Location"))
		}
		if rec := post(controller(userID, nil), url.Values{"csrf_token": {token}, "score": {"5"}}, true); rec.Code != http.StatusForbidden {
			t.Errorf("a user without the permission: %d", rec.Code)
		}

		if rec := post(controller(0, nil), url.Values{"csrf_token": {token}, "score": {"3"}}, true); rec.Code != http.StatusForbidden {
			t.Errorf("a guest while guests may not: %d", rec.Code)
		}
		if err := s.SaveSettings(ctx, Settings{Scale: 5, AllowGuests: true, AllowChange: true}); err != nil {
			t.Fatal(err)
		}
		rec = post(controller(0, nil), url.Values{"csrf_token": {token}, "score": {"3"}}, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("a guest's vote: %d %s", rec.Code, rec.Body.String())
		}
		stats, err := s.repo.Stats(ctx, []int{nodeID})
		if err != nil || stats[nodeID].Count != 2 {
			t.Errorf("the votes: %+v %v", stats, err)
		}
		// The guest's session now names them: their vote shows.
		guestCookies := rec.Result().Cookies()
		req := httptest.NewRequest(http.MethodGet, "/node/"+strconv.Itoa(nodeID)+"/rating", nil)
		for _, c := range guestCookies {
			req.AddCookie(c)
		}
		req.SetPathValue("id", strconv.Itoa(nodeID))
		widget := httptest.NewRecorder()
		controller(0, nil).Widget(widget, req)
		if widget.Code != http.StatusOK || !strings.Contains(widget.Body.String(), `aria-label="3 stars" aria-pressed="true"`) {
			t.Errorf("the guest's widget: %d %s", widget.Code, widget.Body.String())
		}
	})
}
