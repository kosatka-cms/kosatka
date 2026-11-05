package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// StatisticsSettingsRepository implements entity.StatisticsSettingsRepository.
type StatisticsSettingsRepository struct {
	client *generated.Client
}

// NewStatisticsSettingsRepository creates a new StatisticsSettingsRepository.
func NewStatisticsSettingsRepository(db *sql.DB) *StatisticsSettingsRepository {
	return &StatisticsSettingsRepository{client: clientFor(db)}
}

// Get returns the statistics settings (single row).
func (r *StatisticsSettingsRepository) Get(ctx context.Context) (*entity.StatisticsSettings, error) {
	s, err := GetClient(ctx, r.client).StatisticsSettings.Get(ctx, settingsRowID)
	if err != nil {
		return nil, fmt.Errorf("StatisticsSettingsRepository.Get: %w", err)
	}
	return &entity.StatisticsSettings{ViewCountingEnabled: s.ViewCountingEnabled, DailyViewsKeepDays: s.DailyViewsKeepDays}, nil
}

// Update saves statistics settings.
func (r *StatisticsSettingsRepository) Update(ctx context.Context, s *entity.StatisticsSettings) error {
	return GetClient(ctx, r.client).StatisticsSettings.UpdateOneID(settingsRowID).
		SetViewCountingEnabled(s.ViewCountingEnabled).SetDailyViewsKeepDays(max(s.DailyViewsKeepDays, 0)).Exec(ctx)
}
