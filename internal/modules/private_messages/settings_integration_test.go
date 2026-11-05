//go:build dbintegration

package privatemessages

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestMailTemplate - never saved, the e-mail is the default; saved, the
// site's (the row made once, then updated); a blank part is the default's.
func TestMailTemplate(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		repo := NewSettingsRepo(db.DB)
		d := defaultMailTemplate()
		if got, err := repo.MailTemplate(ctx); err != nil || got != d {
			t.Fatalf("never saved: %+v %v", got, err)
		}
		for _, saved := range []MailTemplate{{Subject: "From [pm:sender]", Body: "Read [pm:thread-url]"}, {Subject: "Again", Body: "Read"}} {
			if err := repo.SaveMailTemplate(ctx, saved); err != nil {
				t.Fatal(err)
			}
			if got, err := repo.MailTemplate(ctx); err != nil || got != saved {
				t.Errorf("saved %+v: %+v %v", saved, got, err)
			}
		}
		if err := repo.SaveMailTemplate(ctx, MailTemplate{Subject: "Mine", Body: " \n"}); err != nil {
			t.Fatal(err)
		}
		if got, err := repo.MailTemplate(ctx); err != nil || got != (MailTemplate{Subject: "Mine", Body: d.Body}) {
			t.Errorf("a blank body: %+v %v", got, err)
		}
		var rows int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM pm_settings`).Scan(&rows); err != nil || rows != 1 {
			t.Errorf("rows: %d %v", rows, err)
		}
	})
}

// TestMailNewMessage - a user gets the e-mail until they turn it off (one
// row per user, then updated); on again, they get it.
func TestMailNewMessage(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		testUsers(t, db)
		repo := NewSettingsRepo(db.DB)
		if off, err := repo.MailOff(ctx, []int{1, 2}); err != nil || len(off) != 0 {
			t.Fatalf("never saved: %v %v", off, err)
		}
		for _, on := range []bool{false, false, true, false} {
			if err := repo.SetMailNewMessage(ctx, 2, on); err != nil {
				t.Fatal(err)
			}
			if off, err := repo.MailOff(ctx, []int{1, 2}); err != nil || off[2] == on || off[1] {
				t.Errorf("saved %v: %v %v", on, off, err)
			}
		}
		var rows int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM pm_user_settings`).Scan(&rows); err != nil || rows != 1 {
			t.Errorf("rows: %d %v", rows, err)
		}
	})
}
