package polls

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// RegisterPollVoteRoutes registers authenticated POST for casting poll votes
// (CSRF checked in handler), rate limited per client IP, and the poll
// alone for the "Poll" block (any visitor).
func RegisterPollVoteRoutes(r modulekit.Mux, store *sessions.CookieStore, pc *PollController, voteRL *ratelimit.RateLimiter) {
	r.Handle("GET /node/{id}/poll/embed", modulekit.OptionalAuthMiddleware(store)(http.HandlerFunc(pc.Embed)))
	auth := modulekit.AuthMiddleware(store)
	authJSON := modulekit.AuthJSONMiddleware(store)
	rl := ratelimit.RateLimitMiddleware(voteRL, httputil.ClientIP)
	r.Handle("POST /node/{id}/poll/vote", rl(auth(http.HandlerFunc(pc.VoteSubmit))))
	r.Handle("GET /node/{id}/poll/votes", auth(http.HandlerFunc(pc.VotesAudit)))
	r.Handle("POST /api/node/{id}/poll/vote", rl(authJSON(http.HandlerFunc(pc.VoteAPI))))
}
