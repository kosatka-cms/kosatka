package service

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/panicsafe"
)

// Job - a task the Scheduler runs in the background every Every (a
// minute at least). Module - the module whose job it is: it runs only
// while the module is enabled; empty - the core's.
type Job struct {
	Name   string
	Module string
	Every  time.Duration
	Run    func(ctx context.Context) error
}

// registeredJobs - the jobs added before the scheduler is built (the
// modules', from their Wire).
var (
	registeredJobsMu sync.Mutex
	registeredJobs   []Job
)

// RegisterJob adds a job every Scheduler started afterwards runs; a job
// of the same name is replaced (the app built again in one process - the
// tests - wires the modules again).
func RegisterJob(j Job) {
	registeredJobsMu.Lock()
	defer registeredJobsMu.Unlock()
	if i := slices.IndexFunc(registeredJobs, func(r Job) bool { return r.Name == j.Name }); i >= 0 {
		registeredJobs[i] = j
		return
	}
	registeredJobs = append(registeredJobs, j)
}

// SchedulerTick - how often the scheduler looks for due jobs.
const SchedulerTick = time.Minute

// jobTimeout - how long one run of a job may take.
const jobTimeout = 50 * time.Second

// Scheduler - the site's background jobs: one loop in the site's own
// process, a tick a minute, each due job run in turn. No cron on the
// server: the site's service runs them while it runs.
type Scheduler struct {
	logger *slog.Logger
	now    func() time.Time

	mu   sync.Mutex
	jobs []Job
	last map[string]time.Time
	tick time.Time
	// done - closed when the loop Start began has returned.
	done chan struct{}
}

// NewScheduler - a scheduler with no jobs yet, logging to logger.
func NewScheduler(logger *slog.Logger) *Scheduler {
	return &Scheduler{logger: logger.With("component", "scheduler"), now: time.Now, last: map[string]time.Time{}, done: make(chan struct{})}
}

// Add adds a job of the core's.
func (s *Scheduler) Add(j Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, j)
}

// Start runs the due jobs now and then every tick, with the registered
// ones (RegisterJob), until ctx ends.
func (s *Scheduler) Start(ctx context.Context) {
	registeredJobsMu.Lock()
	s.mu.Lock()
	s.jobs = append(s.jobs, registeredJobs...)
	s.mu.Unlock()
	registeredJobsMu.Unlock()
	go func() {
		defer close(s.done)
		defer panicsafe.Recover(s.logger, "scheduler")
		ticker := time.NewTicker(SchedulerTick)
		defer ticker.Stop()
		s.Tick(ctx)
		for {
			select {
			case <-ticker.C:
				s.Tick(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Done - closed once the loop has returned (Start's ctx ended), its
// running job finished.
func (s *Scheduler) Done() <-chan struct{} { return s.done }

// Tick runs the jobs that are due: never run, or last run at least their
// Every ago (a few seconds' slack, so a minute's job runs every tick).
func (s *Scheduler) Tick(ctx context.Context) {
	now := s.now()
	s.mu.Lock()
	jobs := append([]Job(nil), s.jobs...)
	s.mu.Unlock()
	for _, j := range jobs {
		if j.Module != "" && !ModuleEnabled(j.Module) {
			continue
		}
		s.mu.Lock()
		last := s.last[j.Name]
		s.mu.Unlock()
		if !last.IsZero() && now.Sub(last) < max(j.Every, SchedulerTick)-5*time.Second {
			continue
		}
		s.run(ctx, j)
		s.mu.Lock()
		s.last[j.Name] = now
		s.mu.Unlock()
	}
	s.mu.Lock()
	s.tick = now
	s.mu.Unlock()
}

// run runs one job; its error or panic is logged here (no caller to
// return it to), the other jobs go on.
func (s *Scheduler) run(ctx context.Context, j Job) {
	defer panicsafe.Recover(s.logger, "job "+j.Name)
	ctx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()
	if err := j.Run(ctx); err != nil {
		s.logger.Error("background job failed", "job", j.Name, "error", err)
	}
}

// LastTick - when the scheduler last looked for due jobs; zero - not yet.
func (s *Scheduler) LastTick() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tick
}
