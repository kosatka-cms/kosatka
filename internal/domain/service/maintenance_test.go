package service

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// memMaintenanceRepo keeps the settings in memory.
type memMaintenanceRepo struct{ settings entity.MaintenanceSettings }

func (r *memMaintenanceRepo) Get(context.Context) (*entity.MaintenanceSettings, error) {
	s := r.settings
	return &s, nil
}

func (r *memMaintenanceRepo) Update(_ context.Context, s *entity.MaintenanceSettings) error {
	r.settings = *s
	return nil
}

// TestMaintenanceMessageSanitized - every visitor sees the message: its
// HTML is sanitized when saved and, for a message saved before, when
// loaded.
func TestMaintenanceMessageSanitized(t *testing.T) {
	ctx := context.Background()
	raw := `<p>Back soon</p><script>alert(1)</script>`
	repo := &memMaintenanceRepo{settings: entity.MaintenanceSettings{Enabled: true, Message: raw}}
	s := NewMaintenanceService(repo)
	if err := s.RefreshCache(ctx); err != nil {
		t.Fatal(err)
	}
	if got := s.GetMessage(); got != `<p>Back soon</p>` {
		t.Errorf("a message saved before: %q", got)
	}
	if err := s.UpdateSettings(ctx, true, raw); err != nil {
		t.Fatal(err)
	}
	if repo.settings.Message != `<p>Back soon</p>` || s.GetMessage() != `<p>Back soon</p>` {
		t.Errorf("saved %q, shown %q", repo.settings.Message, s.GetMessage())
	}
}
