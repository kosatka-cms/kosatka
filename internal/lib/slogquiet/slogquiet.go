// Package slogquiet keeps errors that aren't the server's fault out of the
// ERROR level: a request the client cancelled or a connection it closed
// (context canceled, broken pipe, connection reset) is logged at DEBUG.
package slogquiet

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

// Handler wraps a slog.Handler, lowering client disconnects to DEBUG.
type Handler struct {
	next slog.Handler
}

// New wraps next.
func New(next slog.Handler) *Handler {
	return &Handler{next: next}
}

// Enabled reports whether next handles level.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle passes r to next; an error of a client gone away is passed as
// debug.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError && clientGone(r) {
		lowered := slog.NewRecord(r.Time, slog.LevelDebug, r.Message, r.PC)
		r.Attrs(func(a slog.Attr) bool {
			lowered.AddAttrs(a)
			return true
		})
		if !h.next.Enabled(ctx, slog.LevelDebug) {
			return nil
		}
		return h.next.Handle(ctx, lowered)
	}
	return h.next.Handle(ctx, r)
}

// WithAttrs - the handler over next with attrs.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{next: h.next.WithAttrs(attrs)}
}

// WithGroup - the handler over next with the group name.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{next: h.next.WithGroup(name)}
}

// clientGone - the record's error is the client going away.
func clientGone(r slog.Record) bool {
	gone := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key != "error" {
			return true
		}
		if err, ok := a.Value.Any().(error); ok && errors.Is(err, context.Canceled) {
			gone = true
			return false
		}
		msg := a.Value.String()
		for _, s := range []string{"context canceled", "broken pipe", "connection reset by peer"} {
			if strings.Contains(msg, s) {
				gone = true
				return false
			}
		}
		return true
	})
	return gone
}
