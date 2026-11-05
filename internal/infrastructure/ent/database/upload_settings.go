package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// UploadSettingsRepository implements entity.UploadSettingsRepository.
type UploadSettingsRepository struct {
	client *generated.Client
}

// NewUploadSettingsRepository - the upload settings repository on db.
func NewUploadSettingsRepository(db *sql.DB) *UploadSettingsRepository {
	return &UploadSettingsRepository{client: clientFor(db)}
}

// Get - the upload policy from its single row.
func (r *UploadSettingsRepository) Get(ctx context.Context) (*entity.UploadSettings, error) {
	s, err := GetClient(ctx, r.client).UploadSettings.Get(ctx, settingsRowID)
	if err != nil {
		return nil, fmt.Errorf("UploadSettingsRepository.Get: %w", err)
	}
	return &entity.UploadSettings{
		MaxFileSizeBytes:   s.MaxFileSizeBytes,
		MaxImageDimension:  s.MaxImageDimension,
		AllowedMimeTypes:   splitMimeTypes(s.AllowedMimeTypes),
		UserFileExtensions: entity.ParseFileExtensions(s.UserFileExtensions),
	}, nil
}

// Update saves s into the upload policy row.
func (r *UploadSettingsRepository) Update(ctx context.Context, s *entity.UploadSettings) error {
	return GetClient(ctx, r.client).UploadSettings.UpdateOneID(settingsRowID).
		SetMaxFileSizeBytes(s.MaxFileSizeBytes).
		SetMaxImageDimension(s.MaxImageDimension).
		SetAllowedMimeTypes(strings.Join(s.AllowedMimeTypes, ",")).
		SetUserFileExtensions(strings.Join(s.UserFileExtensions, " ")).
		Exec(ctx)
}

func splitMimeTypes(csv string) []string {
	parts := strings.Split(csv, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
