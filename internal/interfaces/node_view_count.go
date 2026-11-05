package interfaces

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
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
	// A repeated view answers as a counted one.
	if !c.seen.Allow(httputil.ClientIP(r) + " " + strconv.Itoa(nid)) {
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
