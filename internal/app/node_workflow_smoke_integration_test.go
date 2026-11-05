//go:build dbintegration

package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestNodeWorkflowOwnsPublishing - a module's editorial workflow
// (modulekit.SetNodeWorkflow) publishes its types' items, not the core:
// their form has no publishing controls, a save its Validate refuses
// isn't made, one it allows leaves the item unpublished whatever the
// request says and runs its Apply; the bulk actions leave the items out,
// their publish route refuses, a schedule set before is cleared, not
// fulfilled. A stand-in workflow on articles, of no module of the site.
func TestNodeWorkflowOwnsPublishing(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, true)
		s.signIn()
		var scheduled int
		if err := db.DB.QueryRow(`SELECT MAX(id) FROM nodes WHERE type = 'article' AND status = 1`).Scan(&scheduled); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.Exec(db.Dialect.Rebind(`UPDATE nodes SET status = 0, publish_on = ? WHERE id = ?`), time.Now().Add(-time.Minute).UTC(), scheduled); err != nil {
			t.Fatal(err)
		}
		type apply struct {
			nid   int
			isNew bool
		}
		var applied []apply
		interfaces.SetNodeWorkflow(interfaces.NodeWorkflow{
			Module:  "stand_in_workflow",
			Handles: func(_ context.Context, contentType string) bool { return contentType == "article" },
			Load:    func(context.Context, string, *entity.Node, *entity.User) (any, error) { return "draft", nil },
			Validate: func(_ context.Context, _ string, _ *entity.Node, _ *entity.User, r *http.Request) string {
				if r.FormValue("transition") == "" {
					return "Choose a transition."
				}
				return ""
			},
			Apply: func(_ context.Context, n entity.Node, _ *entity.User, _ *http.Request, isNew bool) error {
				applied = append(applied, apply{n.ID, isNew})
				return nil
			},
		})
		t.Cleanup(interfaces.ResetModuleHooks)

		// The form: no publishing checkbox; a save Validate refuses isn't
		// made, the allowed one saves the item unpublished.
		_, create := s.get("/create/article")
		if strings.Contains(create, `name="status"`) {
			t.Error("the form of a workflow's type has the publishing checkbox")
		}
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(create)
		if body == nil {
			t.Fatal("the create form has no body field")
		}
		form := url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {"Harbour festival"}, body[1]: {"<p>Three days.</p>"}, "status": {"1"}}
		if resp, page := s.postUploadsPage("/create/article", form, nil); resp.StatusCode == http.StatusFound || !strings.Contains(page, "Choose a transition.") || len(applied) != 0 {
			t.Fatalf("saved without a transition: %d, applied %v", resp.StatusCode, applied)
		}
		form.Set("transition", "draft")
		if resp := s.postMultipart("/create/article", form); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d\n%s", resp.StatusCode, s.log.String())
		}
		var nid, status int
		if err := db.DB.QueryRow(`SELECT id, status FROM nodes WHERE title = 'Harbour festival'`).Scan(&nid, &status); err != nil || status != 0 {
			t.Fatalf("the new article: status %d, %v", status, err)
		}
		if len(applied) != 1 || applied[0] != (apply{nid, true}) {
			t.Errorf("Apply: %v, want the new article %d", applied, nid)
		}

		// The schedule from before: cleared, not fulfilled.
		nodes := database.NewNodeRepository(database.ClientFor(db.DB))
		if err := service.NewScheduledPublishing(nodes, nodes, nil).Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		publishing := func() (status int, publishOn bool) {
			t.Helper()
			var p *time.Time
			if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT status, publish_on FROM nodes WHERE id = ?`), scheduled).Scan(&status, &p); err != nil {
				t.Fatal(err)
			}
			return status, p != nil
		}
		if status, publishOn := publishing(); status != 0 || publishOn {
			t.Errorf("scheduled: status %d, publish_on %v", status, publishOn)
		}

		// The bulk actions leave it out, its publish route refuses.
		for _, action := range []string{"status_publish", "moderation_on"} {
			req, err := http.NewRequest(http.MethodPost, s.base+"/api/admin/bulk/content", strings.NewReader(fmt.Sprintf(`{"action":%q,"ids":[%d]}`, action, scheduled)))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Token", s.csrf("/admin/content"))
			resp, err := s.client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if !strings.Contains(string(b), `"processed":0`) || !strings.Contains(string(b), "transitions") {
				t.Errorf("bulk %s: %s", action, b)
			}
		}
		if code, _, _ := s.postRead(fmt.Sprintf("/node/%d/publish", scheduled), url.Values{"csrf_token": {s.csrf("/admin/content")}}); code != http.StatusForbidden {
			t.Errorf("the publish route: %d", code)
		}
		if status, _ := publishing(); status != 0 {
			t.Errorf("after the refused actions: status %d", status)
		}
		s.checkLog()
	})
}
