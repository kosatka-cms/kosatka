package interfaces

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// ViewCountController handles POST /api/node/view for node_counter increments.
type ViewCountController struct {
	logger       *slog.Logger
	viewCounting service.ViewCountingService
	// seen - a client's views of a node: the first in viewRepeat counts.
	seen *ratelimit.RateLimiter
}

// viewRepeat - how long a client's next view of the same node isn't
// counted (a reload, a bot sending it again).
const viewRepeat = 30 * time.Minute

// crawlerUA - the user agent of a crawler that names itself: "…bot/2.1",
// "…bot;", "…bot-…", a crawler, a spider, a headless browser, a contact
// address ("+http://…"). Not "bot" inside a word: a phone's model can
// have it ("CUBOT_X30").
var crawlerUA = regexp.MustCompile(`(?i)bot(?:[/;)-]|$)|crawl|spider|slurp|headless|\+http`)

// isCrawler - the request is a crawler's: one that names itself
// (crawlerUA) or doesn't say what it is (a browser always does). Search
// engines' crawlers run the page's script too, and come from too many
// addresses for viewRepeat to hold them back.
func isCrawler(r *http.Request) bool {
	ua := r.UserAgent()
	return strings.TrimSpace(ua) == "" || crawlerUA.MatchString(ua)
}

// NewViewCountController creates a ViewCountController.
func NewViewCountController(logger *slog.Logger, viewCounting service.ViewCountingService) *ViewCountController {
	return &ViewCountController{logger: logger, viewCounting: viewCounting, seen: ratelimit.NewRateLimiterEvery(viewRepeat, 1)}
}

// PostView handles POST /api/node/view (application/x-www-form-urlencoded: nid).
func (c *ViewCountController) PostView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		httputil.JSONError(w, "invalid form", http.StatusBadRequest)
		return
	}
	// No CSRF token: the count is sent from cached guest pages too
	// (middleware.csrfExemptPaths).
	nid, err := strconv.Atoi(r.FormValue("nid"))
	if err != nil || nid <= 0 {
		httputil.JSONError(w, "invalid nid", http.StatusBadRequest)
		return
	}
	// A crawler's view and a repeated one answer as a counted one.
	if isCrawler(r) || !c.seen.Allow(httputil.ClientIP(r)+" "+strconv.Itoa(nid)) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	err = c.viewCounting.CountView(r.Context(), nid)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, service.ErrViewCountingOff):
		httputil.JSONError(w, "disabled", http.StatusForbidden)
	// A missing node (often a bot's made-up ID) is a 404, not a logged
	// error; an unpublished one answers the same, not to tell it exists.
	case apperrors.IsNotFound(err), errors.Is(err, service.ErrNodeNotPublic):
		httputil.JSONError(w, "not found", http.StatusNotFound)
	default:
		logFailure(c.logger, err, "counting a view", "nid", nid)
		httputil.InternalServerError(w)
	}
}
