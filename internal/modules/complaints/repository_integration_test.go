//go:build dbintegration

package complaints

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestSettingsSaved - the settings read back as saved; never saved - the
// defaults (comments take complaints, three hide).
func TestSettingsSaved(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		repo := NewRepo(db.DB)
		if _, err := db.DB.Exec("DELETE FROM complaint_settings"); err != nil {
			t.Fatal(err)
		}
		if s, err := repo.Settings(ctx); err != nil || !s.Comments || s.HideAfter != 3 || len(s.NodeTypes) != 0 {
			t.Errorf("never saved: %+v %v", s, err)
		}
		want := Settings{NodeTypes: []string{"forum", "article"}, Reasons: "Off topic\nSpoilers", HideAfter: 0}
		for range 2 { // the row made, then updated
			if err := repo.SaveSettings(ctx, want); err != nil {
				t.Fatal(err)
			}
		}
		s, err := repo.Settings(ctx)
		if err != nil || s.Comments || s.HideAfter != 0 || s.Reasons != want.Reasons || len(s.NodeTypes) != 2 || s.NodeTypes[0] != "forum" {
			t.Errorf("saved: %+v %v", s, err)
		}
	})
}
