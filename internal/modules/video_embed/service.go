package videoembed

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// cacheMax - links kept in memory; past it the cache starts over.
const cacheMax = 10000

// fetchPerRun - links the background job asks about at a time.
const fetchPerRun = 20

// Service knows the videos linked in texts: what's known in memory (a
// text is shown often), from the table otherwise; a link not known yet
// waits for the background job (FetchPending), the editor's "Video"
// button asks at once (Resolve).
type Service struct {
	repo    *Repo
	fetcher *Fetcher
	logger  *slog.Logger

	mu       sync.Mutex
	known    map[string]Embed
	loaded   map[string]bool // asked the table about (known or not)
	pending  map[string]Match
	settings *Settings
}

// NewService creates a Service.
func NewService(repo *Repo, fetcher *Fetcher, logger *slog.Logger) *Service {
	return &Service{repo: repo, fetcher: fetcher, logger: logger,
		known: map[string]Embed{}, loaded: map[string]bool{}, pending: map[string]Match{}}
}

// Settings returns the module's settings (kept in memory once read).
func (s *Service) Settings(ctx context.Context) Settings {
	s.mu.Lock()
	cached := s.settings
	s.mu.Unlock()
	if cached != nil {
		return *cached
	}
	st, err := s.repo.Settings(ctx)
	if err != nil {
		modulekit.LogFallback(s.logger, err, "video embed settings (the defaults used)")
		return defaultSettings()
	}
	s.mu.Lock()
	s.settings = &st
	s.mu.Unlock()
	return st
}

// SaveSettings saves the settings.
func (s *Service) SaveSettings(ctx context.Context, st Settings) error {
	if err := s.repo.SaveSettings(ctx, st); err != nil {
		return err
	}
	s.mu.Lock()
	s.settings = &st
	s.mu.Unlock()
	return nil
}

// Embeds returns what's known about the matched links; one not known
// (or failed long enough ago) is queued for FetchPending.
func (s *Service) Embeds(ctx context.Context, matches []Match) map[string]Embed {
	out := make(map[string]Embed, len(matches))
	var ask []string
	s.mu.Lock()
	for _, m := range matches {
		if e, ok := s.known[m.URL]; ok {
			out[m.URL] = e
		} else if !s.loaded[m.URL] {
			ask = append(ask, m.URL)
		}
	}
	s.mu.Unlock()
	if len(ask) > 0 {
		found, err := s.repo.Embeds(ctx, ask)
		modulekit.LogFallback(s.logger, err, "loading video embeds (shown without previews)")
		s.mu.Lock()
		if len(s.loaded) > cacheMax {
			s.known, s.loaded = map[string]Embed{}, map[string]bool{}
		}
		for _, l := range ask {
			s.loaded[l] = err == nil
			if e, ok := found[l]; ok {
				s.known[l] = e
				out[l] = e
			}
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	for _, m := range matches {
		e, ok := out[m.URL]
		if !ok || (e.Failed && time.Since(e.FetchedAt) > retryAfter) {
			s.pending[m.URL] = m
		}
	}
	s.mu.Unlock()
	return out
}

// Resolve asks the service about the link now (the editor's button) and
// keeps the answer.
func (s *Service) Resolve(ctx context.Context, m Match) (Embed, error) {
	e := s.fetcher.Fetch(ctx, m, s.Settings(ctx).LocalPreviews)
	if err := s.repo.SaveEmbed(ctx, e); err != nil {
		return e, err
	}
	s.remember(e)
	return e, nil
}

// remember keeps e in memory, the link no longer pending.
func (s *Service) remember(e Embed) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.known[e.URL] = e
	s.loaded[e.URL] = true
	delete(s.pending, e.URL)
}

// FetchPending asks the services about the links waiting (up to
// fetchPerRun at a time) - the module's background job.
func (s *Service) FetchPending(ctx context.Context) error {
	s.mu.Lock()
	batch := make([]Match, 0, fetchPerRun)
	for l, m := range s.pending {
		if len(batch) == fetchPerRun {
			break
		}
		batch = append(batch, m)
		delete(s.pending, l)
	}
	s.mu.Unlock()
	for _, m := range batch {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := s.Resolve(ctx, m); err != nil {
			return err
		}
	}
	return nil
}
