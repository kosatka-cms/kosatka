package entity

import "context"

// MaintenanceSettings represents maintenance mode configuration
type MaintenanceSettings struct {
	Enabled bool
	Message string
}

// MaintenanceRepository defines the interface for maintenance settings data access
type MaintenanceRepository interface {
	Get(ctx context.Context) (*MaintenanceSettings, error)
	Update(ctx context.Context, s *MaintenanceSettings) error
}
