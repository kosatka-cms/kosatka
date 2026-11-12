package dbwatch

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestCheck - one failed ping isn't an outage, two in a row are (logged
// once); an answer ends it (logged with how long it lasted); a stopping
// process isn't the database gone.
func TestCheck(t *testing.T) {
	var logs bytes.Buffer
	var pingErr error
	w := New(func(context.Context) error { return pingErr }, slog.New(slog.NewTextHandler(&logs, nil)))
	clock := time.Date(2026, 11, 13, 17, 20, 0, 0, time.UTC)
	w.now = func() time.Time { return clock }
	ctx := context.Background()

	w.Check(ctx)
	if w.Down() {
		t.Fatal("down while it answers")
	}
	pingErr = errors.New("dial tcp: i/o timeout")
	w.Check(ctx)
	if w.Down() {
		t.Fatal("down after one failed ping")
	}
	clock = clock.Add(Interval)
	w.Check(ctx)
	w.Check(ctx)
	if !w.Down() {
		t.Fatal("not down after failed pings in a row")
	}
	if n := strings.Count(logs.String(), "doesn't answer"); n != 1 {
		t.Errorf("the outage logged %d times: %s", n, logs.String())
	}

	pingErr = nil
	clock = clock.Add(time.Minute)
	w.Check(ctx)
	if w.Down() {
		t.Fatal("still down once it answers")
	}
	if !strings.Contains(logs.String(), "answers again") || !strings.Contains(logs.String(), "down_for=1m5s") {
		t.Errorf("the end of the outage: %s", logs.String())
	}

	// A failure then an answer: no outage.
	pingErr = errors.New("busy")
	w.Check(ctx)
	pingErr = nil
	w.Check(ctx)
	pingErr = errors.New("busy")
	w.Check(ctx)
	if w.Down() {
		t.Error("down after failures not in a row")
	}

	stopped, cancel := context.WithCancel(ctx)
	cancel()
	pingErr = context.Canceled
	w.Check(stopped)
	w.Check(stopped)
	if w.Down() {
		t.Error("down while the process stops")
	}
	if (*Watch)(nil).Down() {
		t.Error("a nil watch is down")
	}
}
