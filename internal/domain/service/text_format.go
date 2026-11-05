package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// ErrTextFormatDuplicate - the machine_name is taken by another text format.
var ErrTextFormatDuplicate = errors.New("text format machine_name already taken")

// TextFormatService - the text formats: the markup a text field's value
// may keep and the text filters run on it.
type TextFormatService interface {
	TextFormats(ctx context.Context) (entity.TextFormatDefs, error)
	TextFormatByID(ctx context.Context, id int) (*entity.TextFormatDef, error)
	TextFormatByMachineName(ctx context.Context, machineName string) (*entity.TextFormatDef, error)
	CreateTextFormat(ctx context.Context, f entity.TextFormatDef) (*entity.TextFormatDef, error)
	UpdateTextFormat(ctx context.Context, f entity.TextFormatDef) (*entity.TextFormatDef, error)
	DeleteTextFormat(ctx context.Context, id int) error
}

// DefaultTextFormatService - the TextFormatService on the text formats'
// repository; a saved or deleted format updates the text filters run for
// it (SetTextFormatFilters).
type DefaultTextFormatService struct {
	repository entity.TextFormatRepository
	logger     *slog.Logger
}

// NewTextFormatService - the text format service on r (the formats) and
// baseLogger.
func NewTextFormatService(r entity.TextFormatRepository, baseLogger *slog.Logger) TextFormatService {
	return &DefaultTextFormatService{
		repository: r,
		logger:     baseLogger.With("component", "text_format.service"),
	}
}

// TextFormats - every text format.
func (s *DefaultTextFormatService) TextFormats(ctx context.Context) (entity.TextFormatDefs, error) {
	formats, err := s.repository.TextFormats(ctx)
	return entity.Wrapped(formats, err, "listing text formats")
}

// TextFormatByID - the text format id.
func (s *DefaultTextFormatService) TextFormatByID(ctx context.Context, id int) (*entity.TextFormatDef, error) {
	f, err := s.repository.TextFormatByID(ctx, id)
	return entity.Found(f, err, "finding text format by id (id %v)", id)
}

// TextFormatByMachineName - the text format named machineName.
func (s *DefaultTextFormatService) TextFormatByMachineName(ctx context.Context, machineName string) (*entity.TextFormatDef, error) {
	f, err := s.repository.TextFormatByMachineName(ctx, machineName)
	return entity.Found(f, err, "finding text format by machine name (machineName %v)", machineName)
}

// CreateTextFormat saves the new text format f (a taken machine name -
// ErrTextFormatDuplicate) and the filters run for it.
func (s *DefaultTextFormatService) CreateTextFormat(ctx context.Context, f entity.TextFormatDef) (*entity.TextFormatDef, error) {
	created, err := s.repository.CreateTextFormat(ctx, f)
	if err != nil {
		if isDuplicateKeyErr(err) {
			return nil, fmt.Errorf("%w", ErrTextFormatDuplicate)
		}
		return nil, fmt.Errorf("creating text format (machineName %v): %w", f.MachineName, err)
	}
	SetTextFormatFilters(created.ID, created.FilterNames())
	return created, nil
}

// UpdateTextFormat saves text format f and the filters run for it.
func (s *DefaultTextFormatService) UpdateTextFormat(ctx context.Context, f entity.TextFormatDef) (*entity.TextFormatDef, error) {
	updated, err := s.repository.UpdateTextFormat(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("updating text format (id %v): %w", f.ID, err)
	}
	SetTextFormatFilters(updated.ID, updated.FilterNames())
	return updated, nil
}

// DeleteTextFormat deletes text format id; no filters run for it then.
func (s *DefaultTextFormatService) DeleteTextFormat(ctx context.Context, id int) error {
	if err := s.repository.DeleteTextFormat(ctx, id); err != nil {
		return fmt.Errorf("deleting text format (id %v): %w", id, err)
	}
	SetTextFormatFilters(id, nil)
	return nil
}
