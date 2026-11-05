package service

import (
	"context"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// BlockedBotService provides bot blocking with in-memory cache.
type BlockedBotService interface {
	IsBlocked(userAgent string) bool
	List(ctx context.Context) ([]entity.BlockedBot, error)
	Create(ctx context.Context, pattern, comment string) error
	Delete(ctx context.Context, id int) error
	RefreshCache(ctx context.Context) error
}

// DefaultBlockedBotService implements BlockedBotService.
type DefaultBlockedBotService struct {
	repo  entity.BlockedBotRepository
	cache []string
	mu    sync.RWMutex
}

// NewBlockedBotService creates a new BlockedBotService.
func NewBlockedBotService(repo entity.BlockedBotRepository) *DefaultBlockedBotService {
	return &DefaultBlockedBotService{
		repo:  repo,
		cache: nil,
	}
}

// IsBlocked returns true if the User-Agent matches any blocked pattern (case-insensitive substring).
func (s *DefaultBlockedBotService) IsBlocked(userAgent string) bool {
	s.mu.RLock()
	patterns := s.cache
	s.mu.RUnlock()

	if len(patterns) == 0 {
		return false
	}

	ua := strings.ToLower(userAgent)
	for _, p := range patterns {
		if strings.Contains(ua, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// List returns all blocked bot patterns from the database.
func (s *DefaultBlockedBotService) List(ctx context.Context) ([]entity.BlockedBot, error) {
	return s.repo.List(ctx)
}

// Create adds a new pattern and refreshes the cache.
func (s *DefaultBlockedBotService) Create(ctx context.Context, pattern, comment string) error {
	_, err := s.repo.Create(ctx, pattern, comment)
	if err != nil {
		return err
	}
	return s.RefreshCache(ctx)
}

// Delete removes a pattern and refreshes the cache.
func (s *DefaultBlockedBotService) Delete(ctx context.Context, id int) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.RefreshCache(ctx)
}

// RefreshCache reloads all patterns from the database into the cache.
func (s *DefaultBlockedBotService) RefreshCache(ctx context.Context) error {
	list, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	patterns := make([]string, 0, len(list))
	for _, b := range list {
		patterns = append(patterns, b.Pattern)
	}
	s.mu.Lock()
	s.cache = patterns
	s.mu.Unlock()
	return nil
}
