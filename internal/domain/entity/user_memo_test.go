package entity

import (
	"context"
	"testing"
)

// TestUserMemo - a user kept in a request's ctx comes back as a copy
// (what the caller changes stays out), until a write forgets them; a ctx
// without the memo keeps nothing.
func TestUserMemo(t *testing.T) {
	ctx := WithUserMemo(context.Background())
	MemoizeUser(ctx, User{ID: 7, Username: "ann", Metadata: map[string]any{"k": "v"}, Role: &Role{ID: 2, Permissions: map[string]any{"p": true}}})
	u, ok := MemoizedUser(ctx, 7)
	if !ok || u.Username != "ann" {
		t.Fatalf("memoized: %+v %v", u, ok)
	}
	u.Metadata["k"], u.Role.Permissions["p"], u.Role.ID = "changed", false, 3
	if again, _ := MemoizedUser(ctx, 7); again.Metadata["k"] != "v" || again.Role.Permissions["p"] != true || again.Role.ID != 2 {
		t.Errorf("the copy's changes reached the memo: %+v %+v", again, again.Role)
	}
	if _, ok := MemoizedUser(ctx, 8); ok {
		t.Error("an unknown user")
	}
	ForgetMemoizedUsers(ctx)
	if _, ok := MemoizedUser(ctx, 7); ok {
		t.Error("kept after a write")
	}
	MemoizeUser(context.Background(), User{ID: 7})
	if _, ok := MemoizedUser(context.Background(), 7); ok {
		t.Error("kept without the memo")
	}
}
