//go:build dbintegration

package notfoundlog

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/not_found_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// fakeRedirects - the site's redirects: from (no leading "/") → to.
type fakeRedirects struct {
	modulekit.RedirectService
	to map[string]string
}

func (f fakeRedirects) GetRedirect(from string) (string, bool) {
	to, ok := f.to[from]
	return to, ok
}

// TestRecorder - a path answered 404 is logged once, its hits counted,
// its last source kept; ignore patterns and a disabled module log
// nothing; the filter finds paths by a part and by status; the old paths
// and the redirected ones are purged; the settings are saved.
func TestRecorder(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		testkit.WithModules(t, map[string]bool{manifest.Name: true})
		ctx := context.Background()
		repo := NewRepo(db.DB)
		redirects := fakeRedirects{to: map[string]string{}}
		rec := NewRecorder(repo, redirects, testkit.Logger())
		now := time.Now().UTC().Truncate(time.Second)
		rec.now = func() time.Time { return now }

		rec.Record(ctx, "/news/old", "https://search.example.org/")
		rec.Record(ctx, "/news/old", "https://example.com/news/")
		rec.Record(ctx, "/wp-login.php", "")
		rec.now = func() time.Time { return now.AddDate(0, 0, -100) }
		rec.Record(ctx, "/ancient", "")
		rec.now = func() time.Time { return now }
		rec.Record(ctx, "/heroes/spider-man", "")

		all, total, err := repo.List(ctx, Filter{Status: StatusAll}, 50, 0)
		if err != nil || total != 3 || len(all) != 3 {
			t.Fatalf("list: %+v %d %v", all, total, err)
		}
		if p := all[0]; p.Path != "/news/old" || p.Hits != 2 || p.Referrer != "https://example.com/news/" || !p.LastSeen.Equal(now) {
			t.Errorf("most asked for: %+v", p)
		}

		spider := all[1]
		if spider.Path != "/heroes/spider-man" && all[2].Path == "/heroes/spider-man" {
			spider = all[2]
		}
		if err := repo.SetIgnored(ctx, spider.ID, true); err != nil {
			t.Fatal(err)
		}
		for name, tc := range map[string]struct {
			f    Filter
			want int
		}{
			"new":     {Filter{}, 2},
			"ignored": {Filter{Status: StatusIgnored}, 1},
			"part":    {Filter{Q: "NEWS", Status: StatusAll}, 1},
		} {
			if _, n, err := repo.List(ctx, tc.f, 50, 0); err != nil || n != tc.want {
				t.Errorf("%s: %d %v, want %d", name, n, err, tc.want)
			}
		}
		if n, err := rec.CountNew(ctx); err != nil || n != 2 {
			t.Errorf("count: %d %v", n, err)
		}

		redirects.to["news/old"] = "/news/new"
		if err := rec.Purge(ctx); err != nil {
			t.Fatal(err)
		}
		if _, n, _ := repo.List(ctx, Filter{Status: StatusAll}, 50, 0); n != 1 {
			t.Errorf("after purge: %d paths, want the ignored one", n)
		}

		st := Settings{KeepDays: 30, Ignore: []string{"/old/*"}}
		if err := rec.SaveSettings(ctx, st); err != nil {
			t.Fatal(err)
		}
		got, err := repo.Settings(ctx)
		if err != nil || got.KeepDays != 30 || got.ShowCount || len(got.Ignore) != 1 || got.Ignore[0] != "/old/*" {
			t.Errorf("settings: %+v %v", got, err)
		}
		if n, _ := rec.CountNew(ctx); n != 0 {
			t.Error("no count while it isn't shown")
		}
		if n, err := repo.Clear(ctx); err != nil || n != 1 {
			t.Errorf("clear: %d %v", n, err)
		}
	})
}

// TestRecorderFull - a full log adds no new path; a known one is still
// counted.
func TestRecorderFull(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		testkit.WithModules(t, map[string]bool{manifest.Name: true})
		saved := maxPaths
		maxPaths = 2
		t.Cleanup(func() { maxPaths = saved })
		ctx := context.Background()
		repo := NewRepo(db.DB)
		rec := NewRecorder(repo, nil, testkit.Logger())
		for _, p := range []string{"/a", "/b", "/c", "/a"} {
			rec.Record(ctx, p, "")
		}
		all, total, err := repo.List(ctx, Filter{Status: StatusAll}, 50, 0)
		if err != nil || total != 2 {
			t.Fatalf("list: %+v %d %v", all, total, err)
		}
		if all[0].Path != "/a" || all[0].Hits != 2 {
			t.Errorf("the known path: %+v", all[0])
		}
	})
}
