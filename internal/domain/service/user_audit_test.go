package service

import (
	"context"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestAccountChanges - a profile save's changes for the action log: the
// name, the e-mail and the time zone (empty - the site's) from and to;
// the password, the signature and the picture without values; nothing
// changed - none.
func TestAccountChanges(t *testing.T) {
	ctx := context.Background()
	before := entity.User{Username: "ishmael", Email: "ishmael@example.com", Signature: "<p>Call me</p>", Avatar: entity.File{URI: "public://a.png"}}
	if got := AccountChanges(ctx, before, before, false); len(got) != 0 {
		t.Errorf("nothing changed: %+v", got)
	}
	keep := before
	keep.AvatarUpdateHint = "keep"
	if got := AccountChanges(ctx, before, keep, false); len(got) != 0 {
		t.Errorf("the picture kept: %+v", got)
	}

	after := before
	after.Email, after.Timezone, after.Signature = "queequeg@example.com", "Asia/Tokyo", "<p>Harpooneer</p>"
	after.AvatarUpdateHint = "clear"
	want := []audit.Change{
		{Field: "Email address", From: "ishmael@example.com", To: "queequeg@example.com"},
		{Field: "Time zone", From: "Site default", To: "Asia/Tokyo"},
		{Field: "Password", To: "changed"},
		{Field: "Signature", To: "changed"},
		{Field: "Picture", To: "deleted"},
	}
	got := AccountChanges(ctx, before, after, true)
	if len(got) != len(want) {
		t.Fatalf("changes: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("change %d: %+v, want %+v", i, got[i], want[i])
		}
	}

	renamed := before
	renamed.Username, renamed.Avatar = "starbuck", entity.File{URI: "public://b.png"}
	got = AccountChanges(ctx, before, renamed, false)
	if len(got) != 2 || got[0] != (audit.Change{Field: "Username", From: "ishmael", To: "starbuck"}) || got[1] != (audit.Change{Field: "Picture", To: "uploaded"}) {
		t.Errorf("a new name and picture: %+v", got)
	}
}

// TestUserFieldChanges - the profile fields the form changed, from and
// to, under their labels; a field the form hasn't, or the same value,
// isn't a change; long values cut to 100 characters and "…".
func TestUserFieldChanges(t *testing.T) {
	city := entity.UserFieldDef{ID: 1, Label: "City"}
	ship := entity.UserFieldDef{ID: 2, Label: "Ship"}
	note := entity.UserFieldDef{ID: 3, Label: "About"}
	before := []entity.UserFieldValue{{Field: city, Value: "Nantucket"}, {Field: ship, Value: "Pequod"}, {Field: note, Value: ""}}
	long := strings.Repeat("a", 120)
	got := UserFieldChanges(before, map[int]string{1: "New Bedford", 2: "Pequod", 3: long})
	if len(got) != 2 || got[0] != (audit.Change{Field: "City", From: "Nantucket", To: "New Bedford"}) ||
		got[1].Field != "About" || got[1].From != "" || got[1].To != strings.Repeat("a", 100)+"…" {
		t.Errorf("changes: %+v", got)
	}
	if got := UserFieldChanges(before, map[int]string{}); len(got) != 0 {
		t.Errorf("no form values: %+v", got)
	}
}
