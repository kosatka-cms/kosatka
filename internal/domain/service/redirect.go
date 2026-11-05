package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// RedirectService provides redirect lookup with an in-memory cache.
type RedirectService interface {
	GetRedirect(fromPath string) (toPath string, ok bool)
	List(ctx context.Context) ([]entity.Redirect, error)
	// Search - a page of the redirects whose from or to path contains q
	// ("" - all), by from path, and how many there are.
	Search(ctx context.Context, q string, offset, limit int) ([]entity.Redirect, int, error)
	Create(ctx context.Context, from, to string) error
	Delete(ctx context.Context, id int) error
	// MoveSlug keeps a renamed path working: from redirects (301) to to.
	MoveSlug(ctx context.Context, from, to string) error
	// MoveSlugs - MoveSlug for many paths (each move's FromPath to its
	// ToPath), the cache reloaded once.
	MoveSlugs(ctx context.Context, moves []entity.Redirect) error
	RefreshCache(ctx context.Context) error
}

// DefaultRedirectService implements RedirectService.
type DefaultRedirectService struct {
	repo entity.RedirectRepository
	// uow - a move's deletes and insert in one transaction (nil - none).
	uow   entity.UnitOfWork
	cache map[string]string
	mu    sync.RWMutex
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(repo entity.RedirectRepository, uow entity.UnitOfWork) *DefaultRedirectService {
	return &DefaultRedirectService{
		repo:  repo,
		uow:   uow,
		cache: make(map[string]string),
	}
}

// GetRedirect returns the destination path for a source path (thread-safe).
func (s *DefaultRedirectService) GetRedirect(fromPath string) (string, bool) {
	key := strings.Trim(strings.TrimPrefix(fromPath, "/"), " ")
	if key == "" {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	to, ok := s.cache[key]
	return to, ok
}

// List returns all redirects from the database.
func (s *DefaultRedirectService) List(ctx context.Context) ([]entity.Redirect, error) {
	return s.repo.List(ctx)
}

// Search - see RedirectService.
func (s *DefaultRedirectService) Search(ctx context.Context, q string, offset, limit int) ([]entity.Redirect, int, error) {
	return s.repo.Search(ctx, q, offset, limit)
}

// Create adds a redirect and refreshes the cache.
func (s *DefaultRedirectService) Create(ctx context.Context, from, to string) error {
	_, err := s.repo.Create(ctx, entity.Redirect{FromPath: from, ToPath: to})
	if err != nil {
		return err
	}
	return s.RefreshCache(ctx)
}

// Delete removes a redirect and refreshes the cache.
func (s *DefaultRedirectService) Delete(ctx context.Context, id int) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.RefreshCache(ctx)
}

// MoveSlug makes the old path from redirect to to (a node's slug changed).
// Redirects from either path go first: one from to would send the new
// address back to the old (a loop when a slug is changed back), one from
// from would clash with the new redirect.
func (s *DefaultRedirectService) MoveSlug(ctx context.Context, from, to string) error {
	return s.MoveSlugs(ctx, []entity.Redirect{{FromPath: from, ToPath: to}})
}

// MoveSlugs - see RedirectService.MoveSlugs. The moves are one
// transaction (the caller's when ctx carries one): a failure doesn't lose
// an old redirect.
func (s *DefaultRedirectService) MoveSlugs(ctx context.Context, moves []entity.Redirect) error {
	moved := false
	move := func(ctx context.Context) error {
		for _, m := range moves {
			from, to := strings.Trim(m.FromPath, "/ "), strings.Trim(m.ToPath, "/ ")
			if from == "" || to == "" || from == to {
				continue
			}
			for _, p := range []string{to, from} {
				if err := s.repo.DeleteByFromPath(ctx, p); err != nil {
					return fmt.Errorf("deleting the redirect from %q: %w", p, err)
				}
			}
			if _, err := s.repo.Create(ctx, entity.Redirect{FromPath: from, ToPath: to}); err != nil {
				return fmt.Errorf("redirecting %q to %q: %w", from, to, err)
			}
			moved = true
		}
		return nil
	}
	var err error
	if s.uow != nil {
		err = s.uow.Do(ctx, move)
	} else {
		err = move(ctx)
	}
	if err != nil || !moved {
		return err
	}
	return s.RefreshCache(ctx)
}

// RefreshCache reloads all redirects from the database into the cache.
func (s *DefaultRedirectService) RefreshCache(ctx context.Context) error {
	list, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]string, len(list))
	for _, r := range list {
		s.cache[r.FromPath] = r.ToPath
	}
	return nil
}
