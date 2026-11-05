package audit

import (
	"context"
	"testing"
)

// capture listens to the recorded actions for the test.
func capture(t *testing.T) *[]Event {
	t.Helper()
	ResetListeners()
	t.Cleanup(ResetListeners)
	var got []Event
	OnAction(func(_ context.Context, e Event) { got = append(got, e) })
	return &got
}

// TestRecord - without a listener nothing happens; a listener gets the
// action with who did it (the context's actor, the user set later
// keeping the address) and when.
func TestRecord(t *testing.T) {
	ResetListeners()
	Record(context.Background(), Action{Type: NodeDelete}) // no listener: nothing
	got := capture(t)
	ctx := WithUser(WithActor(context.Background(), Actor{IP: "203.0.113.5"}), 7)
	Record(ctx, Action{Type: NodeDelete, Object: Object{Kind: KindNode, ID: "3", Label: "Post"}})
	if len(*got) != 1 {
		t.Fatalf("events: %+v", *got)
	}
	e := (*got)[0]
	if e.Actor != (Actor{UserID: 7, IP: "203.0.113.5"}) || e.Object.Label != "Post" || e.At.IsZero() {
		t.Errorf("event: %+v", e)
	}
	if a := ActorFrom(context.Background()); a != (Actor{}) {
		t.Errorf("no actor: %+v", a)
	}
}

// TestCoreTypes - every core action type is in a registered group and
// has a label and a tone.
func TestCoreTypes(t *testing.T) {
	groups := map[string]bool{}
	for _, g := range Groups() {
		groups[g.Name] = true
	}
	for _, name := range []string{NodeCreate, NodeDelete, NodePublish, NodeUnpublish, NodeAuthor, NodeAddress, CommentDelete, CommentApprove, CommentSpam,
		UserCreate, UserBlock, UserUnblock, UserActivate, UserDelete, UserRole, TwoFactorOn, TwoFactorOff, TwoFactorReset,
		RoleCreate, RoleDelete, RoleRename, RolePermissions,
		SettingsSave, ModuleEnable, ModuleDisable, ModuleUninstall, Login, LoginFailed} {
		tp, ok := TypeOf(name)
		if !ok || !groups[tp.Group] || tp.Label == "" || tp.Tone == "" {
			t.Errorf("%s: %+v %v", name, tp, ok)
		}
	}
}
