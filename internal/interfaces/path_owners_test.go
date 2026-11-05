package interfaces

import (
	"context"
	"errors"
	"testing"
)

// TestPathOwner - the first owner answering a path, the asking one
// skipped; one registered again under its name is replaced; a failed
// check is an error, not a free address.
func TestPathOwner(t *testing.T) {
	t.Cleanup(resetPathOwners)
	resetPathOwners()
	answers := func(paths ...string) func(context.Context, string) (bool, error) {
		return func(_ context.Context, p string) (bool, error) {
			for _, x := range paths {
				if x == p {
					return true, nil
				}
			}
			return false, nil
		}
	}
	AddPathOwner(PathOwnerRoutes, answers("rss.xml"))
	AddPathOwner(PathOwnerViews, answers("news"))
	AddPathOwner("feeds", answers("old.xml"))
	AddPathOwner("feeds", answers("feeds/news.xml"))
	ctx := context.Background()
	for _, tc := range []struct{ path, except, want string }{
		{"/rss.xml", "feeds", PathOwnerRoutes},
		{"news", "feeds", PathOwnerViews},
		{"news", PathOwnerViews, ""},
		{"feeds/news.xml", PathOwnerViews, "feeds"},
		{"old.xml", PathOwnerViews, ""},
		{"", "feeds", ""},
	} {
		if got, err := PathOwner(ctx, tc.path, tc.except); err != nil || got != tc.want {
			t.Errorf("PathOwner(%q, except %q) = %q %v, want %q", tc.path, tc.except, got, err, tc.want)
		}
	}
	AddPathOwner(PathOwnerTerms, func(context.Context, string) (bool, error) { return false, errors.New("db down") })
	if _, err := PathOwner(ctx, "free", "feeds"); err == nil {
		t.Error("a failed check: no error")
	}
}
