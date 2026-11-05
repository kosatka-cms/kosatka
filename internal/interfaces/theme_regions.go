package interfaces

import (
	"context"
	"log/slog"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// ThemeRegionsFor - the regions of the theme with placement key themeKey
// (theme.yaml, themes.LoadInfoFor); an error is logged and leaves no
// regions, so the block pages refuse to place anything rather than guess.
func ThemeRegionsFor(logger *slog.Logger, themeKey string) entity.ThemeInfo {
	info, err := themes.LoadInfoFor(entity.ThemeDir(themeKey))
	if err != nil && logger != nil {
		logger.Error("error loading theme regions", "error", err, "theme", themeKey)
	}
	return info
}

// ThemeRegions - the active theme's regions (the rendered layout).
func ThemeRegions(logger *slog.Logger) entity.ThemeInfo {
	return ThemeRegionsFor(logger, service.ActiveThemeKey())
}

// RequestThemeRegions - the regions of the theme ctx renders in (a
// previewed one, else the active one).
func RequestThemeRegions(ctx context.Context, logger *slog.Logger) entity.ThemeInfo {
	return ThemeRegionsFor(logger, entity.ThemeKey(paths.ThemeFor(ctx)))
}

// TranslatedRegionLabels - theme.Labels() translated: the regions' labels
// of theme.yaml (English, in the engine's or the theme's catalog) and the
// "Hidden" one (entity.RegionDisabled).
func TranslatedRegionLabels(ctx context.Context, theme entity.ThemeInfo) map[entity.Region]string {
	labels := theme.Labels()
	for region, label := range labels {
		labels[region] = i18n.T(ctx, label)
	}
	return labels
}
