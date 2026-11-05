// Package debounce coalesces bursts of calls into one.
package debounce

import (
	"sync"
	"time"
)

// Func runs fn once, delay after the last Trigger: a burst of triggers
// (e.g. deleting a user's hundreds of nodes) costs one run, not one per
// trigger. With a maximum wait (WithMaxWait) it runs no later than that
// after the burst's first trigger: a steady stream of triggers runs it
// now and then rather than never.
type Func struct {
	mu      sync.Mutex
	timer   *time.Timer
	first   time.Time // the pending burst's first trigger; zero - none
	delay   time.Duration
	maxWait time.Duration
	fn      func()
}

// New creates a Func for fn.
func New(delay time.Duration, fn func()) *Func {
	return &Func{delay: delay, fn: fn}
}

// WithMaxWait - d with the maximum wait after a burst's first trigger.
func (d *Func) WithMaxWait(maxWait time.Duration) *Func {
	d.maxWait = maxWait
	return d
}

// Trigger (re)schedules fn to run delay from now (no later than the
// maximum wait after the burst's first trigger).
func (d *Func) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	now := time.Now()
	if d.first.IsZero() {
		d.first = now
	}
	wait := d.delay
	if d.maxWait > 0 {
		wait = max(min(wait, d.first.Add(d.maxWait).Sub(now)), 0)
	}
	d.timer = time.AfterFunc(wait, d.run)
}

// run ends the burst and runs fn.
func (d *Func) run() {
	d.mu.Lock()
	d.first = time.Time{}
	d.mu.Unlock()
	d.fn()
}
