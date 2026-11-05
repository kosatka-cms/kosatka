package themes

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"maps"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/fallback"
)

// SettingsService - the themes' settings (entity.ThemeSetting, from
// their theme.yaml) and their values: the defaults, then what was saved
// on the Appearance page. Read on every page (cached in memory, the
// manifests once a theme), saved from /admin/config/appearance/theme/<key>.
type SettingsService struct {
	repo   entity.ThemeSettingsRepository
	logger *slog.Logger

	mu sync.RWMutex
	// values - theme key → stored values; nil - not loaded. Never changed
	// in place (store replaces it), so a reader may use the map it took
	// under the lock after releasing it.
	values   map[string]map[string]string
	settings map[string][]entity.ThemeSetting
}

// NewSettingsService - the themes' settings with their saved values in
// repo.
func NewSettingsService(repo entity.ThemeSettingsRepository, logger *slog.Logger) *SettingsService {
	return &SettingsService{repo: repo, logger: logger.With("component", "theme_settings.service"), settings: map[string][]entity.ThemeSetting{}}
}

// Settings - the settings theme dir offers (none when its manifest can't
// be read).
func (s *SettingsService) Settings(dir string) []entity.ThemeSetting {
	key := entity.ThemeKey(dir)
	s.mu.RLock()
	list, ok := s.settings[key]
	s.mu.RUnlock()
	if ok {
		return list
	}
	info, err := LoadInfoFor(dir)
	if err != nil {
		fallback.Log(s.logger, err, "error loading the theme's settings", "theme", key)
	}
	s.mu.Lock()
	s.settings[key] = info.Settings
	s.mu.Unlock()
	return info.Settings
}

// stored - theme key's saved values (none when they can't be loaded).
func (s *SettingsService) stored(ctx context.Context, key string) map[string]string {
	s.mu.RLock()
	all := s.values
	s.mu.RUnlock()
	if all == nil {
		loaded, err := s.repo.All(ctx)
		if err != nil {
			fallback.Log(s.logger, err, "error loading theme settings")
			return nil
		}
		s.mu.Lock()
		s.values = loaded
		s.mu.Unlock()
		all = loaded
	}
	return all[key]
}

// Values - theme dir's setting values: each setting's saved value when
// it's still valid, else its default.
func (s *SettingsService) Values(ctx context.Context, dir string) map[string]string {
	saved := s.stored(ctx, entity.ThemeKey(dir))
	settings := s.Settings(dir)
	out := make(map[string]string, len(settings))
	for _, st := range settings {
		out[st.ID] = st.Default
		if v, ok := saved[st.ID]; ok && st.Valid(v) {
			out[st.ID] = v
		}
	}
	return out
}

// Value - the template value of theme dir's setting id: a bool's as
// bool, the others as string; nil - no such setting.
func (s *SettingsService) Value(ctx context.Context, dir, id string) any {
	for _, st := range s.Settings(dir) {
		if st.ID != id {
			continue
		}
		v := s.Values(ctx, dir)[id]
		if st.Type == entity.ThemeSettingBool {
			return v == "1"
		}
		return v
	}
	return nil
}

// Save stores theme dir's values from in (setting ID → value; a bool
// missing from in is off): every setting's must be valid.
func (s *SettingsService) Save(ctx context.Context, dir string, in map[string]string) error {
	values := map[string]string{}
	for _, st := range s.Settings(dir) {
		v, ok := in[st.ID]
		if !ok && st.Type == entity.ThemeSettingBool {
			v = "0"
		}
		v = strings.TrimSpace(v)
		if st.Type == entity.ThemeSettingColor {
			v = strings.ToLower(v)
		}
		if !st.Valid(v) {
			return fmt.Errorf("theme setting %q: %w", st.ID, ErrSettingInvalid)
		}
		values[st.ID] = v
	}
	return s.store(ctx, dir, values)
}

// Reset - theme dir's settings back to their defaults.
func (s *SettingsService) Reset(ctx context.Context, dir string) error {
	return s.store(ctx, dir, map[string]string{})
}

func (s *SettingsService) store(ctx context.Context, dir string, values map[string]string) error {
	key := entity.ThemeKey(dir)
	if err := s.repo.Save(ctx, key, values); err != nil {
		return fmt.Errorf("saving the settings of theme %q: %w", key, err)
	}
	s.mu.Lock()
	if s.values != nil {
		next := maps.Clone(s.values)
		next[key] = values
		s.values = next
	}
	s.mu.Unlock()
	return nil
}

// ErrSettingInvalid - a value that isn't one of the setting's.
var ErrSettingInvalid = errors.New("invalid value")

// Style - the <style> of theme dir's colours: each colour setting's CSS
// variable in its selector, its value or default (validated "#rrggbb"),
// so the setting is where the colour comes from - a subtheme changing
// its base's default too; "" when the theme has none.
func (s *SettingsService) Style(ctx context.Context, dir string) template.HTML {
	values := s.Values(ctx, dir)
	bySelector := map[string][]string{}
	var selectors []string
	for _, st := range s.Settings(dir) {
		v := values[st.ID]
		if st.CSSVar == "" || !st.Valid(v) {
			continue
		}
		sel := st.CSSSelector()
		if _, ok := bySelector[sel]; !ok {
			selectors = append(selectors, sel)
		}
		bySelector[sel] = append(bySelector[sel], st.CSSVar+": "+v+";")
	}
	if len(selectors) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<style id="theme-settings">`)
	for _, sel := range selectors {
		b.WriteString(sel + " { " + strings.Join(bySelector[sel], " ") + " }")
	}
	b.WriteString("</style>")
	return template.HTML(b.String())
}
