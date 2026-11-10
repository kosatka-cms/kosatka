//go:build dbintegration

package actionlog

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/action_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestRecorder - an action is kept with who did it (their name looked up)
// and from where, what it was done to and what changed; a group turned
// off isn't kept, nor the address unless asked; the filter finds entries
// by who, group, object and days; old entries are purged.
func TestRecorder(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		testkit.WithModules(t, map[string]bool{manifest.Name: true})
		ctx := context.Background()
		if _, err := db.DB.Exec(`INSERT INTO users (id, username, slug, password_hash) VALUES (1, 'ann', 'ann', '')`); err != nil {
			t.Fatal(err)
		}
		repo := NewRepo(db.DB)
		rec := NewRecorder(repo, testkit.Deps(db).Users, testkit.Logger())
		now := time.Now().UTC().Truncate(time.Second)
		actor := modulekit.ActionActor{UserID: 1, IP: "203.0.113.5"}
		rec.Record(ctx, modulekit.ActionEvent{At: now, Actor: actor, Action: modulekit.Action{Type: "node.delete",
			Object: modulekit.ActionObject{Kind: "node", ID: "12", Label: "Old post", Subtype: "article"}}})
		rec.Record(ctx, modulekit.ActionEvent{At: now.Add(-48 * time.Hour), Actor: actor, Action: modulekit.Action{Type: "role.permissions",
			Object:  modulekit.ActionObject{Kind: "role", ID: "editor", Label: "Editor", URL: "/admin/users/permissions"},
			Changes: []modulekit.ActionChange{{Field: "Delete any content", From: "no", To: "yes"}}}})

		all, total, err := repo.List(ctx, Filter{}, 50, 0)
		if err != nil || total != 2 || len(all) != 2 {
			t.Fatalf("list: %+v %d %v", all, total, err)
		}
		if e := all[0]; e.Type != "node.delete" || e.Group != "content" || e.ActorID != 1 || e.ActorName != "ann" || e.IP != "203.0.113.5" || e.Object.Subtype != "article" || !e.At.Equal(now) {
			t.Errorf("newest: %+v", e)
		}
		if e := all[1]; len(e.Changes) != 1 || e.Changes[0].To != "yes" || e.Object.URL != "/admin/users/permissions" {
			t.Errorf("older: %+v", e)
		}
		for name, f := range map[string]Filter{
			"who":     {Actor: "AN"},
			"user":    {ActorID: 1},
			"group":   {Group: "users"},
			"object":  {Object: "old"},
			"its own": {ObjectKind: "node", ObjectID: "12"},
			"days":    {From: now.AddDate(0, 0, -1)},
		} {
			n, err := repo.Count(ctx, f)
			want := 1
			if name == "who" || name == "user" {
				want = 2
			}
			if err != nil || n != want {
				t.Errorf("%s: %d %v, want %d", name, n, err, want)
			}
		}

		if err := rec.SaveSettings(ctx, Settings{KeepDays: 1, GroupsOff: []string{"logins"}}); err != nil {
			t.Fatal(err)
		}
		rec.Record(ctx, modulekit.ActionEvent{At: now, Actor: actor, Action: modulekit.Action{Type: "login"}})
		rec.Record(ctx, modulekit.ActionEvent{At: now, Actor: actor, Action: modulekit.Action{Type: "settings.save", Object: modulekit.ActionObject{Kind: "settings", Label: "Text formats"}}})
		if n, _ := repo.Count(ctx, Filter{Group: "logins"}); n != 0 {
			t.Error("a group turned off was kept")
		}
		if list, _, _ := repo.List(ctx, Filter{Group: "settings"}, 1, 0); len(list) != 1 || list[0].IP != "" {
			t.Errorf("the address not asked for: %+v", list)
		}
		if st, err := repo.Settings(ctx); err != nil || st.KeepDays != 1 || st.LogIP || !st.Off("logins") {
			t.Errorf("settings: %+v %v", st, err)
		}

		if err := rec.Purge(ctx); err != nil {
			t.Fatal(err)
		}
		if n, _ := repo.Count(ctx, Filter{}); n != 2 {
			t.Errorf("after purging older than a day: %d, want 2", n)
		}
	})
}
