package middleware

import (
	"context"
	"net/http"
	"sync"

	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// UserTimezones puts the request's time zone into its context
// (sitetime.WithLocation): the logged-in user's zone when user time zones
// are on, else the site's. User zones are cached; Forget drops one.
type UserTimezones struct {
	lookup func(ctx context.Context, userID int) (string, error)

	mu    sync.RWMutex
	zones map[int]string
}

// NewUserTimezones - lookup gives a user's stored zone ("" - none).
func NewUserTimezones(lookup func(ctx context.Context, userID int) (string, error)) *UserTimezones {
	return &UserTimezones{lookup: lookup, zones: map[int]string{}}
}

// Forget drops the cached zone of userID (their profile changed).
func (u *UserTimezones) Forget(userID int) {
	u.mu.Lock()
	delete(u.zones, userID)
	u.mu.Unlock()
}

func (u *UserTimezones) zone(ctx context.Context, userID int) string {
	u.mu.RLock()
	z, ok := u.zones[userID]
	u.mu.RUnlock()
	if ok {
		return z
	}
	z, err := u.lookup(ctx, userID)
	if err != nil {
		return ""
	}
	u.mu.Lock()
	u.zones[userID] = z
	u.mu.Unlock()
	return z
}

// Middleware must run after OptionalAuthMiddleware (needs the user ID in
// the context).
func (u *UserTimezones) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sitetime.UserZonesEnabled() {
			next.ServeHTTP(w, r)
			return
		}
		userID, _ := r.Context().Value(userIDKey).(int)
		if userID == 0 {
			next.ServeHTTP(w, r)
			return
		}
		loc := sitetime.ForUser(u.zone(r.Context(), userID))
		next.ServeHTTP(w, r.WithContext(sitetime.WithLocation(r.Context(), loc)))
	})
}
