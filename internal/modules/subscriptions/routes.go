package subscriptions

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// RegisterRoutes registers the module's pages: following content and
// terms, the tab's actions, a notification's link (a guest signs in
// first), the e-mails' unsubscribe link (no sign-in).
func RegisterRoutes(r modulekit.Mux, store *sessions.CookieStore, c *Controller) {
	auth := modulekit.AuthMiddleware(store)
	optional := modulekit.OptionalAuthMiddleware(store)
	r.Handle("POST /subscriptions/follow/{kind}/{id}", auth(http.HandlerFunc(c.Follow)))
	r.Handle("POST /subscriptions/delete/{id}", auth(http.HandlerFunc(c.Unfollow)))
	r.Handle("GET /subscriptions/unsubscribe/{token}", http.HandlerFunc(c.Unsubscribe))
	r.Handle("POST /subscriptions/unsubscribe/{token}", http.HandlerFunc(c.Unsubscribe))
	r.Handle("GET /notifications/{id}", optional(http.HandlerFunc(c.Open)))
	r.Handle("POST /notifications/read", auth(http.HandlerFunc(c.ReadAll)))
}
