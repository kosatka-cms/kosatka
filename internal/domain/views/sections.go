package views

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// section - one of a display's lists of rows (filters, sort criteria,
// fields: S a list of T) that the display either follows (its View's
// Default's) or overrides (its own copy, overridden). The sections share
// the operations below; each describes how its rows are kept.
type section[S ~[]T, T any] struct {
	list   func(ctx context.Context, displayID int) (S, error)
	create func(ctx context.Context, row T) (*T, error)
	delete func(ctx context.Context, id int) error
	// id - the row's; moved - the row as a new one of display displayID.
	id    func(row T) int
	moved func(row T, displayID int) T
	// overridden - the display's flag of owning its rows of the section.
	overridden func(d *entity.ViewDisplayDef) *bool
}

func (s *DefaultService) filters() section[entity.ViewFilterDefs, entity.ViewFilterDef] {
	return section[entity.ViewFilterDefs, entity.ViewFilterDef]{
		list: s.repository.FiltersByViewID, create: s.repository.CreateFilter, delete: s.repository.DeleteFilter,
		id: func(f entity.ViewFilterDef) int { return f.ID },
		moved: func(f entity.ViewFilterDef, displayID int) entity.ViewFilterDef {
			f.ID, f.ViewDisplayID = 0, displayID
			return f
		},
		overridden: func(d *entity.ViewDisplayDef) *bool { return &d.OverrideFilters },
	}
}

func (s *DefaultService) sorts() section[entity.ViewSortDefs, entity.ViewSortDef] {
	return section[entity.ViewSortDefs, entity.ViewSortDef]{
		list: s.repository.SortsByViewID, create: s.repository.CreateSort, delete: s.repository.DeleteSort,
		id: func(sd entity.ViewSortDef) int { return sd.ID },
		moved: func(sd entity.ViewSortDef, displayID int) entity.ViewSortDef {
			sd.ID, sd.ViewDisplayID = 0, displayID
			return sd
		},
		overridden: func(d *entity.ViewDisplayDef) *bool { return &d.OverrideSort },
	}
}

func (s *DefaultService) fields() section[entity.ViewFieldDefs, entity.ViewFieldDef] {
	return section[entity.ViewFieldDefs, entity.ViewFieldDef]{
		list: s.repository.FieldsByViewID, create: s.repository.CreateViewField, delete: s.repository.DeleteViewField,
		id: func(f entity.ViewFieldDef) int { return f.ID },
		moved: func(f entity.ViewFieldDef, displayID int) entity.ViewFieldDef {
			f.ID, f.ViewDisplayID = 0, displayID
			return f
		},
		overridden: func(d *entity.ViewDisplayDef) *bool { return &d.OverrideFields },
	}
}

// effectiveRows - the rows display shows: its own or its View's
// Default's (effectiveOwnerID).
func effectiveRows[S ~[]T, T any](ctx context.Context, s *DefaultService, sec section[S, T], display entity.ViewDisplayDef) (S, error) {
	ownerID, err := s.effectiveOwnerID(ctx, display, *sec.overridden(&display))
	if err != nil {
		return nil, err
	}
	return sec.list(ctx, ownerID)
}

// createRows creates rows as displayID's (new ones, already valid: not
// validated again).
func createRows[S ~[]T, T any](ctx context.Context, sec section[S, T], rows S, displayID int) error {
	for _, row := range rows {
		if _, err := sec.create(ctx, sec.moved(row, displayID)); err != nil {
			return err
		}
	}
	return nil
}

// copyRows copies the rows of display fromID to display toID.
func copyRows[S ~[]T, T any](ctx context.Context, sec section[S, T], fromID, toID int) error {
	rows, err := sec.list(ctx, fromID)
	if err != nil {
		return err
	}
	return createRows(ctx, sec, rows, toID)
}

// deleteRows deletes display displayID's own rows.
func deleteRows[S ~[]T, T any](ctx context.Context, sec section[S, T], displayID int) error {
	own, err := sec.list(ctx, displayID)
	if err != nil {
		return err
	}
	for _, row := range own {
		if err := sec.delete(ctx, sec.id(row)); err != nil {
			return err
		}
	}
	return nil
}

// overrideRows forks the section of display displayID: its effective
// rows copied as its own (once), the section overridden; its rows.
func overrideRows[S ~[]T, T any](ctx context.Context, s *DefaultService, sec section[S, T], displayID int) (S, error) {
	display, err := s.editableDisplay(ctx, displayID)
	if err != nil {
		return nil, err
	}
	if !*sec.overridden(display) {
		ownerID, err := s.effectiveOwnerID(ctx, *display, false)
		if err != nil {
			return nil, err
		}
		if err := copyRows(ctx, sec, ownerID, displayID); err != nil {
			return nil, err
		}
		*sec.overridden(display) = true
		if _, err := s.repository.UpdateView(ctx, *display); err != nil {
			return nil, err
		}
	}
	return sec.list(ctx, displayID)
}

// revertRows deletes the section's own rows of display displayID, which
// follows its View's Default again.
func revertRows[S ~[]T, T any](ctx context.Context, s *DefaultService, sec section[S, T], displayID int) error {
	display, err := s.editableDisplay(ctx, displayID)
	if err != nil {
		return err
	}
	if err := deleteRows(ctx, sec, displayID); err != nil {
		return err
	}
	*sec.overridden(display) = false
	_, err = s.repository.UpdateView(ctx, *display)
	return err
}

// editableDisplay - the display displayID, not the Default (edited
// directly - ErrCannotEditDefault).
func (s *DefaultService) editableDisplay(ctx context.Context, displayID int) (*entity.ViewDisplayDef, error) {
	display, err := s.repository.ViewByID(ctx, displayID)
	if err != nil {
		return nil, err
	}
	if display.IsDefault {
		return nil, ErrCannotEditDefault
	}
	return display, nil
}

// EffectiveFiltersByView - the filters display applies: its own or its
// View's Default's (effectiveOwnerID).
func (s *DefaultService) EffectiveFiltersByView(ctx context.Context, display entity.ViewDisplayDef) (entity.ViewFilterDefs, error) {
	return effectiveRows(ctx, s, s.filters(), display)
}

// EffectiveSortsByView - the sort criteria display applies: its own or
// its View's Default's (effectiveOwnerID).
func (s *DefaultService) EffectiveSortsByView(ctx context.Context, display entity.ViewDisplayDef) (entity.ViewSortDefs, error) {
	return effectiveRows(ctx, s, s.sorts(), display)
}

// EffectiveFieldsByView - the fields display shows: its own or its
// View's Default's (effectiveOwnerID).
func (s *DefaultService) EffectiveFieldsByView(ctx context.Context, display entity.ViewDisplayDef) (entity.ViewFieldDefs, error) {
	return effectiveRows(ctx, s, s.fields(), display)
}

// OverrideFilters forks the display's filters: its effective filters
// copied as its own, OverrideFilters set (overrideRows).
func (s *DefaultService) OverrideFilters(ctx context.Context, displayID int) (entity.ViewFilterDefs, error) {
	return overrideRows(ctx, s, s.filters(), displayID)
}

// RevertFilters drops the display's own filters: it follows its View's
// Default again (revertRows).
func (s *DefaultService) RevertFilters(ctx context.Context, displayID int) error {
	return revertRows(ctx, s, s.filters(), displayID)
}

// OverrideSort forks the display's sort criteria: its effective ones
// copied as its own, OverrideSort set (overrideRows).
func (s *DefaultService) OverrideSort(ctx context.Context, displayID int) (entity.ViewSortDefs, error) {
	return overrideRows(ctx, s, s.sorts(), displayID)
}

// RevertSort drops the display's own sort criteria: it follows its
// View's Default again (revertRows).
func (s *DefaultService) RevertSort(ctx context.Context, displayID int) error {
	return revertRows(ctx, s, s.sorts(), displayID)
}

// OverrideFields forks the display's fields: its effective fields
// copied as its own, OverrideFields set (overrideRows).
func (s *DefaultService) OverrideFields(ctx context.Context, displayID int) (entity.ViewFieldDefs, error) {
	return overrideRows(ctx, s, s.fields(), displayID)
}

// RevertFields drops the display's own fields: it follows its View's
// Default again (revertRows).
func (s *DefaultService) RevertFields(ctx context.Context, displayID int) error {
	return revertRows(ctx, s, s.fields(), displayID)
}
