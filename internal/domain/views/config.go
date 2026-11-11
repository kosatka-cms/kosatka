package views

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/fallback"
)

// Config - a display's configuration as a run and its rendering use it:
// View - the display with its settings resolved (against the View's
// Default display, or the published snapshot's), its filters, sort
// criteria and fields.
type Config struct {
	View    entity.ViewDisplayDef
	Filters entity.ViewFilterDefs
	Sorts   entity.ViewSortDefs
	Fields  entity.ViewFieldDefs
	// Published - from the display's published snapshot.
	Published bool
}

// RunContext - what a run takes besides the configuration: the query
// string's values (exposed filters, the visitors' sort, the glossary
// letter), the page (1-based; 0 - the first) and the contextual filters'
// values - CreatedByID (created_by: a profile, a URL argument, the
// rendered node's author), NodeID (the rendered node: contextual node
// filters, and it is left out of the results), TermID (has_current_term).
type RunContext struct {
	QueryParams map[string]string
	Page        int
	CreatedByID int
	NodeID      int
	TermID      int
}

// PublishedConfig - see Service.PublishedConfig.
func (s *DefaultService) PublishedConfig(ctx context.Context, view entity.ViewDisplayDef) (Config, error) {
	return s.publishedConfig(ctx, view, true)
}

// LiveConfig - see Service.LiveConfig.
func (s *DefaultService) LiveConfig(ctx context.Context, view entity.ViewDisplayDef) (Config, error) {
	return s.liveConfig(ctx, view, true)
}

// publishedConfig - PublishedConfig; without withFields the live
// configuration's fields aren't loaded (a run needs none).
func (s *DefaultService) publishedConfig(ctx context.Context, view entity.ViewDisplayDef, withFields bool) (Config, error) {
	if view.PublishedAt != nil {
		snap, err := s.DecodePublishedSnapshot(view)
		if err == nil {
			return Config{View: snap.Settings.ApplyTo(view), Filters: snap.Filters, Sorts: snap.Sorts, Fields: snap.Fields, Published: true}, nil
		}
		fallback.Log(s.logger, fmt.Errorf("decoding the published snapshot (viewID %v): %w", view.ID, err), "a View's published version is unreadable: its live one shown", "viewID", view.ID)
	}
	return s.liveConfig(ctx, view, withFields)
}

// liveConfig - LiveConfig, its fields only withFields.
func (s *DefaultService) liveConfig(ctx context.Context, view entity.ViewDisplayDef, withFields bool) (Config, error) {
	effective, err := s.EffectiveSettings(ctx, view)
	if err != nil {
		return Config{}, fmt.Errorf("resolving effective settings (viewID %v): %w", view.ID, err)
	}
	cfg := Config{View: effective}
	if cfg.Filters, err = s.EffectiveFiltersByView(ctx, view); err != nil {
		return Config{}, fmt.Errorf("loading view filters (viewID %v): %w", view.ID, err)
	}
	if cfg.Sorts, err = s.EffectiveSortsByView(ctx, view); err != nil {
		return Config{}, fmt.Errorf("loading view sorts (viewID %v): %w", view.ID, err)
	}
	if withFields {
		if cfg.Fields, err = s.EffectiveFieldsByView(ctx, view); err != nil {
			return Config{}, fmt.Errorf("loading view fields (viewID %v): %w", view.ID, err)
		}
	}
	return cfg, nil
}

// RunConfig - see Service.RunConfig.
func (s *DefaultService) RunConfig(ctx context.Context, cfg Config, rc RunContext) (entity.Nodes, int, error) {
	return s.runView(ctx, cfg.View, cfg.Filters, cfg.Sorts, rc.QueryParams, max(rc.Page, 1), rc.CreatedByID, rc.NodeID, rc.TermID)
}

// RunCommentConfig - see Service.RunCommentConfig.
func (s *DefaultService) RunCommentConfig(ctx context.Context, cfg Config, rc RunContext) (entity.Comments, int, error) {
	return s.runCommentView(ctx, cfg.View, cfg.Filters, cfg.Sorts, rc.QueryParams, max(rc.Page, 1), rc.CreatedByID)
}

// GlossaryOf - see Service.GlossaryOf.
func (s *DefaultService) GlossaryOf(ctx context.Context, cfg Config, queryParams map[string]string) (entity.GlossaryData, error) {
	if !cfg.View.ShowGlossary {
		return entity.GlossaryData{}, nil
	}
	return s.glossary(ctx, cfg.View, cfg.Filters, queryParams)
}

// runPublished - a run of view as visitors see it (PublishedConfig, no
// fields).
func (s *DefaultService) runPublished(ctx context.Context, view entity.ViewDisplayDef, rc RunContext) (entity.Nodes, int, error) {
	cfg, err := s.publishedConfig(ctx, view, false)
	if err != nil {
		return nil, 0, err
	}
	return s.RunConfig(ctx, cfg, rc)
}

// runPublishedComments - runPublished for a comment View.
func (s *DefaultService) runPublishedComments(ctx context.Context, view entity.ViewDisplayDef, rc RunContext) (entity.Comments, int, error) {
	cfg, err := s.publishedConfig(ctx, view, false)
	if err != nil {
		return nil, 0, err
	}
	return s.RunCommentConfig(ctx, cfg, rc)
}
