package service

import (
	"context"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
)

// MaintenanceService provides maintenance mode with an in-memory cache.
type MaintenanceService interface {
	IsEnabled() bool
	GetMessage() string
	GetSettings(ctx context.Context) (*entity.MaintenanceSettings, error)
	UpdateSettings(ctx context.Context, enabled bool, message string) error
	RefreshCache(ctx context.Context) error
}

// DefaultMaintenanceService implements MaintenanceService.
type DefaultMaintenanceService struct {
	repo    entity.MaintenanceRepository
	enabled bool
	message string
	mu      sync.RWMutex
}

// NewMaintenanceService creates a new MaintenanceService.
func NewMaintenanceService(repo entity.MaintenanceRepository) *DefaultMaintenanceService {
	return &DefaultMaintenanceService{repo: repo}
}

// IsEnabled reports whether maintenance mode is on (thread-safe).
func (s *DefaultMaintenanceService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// GetMessage returns the maintenance message, its HTML sanitized: every
// visitor sees it (thread-safe).
func (s *DefaultMaintenanceService) GetMessage() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.message
}

// GetSettings returns the current settings from the database.
func (s *DefaultMaintenanceService) GetSettings(ctx context.Context) (*entity.MaintenanceSettings, error) {
	return s.repo.Get(ctx)
}

// UpdateSettings saves settings, the message's HTML sanitized, and
// refreshes the cache.
func (s *DefaultMaintenanceService) UpdateSettings(ctx context.Context, enabled bool, message string) error {
	if err := s.repo.Update(ctx, &entity.MaintenanceSettings{Enabled: enabled, Message: htmlsanitize.SanitizeAdmin(message)}); err != nil {
		return err
	}
	return s.RefreshCache(ctx)
}

// RefreshCache reloads settings from the database.
func (s *DefaultMaintenanceService) RefreshCache(ctx context.Context) error {
	settings, err := s.repo.Get(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.enabled = settings.Enabled
	// Sanitized here too: a message saved before UpdateSettings sanitized.
	s.message = htmlsanitize.SanitizeAdmin(settings.Message)
	s.mu.Unlock()
	return nil
}
