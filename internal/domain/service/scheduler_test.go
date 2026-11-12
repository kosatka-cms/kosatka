package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// TestSchedulerTick - a job runs on the first tick and again once its
// Every has passed; a failing or panicking job doesn't stop the others; a
// disabled module's job doesn't run.
func TestSchedulerTick(t *testing.T) {
	now := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	s := NewScheduler(slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.now = func() time.Time { return now }
	runs := map[string]int{}
	job := func(name string, every time.Duration, err error) Job {
		return Job{Name: name, Every: every, Run: func(context.Context) error {
			runs[name]++
			if name == "panics" {
				panic("boom")
			}
			return err
		}}
	}
	s.Add(job("minute", time.Minute, nil))
	s.Add(job("hourly", time.Hour, nil))
	s.Add(job("fails", time.Minute, errors.New("down")))
	s.Add(job("panics", time.Minute, nil))
	SetModules(map[string]bool{"scheduler-test-module": false})
	t.Cleanup(func() { SetModules(map[string]bool{"scheduler-test-module": true}) })
	off := job("module", time.Minute, nil)
	off.Module = "scheduler-test-module"
	s.Add(off)

	s.Tick(context.Background())
	now = now.Add(time.Minute)
	s.Tick(context.Background())
	want := map[string]int{"minute": 2, "hourly": 1, "fails": 2, "panics": 2}
	for name, n := range want {
		if runs[name] != n {
			t.Errorf("%s ran %d times, want %d", name, runs[name], n)
		}
	}
	if runs["module"] != 0 {
		t.Error("a disabled module's job ran")
	}
	if !s.LastTick().Equal(now) {
		t.Errorf("last tick %v, want %v", s.LastTick(), now)
	}
}

// TestSchedulerPaused - no job runs while the scheduler is paused (the
// database gone); the due ones run on the first tick after.
func TestSchedulerPaused(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	s := NewScheduler(slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.now = func() time.Time { return now }
	runs := 0
	s.Add(Job{Name: "hourly", Every: time.Hour, Run: func(context.Context) error { runs++; return nil }})
	down := true
	s.SetPaused(func() bool { return down })
	s.Tick(context.Background())
	now = now.Add(time.Minute)
	s.Tick(context.Background())
	if runs != 0 {
		t.Fatalf("ran %d times while paused", runs)
	}
	down = false
	now = now.Add(time.Minute)
	s.Tick(context.Background())
	if runs != 1 {
		t.Errorf("ran %d times after the pause, want 1", runs)
	}
	if !s.LastTick().Equal(now) {
		t.Errorf("last tick %v, want %v", s.LastTick(), now)
	}
}

// TestRegisterJobReplaces - a job registered again under its name (the
// app wired again in one process) replaces the earlier one.
func TestRegisterJobReplaces(t *testing.T) {
	registeredJobsMu.Lock()
	saved := registeredJobs
	registeredJobs = nil
	registeredJobsMu.Unlock()
	t.Cleanup(func() {
		registeredJobsMu.Lock()
		registeredJobs = saved
		registeredJobsMu.Unlock()
	})
	var ran []string
	for _, site := range []string{"first", "second"} {
		RegisterJob(Job{Name: "mail", Every: time.Minute, Run: func(context.Context) error { ran = append(ran, site); return nil }})
	}
	RegisterJob(Job{Name: "cleanup", Every: time.Hour, Run: func(context.Context) error { return nil }})
	if len(registeredJobs) != 2 {
		t.Fatalf("%d jobs registered, want mail and cleanup", len(registeredJobs))
	}
	_ = registeredJobs[0].Run(context.Background())
	if len(ran) != 1 || ran[0] != "second" {
		t.Errorf("the mail job registered is %v, want the second", ran)
	}
}
