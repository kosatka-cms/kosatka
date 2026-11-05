package entity

import (
	"context"
	"maps"
	"sync"
)

// userMemo - the users loaded by ID during one request: a request reads
// its user several times (the session and access middlewares, the
// handler, the page data), the database once.
type userMemo struct {
	mu    sync.Mutex
	users map[int]User
}

type userMemoKey struct{}

// WithUserMemo - ctx keeping the users loaded by ID until it ends (a
// request's: the middleware chain sets it). A write of any user clears
// it (ForgetMemoizedUsers).
func WithUserMemo(ctx context.Context) context.Context {
	return context.WithValue(ctx, userMemoKey{}, &userMemo{users: map[int]User{}})
}

// MemoizedUser - a copy of the user id loaded earlier in ctx's request;
// false - none (or ctx keeps no users).
func MemoizedUser(ctx context.Context, id int) (*User, bool) {
	m, ok := ctx.Value(userMemoKey{}).(*userMemo)
	if !ok {
		return nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, false
	}
	u = cloneUser(u)
	return &u, true
}

// MemoizeUser keeps a copy of u for the rest of ctx's request.
func MemoizeUser(ctx context.Context, u User) {
	if m, ok := ctx.Value(userMemoKey{}).(*userMemo); ok {
		m.mu.Lock()
		m.users[u.ID] = cloneUser(u)
		m.mu.Unlock()
	}
}

// ForgetMemoizedUsers drops the users ctx's request kept: one was
// written.
func ForgetMemoizedUsers(ctx context.Context) {
	if m, ok := ctx.Value(userMemoKey{}).(*userMemo); ok {
		m.mu.Lock()
		clear(m.users)
		m.mu.Unlock()
	}
}

// cloneUser - u with its own metadata and role: what a caller changes in
// the copy it got stays out of the memo.
func cloneUser(u User) User {
	u.Metadata = maps.Clone(u.Metadata)
	if u.Role != nil {
		r := *u.Role
		r.Permissions = maps.Clone(r.Permissions)
		u.Role = &r
	}
	return u
}
