//go:build dbintegration

package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestScheduledPublishing - an administrator schedules a node: a
// publishing in the future keeps it unpublished, its editors see when it
// comes and the content list finds it under "On schedule"; an unpublishing
// before the publishing is refused. When its time comes the job publishes
// it (past moderation) and later unpublishes it, each change in its
// history, the schedule cleared.
func TestScheduledPublishing(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil || !strings.Contains(form, `name="publish_on"`) {
			t.Fatal("no body field or schedule on /create/article")
		}
		at := func(d time.Duration) string { return time.Now().Add(d).Format("2006-01-02T15:04") }
		save := func(path string, set url.Values) int {
			t.Helper()
			v := url.Values{"csrf_token": {s.csrf(path)}, "title": {"Announcement"}, body[1]: {"<p>Soon</p>"},
				"status": {"1"}, "on_moderation": {"1"}, "schedule_shown": {"1"}}
			for k, vs := range set {
				v[k] = vs
			}
			resp := s.postMultipart(path, v)
			return resp.StatusCode
		}
		// The unpublishing before the publishing: the form comes back.
		if code := save("/create/article", url.Values{"publish_on": {at(48 * time.Hour)}, "unpublish_on": {at(24 * time.Hour)}}); code != http.StatusOK {
			t.Fatalf("an unpublishing before the publishing: %d, want the form again", code)
		}
		if code := save("/create/article", url.Values{"publish_on": {at(24 * time.Hour)}, "unpublish_on": {at(48 * time.Hour)}}); code != http.StatusFound {
			t.Fatalf("create: %d\n%s", code, s.log.String())
		}
		var id, status int
		var slug string
		if err := db.DB.QueryRow(`SELECT id, status, slug FROM nodes WHERE title = 'Announcement'`).Scan(&id, &status, &slug); err != nil {
			t.Fatal(err)
		}
		if status != 0 {
			t.Errorf("scheduled for tomorrow but published")
		}
		if _, page := s.get("/" + slug); !strings.Contains(page, "node-schedule-notice") {
			t.Error("the editor sees no schedule on the page")
		}
		if _, page := s.get("/admin/content?status=scheduled"); !strings.Contains(page, "Announcement") || !strings.Contains(page, "admin-pill--scheduled") {
			t.Error("the content list's \"On schedule\" doesn't show it")
		}

		// Its time comes.
		job := service.NewScheduledPublishing(database.NewNodeRepository(database.ClientFor(db.DB)), database.NewNodeRepository(database.ClientFor(db.DB)), nil)
		state := func() (status, onModeration int, publishOn, unpublishOn bool) {
			t.Helper()
			var p, u *time.Time
			if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT status, on_moderation, publish_on, unpublish_on FROM nodes WHERE id = ?`), id).Scan(&status, &onModeration, &p, &u); err != nil {
				t.Fatal(err)
			}
			return status, onModeration, p != nil, u != nil
		}
		move := func(column string) {
			t.Helper()
			if _, err := db.DB.Exec(db.Dialect.Rebind(fmt.Sprintf(`UPDATE nodes SET %s = ? WHERE id = ?`, column)), time.Now().Add(-time.Minute), id); err != nil {
				t.Fatal(err)
			}
		}
		move("publish_on")
		if err := job.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if status, onModeration, publishOn, unpublishOn := state(); status != 1 || onModeration != 0 || publishOn || !unpublishOn {
			t.Errorf("after the publishing: status %d, moderation %d, publish_on %v, unpublish_on %v", status, onModeration, publishOn, unpublishOn)
		}
		move("unpublish_on")
		if err := job.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if status, _, publishOn, unpublishOn := state(); status != 0 || publishOn || unpublishOn {
			t.Errorf("after the unpublishing: status %d, publish_on %v, unpublish_on %v", status, publishOn, unpublishOn)
		}
		s.checkLog()
	})
}
