package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// Date format errors.
var (
	ErrDateFormatLabel       = errors.New("date format: label is required")
	ErrDateFormatMachineName = errors.New("date format: machine name - latin letters, digits and _")
	ErrDateFormatTaken       = errors.New("date format: machine name is taken")
	ErrDateFormatPattern     = errors.New("date format: pattern is required")
	ErrDateFormatDefault     = errors.New("date format: the default format can't be deleted")
	ErrDateFormatInUse       = errors.New("date format: chosen by fields")
)

var dateFormatMachineNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// defaultDateFormatPattern - the pattern of entity.DefaultDateFormat when
// the catalog can't be loaded: the site's plain date.
const defaultDateFormatPattern = "d.m.Y"

// defaultTimeFormatPattern - the time of a date field with times whose
// time format isn't set (or gone): hours and minutes.
const defaultTimeFormatPattern = "H:i"

// defaultDateTimeFormatPattern - the date and time where the catalog has
// no entity.DateTimeFormat.
const defaultDateTimeFormatPattern = "d.m.Y H:i"

// DateFormatService - the date formats catalog: read on every date render
// (cached in memory), edited in /admin/config/date-formats.
type DateFormatService struct {
	repo   entity.DateFormatRepository
	logger *slog.Logger

	mu        sync.RWMutex
	cached    map[string]string // machine name → pattern
	instances map[string]string // entity.InstanceDateFormatKey → machine name
}

// NewDateFormatService - the date formats service on repo (the catalog)
// and logger.
func NewDateFormatService(repo entity.DateFormatRepository, logger *slog.Logger) *DateFormatService {
	return &DateFormatService{repo: repo, logger: logger.With("component", "date_formats.service")}
}

// Pattern - the pattern of the format named name; an empty or unknown
// name gets the default format's.
func (s *DateFormatService) Pattern(ctx context.Context, name string) string {
	patterns := s.patterns(ctx)
	if p, ok := patterns[name]; ok && name != "" {
		return p
	}
	if p, ok := patterns[entity.DefaultDateFormat]; ok {
		return p
	}
	return defaultDateFormatPattern
}

func (s *DateFormatService) patterns(ctx context.Context) map[string]string {
	s.mu.RLock()
	c := s.cached
	s.mu.RUnlock()
	if c != nil {
		return c
	}
	formats, err := s.repo.DateFormats(ctx)
	if err != nil {
		LogFallback(s.logger, err, "error loading date formats")
		return nil
	}
	c = make(map[string]string, len(formats))
	for _, f := range formats {
		c[f.MachineName] = f.Pattern
	}
	s.mu.Lock()
	s.cached = c
	s.mu.Unlock()
	return c
}

// Lookup - the pattern of the format named name, ok=false when there is
// no such format (unlike Pattern, no default).
func (s *DateFormatService) Lookup(ctx context.Context, name string) (string, bool) {
	p, ok := s.patterns(ctx)[name]
	return p, ok && name != ""
}

// Reset drops the cache - after a format or a field's format changes.
func (s *DateFormatService) Reset() {
	s.mu.Lock()
	s.cached, s.instances = nil, nil
	s.mu.Unlock()
}

// InstanceFormat - the format chosen on the date field instance of
// contentType with fieldStorageID ("" - none: the default format).
// For a Views field with no format of its own: the field can be on
// several content types, each with its own format.
func (s *DateFormatService) InstanceFormat(ctx context.Context, contentType string, fieldStorageID int) string {
	s.mu.RLock()
	m := s.instances
	s.mu.RUnlock()
	if m == nil {
		loaded, err := s.repo.InstanceDateFormats(ctx)
		if err != nil {
			LogFallback(s.logger, err, "error loading field date formats")
			return ""
		}
		s.mu.Lock()
		s.instances = loaded
		s.mu.Unlock()
		m = loaded
	}
	return m[entity.InstanceDateFormatKey(contentType, fieldStorageID)]
}

// DateFormats - the catalog, for the admin and the format pickers.
func (s *DateFormatService) DateFormats(ctx context.Context) ([]entity.DateFormat, error) {
	return s.repo.DateFormats(ctx)
}

// DateFormatByID - the date format id, for its edit form.
func (s *DateFormatService) DateFormatByID(ctx context.Context, id int) (*entity.DateFormat, error) {
	return s.repo.DateFormatByID(ctx, id)
}

// Save validates and stores f: creates it when f.ID is 0 (the machine
// name is set only then), else updates its label and pattern.
func (s *DateFormatService) Save(ctx context.Context, f entity.DateFormat) (int, error) {
	f.Label = strings.TrimSpace(f.Label)
	f.Pattern = strings.TrimSpace(f.Pattern)
	if f.Label == "" {
		return 0, ErrDateFormatLabel
	}
	if f.Pattern == "" {
		return 0, ErrDateFormatPattern
	}
	defer s.Reset()
	if f.ID != 0 {
		return f.ID, s.repo.UpdateDateFormat(ctx, f)
	}
	f.MachineName = strings.TrimSpace(f.MachineName)
	if !dateFormatMachineNameRe.MatchString(f.MachineName) {
		return 0, ErrDateFormatMachineName
	}
	existing, err := s.repo.DateFormats(ctx)
	if err != nil {
		return 0, err
	}
	for _, e := range existing {
		if e.MachineName == f.MachineName {
			return 0, ErrDateFormatTaken
		}
	}
	return s.repo.CreateDateFormat(ctx, f)
}

// Delete removes the format with id unless it is the default one or a
// field or Views field chose it (usage lists them).
func (s *DateFormatService) Delete(ctx context.Context, id int) (usage []string, err error) {
	f, err := s.repo.DateFormatByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if f.MachineName == entity.DefaultDateFormat {
		return nil, ErrDateFormatDefault
	}
	usage, err = s.repo.DateFormatUsage(ctx, f.MachineName)
	if err != nil {
		return nil, err
	}
	if len(usage) > 0 {
		return usage, fmt.Errorf("%w: %s", ErrDateFormatInUse, strings.Join(usage, "; "))
	}
	defer s.Reset()
	return nil, s.repo.DeleteDateFormat(ctx, id)
}
