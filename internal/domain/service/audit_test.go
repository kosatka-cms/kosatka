package service

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// captureActions listens to the recorded actions for the test.
func captureActions(t *testing.T) *[]audit.Event {
	t.Helper()
	audit.ResetListeners()
	t.Cleanup(audit.ResetListeners)
	var got []audit.Event
	audit.OnAction(func(_ context.Context, e audit.Event) { got = append(got, e) })
	return &got
}

func eventTypes(events []audit.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}

// TestAuditNodeChange - a save that makes the node public, gives it
// another author and address records those three; one that changes none
// of them records nothing; a node leaving moderation counts as published.
func TestAuditNodeChange(t *testing.T) {
	got := captureActions(t)
	before := entity.Node{ID: 4, Type: "article", Title: "Post", Slug: "article/old", Status: 0, CreatedBy: entity.User{ID: 1, Username: "ann"}}
	after := before
	after.Status, after.Slug, after.CreatedBy = 1, "article/new", entity.User{ID: 2, Username: "bob"}
	AuditNodeChange(context.Background(), before, after)
	types := eventTypes(*got)
	if len(types) != 3 || types[0] != audit.NodePublish || types[1] != audit.NodeAuthor || types[2] != audit.NodeAddress {
		t.Fatalf("events: %v", types)
	}
	if e := (*got)[2]; e.Object.URL != "/node/4" || e.Object.Subtype != "article" || e.Changes[0].From != "/article/old" || e.Changes[0].To != "/article/new" {
		t.Errorf("address event: %+v", e)
	}
	*got = nil
	AuditNodeChange(context.Background(), after, after)
	if len(*got) != 0 {
		t.Errorf("no change: %v", eventTypes(*got))
	}
	moderated := after
	moderated.OnModeration = 1
	AuditNodeChange(context.Background(), moderated, after)
	if types := eventTypes(*got); len(types) != 1 || types[0] != audit.NodePublish {
		t.Errorf("approved: %v", types)
	}
}

// TestAuditUserStatus - blocked, unblocked, activated from pending; the
// same status records nothing.
func TestAuditUserStatus(t *testing.T) {
	got := captureActions(t)
	u := entity.User{ID: 9, Username: "carl"}
	AuditUserStatus(context.Background(), u, entity.UserStatusActive, entity.UserStatusBlocked)
	AuditUserStatus(context.Background(), u, entity.UserStatusBlocked, entity.UserStatusActive)
	AuditUserStatus(context.Background(), u, entity.UserStatusPending, entity.UserStatusActive)
	AuditUserStatus(context.Background(), u, entity.UserStatusActive, entity.UserStatusActive)
	if types := eventTypes(*got); len(types) != 3 || types[0] != audit.UserBlock || types[1] != audit.UserUnblock || types[2] != audit.UserActivate {
		t.Errorf("events: %v", types)
	}
}

// TestSaveGrantsAudit - a role's saved permissions record what changed
// (granted and revoked ones); saving the same set records nothing.
func TestSaveGrantsAudit(t *testing.T) {
	s, roles := newTestPermissions()
	got := captureActions(t)
	ctx := t.Context()
	had := HasGrant(roles.roles[2], PermModerateContent)
	if err := s.SaveGrants(ctx, 2, map[string]bool{PermEditAny("article"): true}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 || (*got)[0].Type != audit.RolePermissions {
		t.Fatalf("events: %v", eventTypes(*got))
	}
	want := 1
	if had {
		want = 2
	}
	if n := len((*got)[0].Changes); n != want {
		t.Errorf("changes: %+v, want %d", (*got)[0].Changes, want)
	}
	*got = nil
	if err := s.SaveGrants(ctx, 2, map[string]bool{PermEditAny("article"): true}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 {
		t.Errorf("the same set: %v", eventTypes(*got))
	}
}
