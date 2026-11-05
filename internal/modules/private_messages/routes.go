package privatemessages

import (
	"net/http"
	"strconv"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// base - where the module's pages live: the profile's messages tab
// (/me/messages, served by the core's profile routes) and under it.
const base = "/me/" + manifest.TabPath

// RegisterRoutes registers the module's pages, for signed-in users: a
// thread, the new message form, the actions on threads; and the old
// address of the messages (/user/messages). Sending - a new thread, a
// reply, each mailing its recipients - is rate limited per user (send).
func RegisterRoutes(r modulekit.Mux, store *sessions.CookieStore, c *Controller, send *ratelimit.RateLimiter) {
	auth := modulekit.AuthMiddleware(store)
	perUser := ratelimit.RateLimitMiddleware(send, func(r *http.Request) string {
		id, _ := modulekit.ContextUserID(r.Context())
		return "user:" + strconv.Itoa(id)
	})
	r.Handle("GET "+base+"/thread/{id}", auth(http.HandlerFunc(c.ThreadView)))
	r.Handle("POST "+base+"/new", auth(perUser(http.HandlerFunc(c.ThreadCreate))))
	r.Handle("GET "+base+"/new", auth(http.HandlerFunc(c.ThreadNew)))
	r.Handle("POST "+base+"/thread/{id}/reply", auth(perUser(http.HandlerFunc(c.MessageReply))))
	r.Handle("POST "+base+"/thread/{id}/read", auth(http.HandlerFunc(c.ThreadMarkRead)))
	r.Handle("POST "+base+"/thread/{id}/archive", auth(http.HandlerFunc(c.ThreadArchive)))
	r.Handle("POST "+base+"/archive", auth(http.HandlerFunc(c.ThreadBulkArchive)))
	r.Handle("POST "+base+"/unarchive", auth(http.HandlerFunc(c.ThreadBulkUnarchive)))
	r.Handle("GET /user/messages", auth(http.HandlerFunc(c.Messages)))
}
