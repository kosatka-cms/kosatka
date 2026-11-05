package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// PageCacheMiddleware lets a shared cache (a CDN in front of the site)
// keep guests' pages: a GET or HEAD from a visitor without the session
// cookie answered with a 200 HTML page that sets no cookie and no
// Cache-Control of its own gets "public, max-age=0, s-maxage=<maxAge>"
// (the CDN keeps it maxAge seconds, browsers ask again); every other HTML
// answer gets "private, no-store", so a signed-in visitor's page is never
// shared. maxAge <= 0 - off: answers are left as they are.
func PageCacheMiddleware(maxAge int, sessionCookie string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if maxAge <= 0 {
			return next
		}
		shared := "public, max-age=0, s-maxage=" + strconv.Itoa(maxAge)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := r.Cookie(sessionCookie)
			guest := errors.Is(err, http.ErrNoCookie) && (r.Method == http.MethodGet || r.Method == http.MethodHead)
			pw := &pageCacheWriter{ResponseWriter: w, guest: guest, shared: shared}
			next.ServeHTTP(pw, r)
			pw.finish()
		})
	}
}

// pageCacheWriter sets Cache-Control when the answer's headers go out;
// a status written before the content type is known waits for the first
// bytes (to sniff the type), or for finish.
type pageCacheWriter struct {
	http.ResponseWriter
	guest   bool
	shared  string
	decided bool
	pending int
}

func (w *pageCacheWriter) decide(status int, firstBytes []byte) {
	if w.decided {
		return
	}
	w.decided = true
	h := w.Header()
	if h.Get("Cache-Control") != "" {
		return
	}
	contentType := h.Get("Content-Type")
	if contentType == "" && firstBytes != nil {
		contentType = http.DetectContentType(firstBytes)
	}
	if !strings.HasPrefix(contentType, "text/html") {
		return
	}
	if w.guest && status == http.StatusOK && len(h.Values("Set-Cookie")) == 0 {
		h.Set("Cache-Control", w.shared)
		return
	}
	h.Set("Cache-Control", "private, no-store")
}

func (w *pageCacheWriter) WriteHeader(status int) {
	if w.decided || w.pending != 0 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.Header().Get("Content-Type") == "" {
		w.pending = status
		return
	}
	w.decide(status, nil)
	w.ResponseWriter.WriteHeader(status)
}

func (w *pageCacheWriter) Write(p []byte) (int, error) {
	if !w.decided {
		status := http.StatusOK
		if w.pending != 0 {
			status = w.pending
		}
		w.decide(status, p)
		if w.pending != 0 {
			w.ResponseWriter.WriteHeader(w.pending)
			w.pending = 0
		}
	}
	return w.ResponseWriter.Write(p)
}

// finish sends a status still waiting for bytes that never came.
func (w *pageCacheWriter) finish() {
	if w.pending != 0 {
		w.decided = true
		w.ResponseWriter.WriteHeader(w.pending)
		w.pending = 0
	}
}

// Unwrap - the wrapped writer, for http.ResponseController.
func (w *pageCacheWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Flush passes a flush through (a waiting status goes first).
func (w *pageCacheWriter) Flush() {
	w.finish()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack passes a hijack through.
func (w *pageCacheWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("pagecache: the writer can't be hijacked")
}
