package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// MaintenanceRepository implements entity.MaintenanceRepository.
type MaintenanceRepository struct {
	client *generated.Client
}

// NewMaintenanceRepository creates a new MaintenanceRepository.
func NewMaintenanceRepository(db *sql.DB) *MaintenanceRepository {
	return &MaintenanceRepository{client: clientFor(db)}
}

// Get returns the maintenance settings (single row).
func (r *MaintenanceRepository) Get(ctx context.Context) (*entity.MaintenanceSettings, error) {
	s, err := GetClient(ctx, r.client).MaintenanceSettings.Get(ctx, settingsRowID)
	if err != nil {
		return nil, fmt.Errorf("MaintenanceRepository.Get: %w", err)
	}
	return &entity.MaintenanceSettings{Enabled: s.Enabled, Message: s.Message}, nil
}

// Update saves the maintenance settings.
func (r *MaintenanceRepository) Update(ctx context.Context, s *entity.MaintenanceSettings) error {
	return GetClient(ctx, r.client).MaintenanceSettings.UpdateOneID(settingsRowID).
		SetEnabled(s.Enabled).SetMessage(s.Message).Exec(ctx)
}
