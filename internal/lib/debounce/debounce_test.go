package debounce

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestFuncCoalescesBurst(t *testing.T) {
	var runs atomic.Int32
	d := New(30*time.Millisecond, func() { runs.Add(1) })
	for i := 0; i < 50; i++ {
		d.Trigger()
	}
	time.Sleep(120 * time.Millisecond)
	if got := runs.Load(); got != 1 {
		t.Fatalf("burst of triggers ran fn %d times, want 1", got)
	}
	d.Trigger()
	time.Sleep(120 * time.Millisecond)
	if got := runs.Load(); got != 2 {
		t.Fatalf("a later trigger must run again, got %d runs", got)
	}
}

// TestFuncMaxWait - a steady stream of triggers, closer than the delay,
// still runs fn by the maximum wait.
func TestFuncMaxWait(t *testing.T) {
	var runs atomic.Int32
	d := New(50*time.Millisecond, func() { runs.Add(1) }).WithMaxWait(100 * time.Millisecond)
	stop := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(stop) {
		d.Trigger()
		time.Sleep(10 * time.Millisecond)
	}
	if got := runs.Load(); got < 2 {
		t.Fatalf("400 ms of triggers every 10 ms ran fn %d times, want at least 2", got)
	}
}
