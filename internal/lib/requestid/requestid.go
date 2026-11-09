// Package requestid - the request's id: in its context, in the
// X-Request-Id response header and in the log lines written with the
// context (Handler), so a page's answer and its log lines can be matched.
package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
)

// Header - the header carrying the id (a proxy in front may send its own).
const Header = "X-Request-Id"

// Attr - the log field holding the id.
const Attr = "request_id"

type ctxKey struct{}

// New - a fresh id: 16 hex digits.
func New() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Valid - whether s may be taken as an id from a proxy: 1 to 64 letters,
// digits, '-', '_' or '.' (it goes to the logs and a header as is).
func Valid(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !idByte(s[i]) {
			return false
		}
	}
	return true
}

// idByte - whether c may be in an id.
func idByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return c == '-' || c == '_' || c == '.'
}

// WithID - ctx carrying the id.
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext - the id ctx carries, "" without one.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// Handler - a log handler adding request_id to the lines written with a
// request's context (logger.ErrorContext(r.Context(), …)).
type Handler struct {
	slog.Handler
}

// NewHandler wraps h.
func NewHandler(h slog.Handler) *Handler { return &Handler{Handler: h} }

// Handle adds the context's id, if any.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if id := FromContext(ctx); id != "" {
		r.AddAttrs(slog.String(Attr, id))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs keeps the wrapper.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup keeps the wrapper.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{Handler: h.Handler.WithGroup(name)}
}
