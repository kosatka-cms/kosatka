package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/themesettings"
)

// ThemeSettingsRepository implements entity.ThemeSettingsRepository.
type ThemeSettingsRepository struct {
	client *generated.Client
}

// NewThemeSettingsRepository creates a new ThemeSettingsRepository.
func NewThemeSettingsRepository(db *sql.DB) *ThemeSettingsRepository {
	return &ThemeSettingsRepository{client: clientFor(db)}
}

// All - every theme's stored values, by theme key.
func (r *ThemeSettingsRepository) All(ctx context.Context) (map[string]map[string]string, error) {
	rows, err := GetClient(ctx, r.client).ThemeSettings.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ThemeSettingsRepository.All: %w", err)
	}
	out := make(map[string]map[string]string, len(rows))
	for _, row := range rows {
		out[row.Theme] = row.Data
	}
	return out, nil
}

// Save replaces theme's values, creating its row if it isn't there.
func (r *ThemeSettingsRepository) Save(ctx context.Context, theme string, values map[string]string) error {
	client := GetClient(ctx, r.client)
	n, err := client.ThemeSettings.Update().Where(themesettings.Theme(theme)).SetData(values).Save(ctx)
	if err != nil || n > 0 {
		return err
	}
	return client.ThemeSettings.Create().SetTheme(theme).SetData(values).Exec(ctx)
}
