// Package dbwatch - whether the site's database answers: a loop pinging
// it, so the requests and the background work learn it is gone at once
// instead of each waiting for a connection that doesn't come.
package dbwatch

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/panicsafe"
)

const (
	// Interval - how often the database is pinged.
	Interval = 5 * time.Second
	// pingTimeout - how long a ping may take.
	pingTimeout = 3 * time.Second
	// failuresDown - the failed pings in a row that make it gone: one slow
	// answer of a busy server isn't an outage.
	failuresDown = 2
)

// Watch - whether the database answers (Down), from a ping every
// Interval (Start). A nil Watch - the database is never down.
type Watch struct {
	ping   func(context.Context) error
	logger *slog.Logger
	now    func() time.Time
	down   atomic.Bool
	done   chan struct{}

	mu sync.Mutex
	// fails - the failed pings in a row; since - when the first of them
	// was.
	fails int
	since time.Time
}

// New - a watch pinging the database with ping, telling logger when it
// goes and comes back.
func New(ping func(context.Context) error, logger *slog.Logger) *Watch {
	return &Watch{ping: ping, logger: logger, now: time.Now, done: make(chan struct{})}
}

// Down - the database doesn't answer: its last pings failed.
func (w *Watch) Down() bool { return w != nil && w.down.Load() }

// Start pings the database every Interval until ctx ends (Done).
func (w *Watch) Start(ctx context.Context) {
	go func() {
		defer close(w.done)
		defer panicsafe.Recover(w.logger, "dbwatch")
		ticker := time.NewTicker(Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.Check(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Done - closed once the loop Start began has returned.
func (w *Watch) Done() <-chan struct{} { return w.done }

// Check pings the database once: failuresDown failures in a row make it
// down, an answer up again. Both changes are logged - here, the loop has
// nobody to return them to.
func (w *Watch) Check(ctx context.Context) {
	pctx, cancel := context.WithTimeout(ctx, pingTimeout)
	err := w.ping(pctx)
	cancel()
	if err != nil && ctx.Err() != nil {
		// The process is stopping, not the database.
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err == nil {
		if w.down.Swap(false) {
			w.logger.Info("the database answers again", "down_for", w.now().Sub(w.since).Round(time.Second).String())
		}
		w.fails = 0
		return
	}
	w.fails++
	if w.fails == 1 {
		w.since = w.now()
	}
	if w.fails == failuresDown {
		w.down.Store(true)
		w.logger.Error("the database doesn't answer: the site answers 503 until it does", "error", err)
	}
}
