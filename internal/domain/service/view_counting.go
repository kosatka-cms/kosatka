package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// The reasons CountView doesn't count a view.
var (
	// ErrViewCountingOff - views aren't counted (the Statistics settings).
	ErrViewCountingOff = errors.New("view counting is off")
	// ErrNodeNotPublic - a node not everyone may see: its views aren't
	// counted.
	ErrNodeNotPublic = errors.New("the node is not public")
)

// ViewCountingService toggles client-side view tracking and caches the DB flag in memory.
type ViewCountingService interface {
	IsEnabled() bool
	// CountView counts a view of the node (node_counter): ErrViewCountingOff
	// while counting is off, a not-found error (apperrors.IsNotFound) for
	// a missing node, ErrNodeNotPublic for a node not everyone may see.
	CountView(ctx context.Context, nodeID int) error
	GetSettings(ctx context.Context) (*entity.StatisticsSettings, error)
	UpdateSettings(ctx context.Context, settings entity.StatisticsSettings) error
	RefreshCache(ctx context.Context) error
}

// DefaultViewCountingService implements ViewCountingService.
type DefaultViewCountingService struct {
	repo     entity.StatisticsSettingsRepository
	nodes    entity.NodesRepository
	counters entity.NodeCounterRepository
	enabled  bool
	mu       sync.RWMutex
}

// NewViewCountingService creates a new DefaultViewCountingService.
func NewViewCountingService(repo entity.StatisticsSettingsRepository, nodes entity.NodesRepository, counters entity.NodeCounterRepository) *DefaultViewCountingService {
	return &DefaultViewCountingService{repo: repo, nodes: nodes, counters: counters}
}

// CountView - see ViewCountingService.
func (s *DefaultViewCountingService) CountView(ctx context.Context, nodeID int) error {
	if !s.IsEnabled() {
		return ErrViewCountingOff
	}
	node, err := s.nodes.ByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("counting a view (node %d): %w", nodeID, err)
	}
	if !node.IsPublic() {
		return ErrNodeNotPublic
	}
	if err := s.counters.IncrementView(ctx, nodeID); err != nil {
		return fmt.Errorf("counting a view (node %d): %w", nodeID, err)
	}
	return nil
}

// IsEnabled returns whether view counting is on (thread-safe).
func (s *DefaultViewCountingService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// GetSettings returns settings from the database.
func (s *DefaultViewCountingService) GetSettings(ctx context.Context) (*entity.StatisticsSettings, error) {
	return s.repo.Get(ctx)
}

// UpdateSettings saves settings and refreshes the cache.
func (s *DefaultViewCountingService) UpdateSettings(ctx context.Context, settings entity.StatisticsSettings) error {
	if err := s.repo.Update(ctx, &settings); err != nil {
		return err
	}
	return s.RefreshCache(ctx)
}

// ViewDaysPurgeJob - the scheduler's job deleting the views by day older
// than StatisticsSettings.DailyViewsKeepDays (today is the first of
// them; 0 - kept for ever), hourly.
func (s *DefaultViewCountingService) ViewDaysPurgeJob(now func() time.Time) Job {
	return Job{Name: "view_days_purge", Every: time.Hour, Run: func(ctx context.Context) error {
		cfg, err := s.repo.Get(ctx)
		if err != nil || cfg.DailyViewsKeepDays <= 0 {
			return err
		}
		return s.counters.DeleteViewDaysBefore(ctx, entity.ViewDay(now().AddDate(0, 0, 1-cfg.DailyViewsKeepDays)))
	}}
}

// RefreshCache reloads settings from the database.
func (s *DefaultViewCountingService) RefreshCache(ctx context.Context) error {
	settings, err := s.repo.Get(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.enabled = settings.ViewCountingEnabled
	s.mu.Unlock()
	return nil
}
