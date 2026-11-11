package views

import (
	"context"
	"encoding/json"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// buildDisplaySnapshot gathers display's current EFFECTIVE Settings
// bundle + Filters + Sorts + Fields into a ViewSnapshot - the shared
// "what would we publish/compare right now" step used by both
// PublishDisplay and HasUnpublishedChanges.
func (s *DefaultService) buildDisplaySnapshot(ctx context.Context, display entity.ViewDisplayDef) (entity.ViewSnapshot, error) {
	effective, err := s.EffectiveSettings(ctx, display)
	if err != nil {
		return entity.ViewSnapshot{}, err
	}
	filters, err := s.EffectiveFiltersByView(ctx, display)
	if err != nil {
		return entity.ViewSnapshot{}, err
	}
	sorts, err := s.EffectiveSortsByView(ctx, display)
	if err != nil {
		return entity.ViewSnapshot{}, err
	}
	fields, err := s.EffectiveFieldsByView(ctx, display)
	if err != nil {
		return entity.ViewSnapshot{}, err
	}
	return entity.ViewSnapshot{
		Settings: entity.SnapshotSettingsOf(effective),
		Filters:  filters.SortBySortOrder(),
		Sorts:    sorts.SortBySortOrder(),
		Fields:   fields.SortBySortOrder(),
	}, nil
}

// PublishDisplay serializes the display's current effective Settings,
// Filters, Sorts and Fields into published_snapshot and sets
// published_at = now, without touching live rows. Idempotent.
func (s *DefaultService) PublishDisplay(ctx context.Context, displayID int) (*entity.ViewDisplayDef, error) {
	display, err := s.editableDisplay(ctx, displayID)
	if err != nil {
		return nil, err
	}
	snap, err := s.buildDisplaySnapshot(ctx, *display)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	return s.repository.SetPublishedSnapshot(ctx, displayID, string(raw), time.Now())
}

// DiscardDisplayChanges replaces the display's own Settings columns
// and Filters/Sorts/Fields rows with the last published snapshot, in one
// transaction. It always sets all OverrideX flags: the snapshot stores
// resolved values, not which flags produced them. Returns
// ErrNoPublishedSnapshot if the display was never published.
func (s *DefaultService) DiscardDisplayChanges(ctx context.Context, displayID int) (*entity.ViewDisplayDef, error) {
	display, err := s.editableDisplay(ctx, displayID)
	if err != nil {
		return nil, err
	}
	snap, err := s.DecodePublishedSnapshot(*display)
	if err != nil {
		return nil, err
	}

	var result *entity.ViewDisplayDef
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		updated := snap.Settings.ApplyTo(*display)
		updated.OverrideSettings = true
		updated.OverrideFilters = true
		updated.OverrideSort = true
		updated.OverrideFields = true
		saved, err := s.repository.UpdateView(ctx, updated)
		if err != nil {
			return err
		}

		if err := deleteRows(ctx, s.filters(), displayID); err != nil {
			return err
		}
		if err := createRows(ctx, s.filters(), snap.Filters, displayID); err != nil {
			return err
		}
		if err := deleteRows(ctx, s.sorts(), displayID); err != nil {
			return err
		}
		if err := createRows(ctx, s.sorts(), snap.Sorts, displayID); err != nil {
			return err
		}
		if err := deleteRows(ctx, s.fields(), displayID); err != nil {
			return err
		}
		if err := createRows(ctx, s.fields(), snap.Fields, displayID); err != nil {
			return err
		}

		result = saved
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// HasUnpublishedChanges reports whether the display's current
// effective state differs from its stored snapshot; always false before
// the first publish.
func (s *DefaultService) HasUnpublishedChanges(ctx context.Context, displayID int) (bool, error) {
	display, err := s.repository.ViewByID(ctx, displayID)
	if err != nil {
		return false, err
	}
	if display.IsDefault || display.PublishedAt == nil {
		return false, nil
	}
	current, err := s.buildDisplaySnapshot(ctx, *display)
	if err != nil {
		return false, err
	}
	stored, err := s.DecodePublishedSnapshot(*display)
	if err != nil {
		return false, err
	}
	currentRaw, err := json.Marshal(stripSnapshotAuditFields(current))
	if err != nil {
		return false, err
	}
	storedRaw, err := json.Marshal(stripSnapshotAuditFields(stored))
	if err != nil {
		return false, err
	}
	return string(currentRaw) != string(storedRaw), nil
}

// stripSnapshotAuditFields zeroes per-row identity/audit columns
// (ID/ViewDisplayID/CreatedAt/UpdatedAt) so HasUnpublishedChanges
// compares by value: DiscardDisplayChanges recreates rows with new IDs
// and timestamps.
func stripSnapshotAuditFields(snap entity.ViewSnapshot) entity.ViewSnapshot {
	filters := make(entity.ViewFilterDefs, len(snap.Filters))
	for i, f := range snap.Filters {
		f.ID, f.ViewDisplayID, f.CreatedAt, f.UpdatedAt = 0, 0, time.Time{}, time.Time{}
		filters[i] = f
	}
	sorts := make(entity.ViewSortDefs, len(snap.Sorts))
	for i, sd := range snap.Sorts {
		sd.ID, sd.ViewDisplayID, sd.CreatedAt, sd.UpdatedAt = 0, 0, time.Time{}, time.Time{}
		sorts[i] = sd
	}
	fields := make(entity.ViewFieldDefs, len(snap.Fields))
	for i, f := range snap.Fields {
		f.ID, f.ViewDisplayID, f.CreatedAt, f.UpdatedAt = 0, 0, time.Time{}, time.Time{}
		fields[i] = f
	}
	snap.Filters, snap.Sorts, snap.Fields = filters, sorts, fields
	return snap
}

// DecodePublishedSnapshot decodes display.PublishedSnapshot - the one
// place the stored blob turns back into an entity.ViewSnapshot
// (PublishedConfig, discarding, comparing). Errors with
// ErrNoPublishedSnapshot if display was never published.
func (s *DefaultService) DecodePublishedSnapshot(display entity.ViewDisplayDef) (entity.ViewSnapshot, error) {
	var snap entity.ViewSnapshot
	if display.PublishedAt == nil || display.PublishedSnapshot == "" {
		return snap, ErrNoPublishedSnapshot
	}
	if err := json.Unmarshal([]byte(display.PublishedSnapshot), &snap); err != nil {
		return snap, err
	}
	return snap, nil
}
