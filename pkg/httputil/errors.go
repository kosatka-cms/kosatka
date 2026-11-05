package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"net"
	"net/http"
	"strings"
	"syscall"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// InternalServerError sends HTTP 500. If cause is set and indicates a failure while
// writing the response body to the client (timeout, broken pipe, reset), the status
// line may already be sent; in that case it does nothing to avoid a superfluous
// WriteHeader (e.g. after template streaming stops mid-body).
func InternalServerError(w http.ResponseWriter, cause ...error) {
	if len(cause) > 0 && cause[0] != nil && IsLikelyResponseWriteFailure(cause[0]) {
		return
	}
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

// TemplateExecutionError sends a minimal HTML 500 page with a message for
// the visitor; err itself is for the caller's log (its text names files
// and templates). Does nothing if err looks like a client disconnect /
// write failure.
func TemplateExecutionError(w http.ResponseWriter, err error) {
	if err == nil || IsLikelyResponseWriteFailure(err) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	// No request here: the site's default language.
	ctx := context.Background()
	_, _ = w.Write([]byte(`<!DOCTYPE html><html lang="` + html.EscapeString(i18n.Default()) + `"><head><meta charset="utf-8"><title>` +
		html.EscapeString(i18n.T(ctx, "Page rendering error")) + `</title></head><body>` +
		`<h1>` + html.EscapeString(i18n.T(ctx, "Something went wrong")) + `</h1>` +
		`<p>` + html.EscapeString(i18n.T(ctx, "Try refreshing the page. We will fix it soon.")) + `</p>` +
		`</body></html>`))
}

// IsLikelyResponseWriteFailure reports whether err is probably a failure writing
// the response to the client (timeout, reset, broken pipe), as opposed to a
// template or application error before any bytes were sent.
func IsLikelyResponseWriteFailure(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "write" {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "connection reset by peer") ||
		strings.Contains(s, "use of closed network connection")
}

// JSON writes a JSON response
func JSON(w http.ResponseWriter, data any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

// SendJSON is an alias for JSON with flipped parameter order for convenience
func SendJSON(w http.ResponseWriter, statusCode int, data any) {
	JSON(w, data, statusCode)
}

// JSONError writes a JSON error response
func JSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// DecodeJSON decodes JSON request body
func DecodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
