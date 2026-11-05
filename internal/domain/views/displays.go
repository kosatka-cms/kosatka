package views

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// effectiveOwnerID - the display ID a section should be read from: the
// display's own when it has no group, is the Default, or overrides this
// section; otherwise its View's Default.
func (s *DefaultService) effectiveOwnerID(ctx context.Context, display entity.ViewDisplayDef, overridden bool) (int, error) {
	if display.IsDefault || display.ViewID == nil || overridden {
		return display.ID, nil
	}
	def, err := s.repository.DefaultDisplayOfView(ctx, *display.ViewID)
	if err != nil {
		return 0, err
	}
	return def.ID, nil
}

// EffectiveSettings - a copy of display whose Settings fields
// (DisplayStyle, GridColumns, Carousel*, pagination, glossary, PageSize,
// HeaderText/FooterText, CustomCSS) come from its View's Default when
// inherited. Identity fields (MachineName, Label, meta, URLPath,
// DisplayType, Status, IDs, Override*) always stay the display's own.
func (s *DefaultService) EffectiveSettings(ctx context.Context, display entity.ViewDisplayDef) (entity.ViewDisplayDef, error) {
	if display.IsDefault || display.ViewID == nil || display.OverrideSettings {
		return display, nil
	}
	def, err := s.repository.DefaultDisplayOfView(ctx, *display.ViewID)
	if err != nil {
		return display, err
	}
	return display.WithSettingsOf(*def), nil
}

// CloneDisplay - the only way a View container is created. If source
// has no group yet: create a View and a Default seeded with source's
// config and rows, then mark source fully overridden (it keeps its own
// rows, so its output is unchanged). Then a new sibling is created and
// returned.
func (s *DefaultService) CloneDisplay(ctx context.Context, sourceDisplayID int, createdBy int) (*entity.ViewDisplayDef, error) {
	source, err := s.editableDisplay(ctx, sourceDisplayID)
	if err != nil {
		return nil, err
	}

	if source.ViewID == nil {
		group, err := s.createViewContainer(ctx, entity.View{
			Label:     source.Label,
			CreatedBy: createdBy,
		})
		if err != nil {
			return nil, err
		}

		defURLPath, err := s.uniqueViewSlug(ctx, "__default__/"+source.MachineName, s.urlPathTakenAny)
		if err != nil {
			return nil, err
		}
		def, err := s.createDisplay(ctx, entity.ViewDisplayDef{
			ViewID:      &group.ID,
			IsDefault:   true,
			Label:       source.Label + " - Default",
			URLPath:     defURLPath,
			BaseEntity:  source.BaseEntity,
			DisplayType: source.DisplayType,
			Status:      0,
			CreatedBy:   createdBy,
		}.WithSettingsOf(*source))
		if err != nil {
			return nil, err
		}

		if err := copyRows(ctx, s.filters(), source.ID, def.ID); err != nil {
			return nil, err
		}
		if err := copyRows(ctx, s.sorts(), source.ID, def.ID); err != nil {
			return nil, err
		}
		if err := copyRows(ctx, s.fields(), source.ID, def.ID); err != nil {
			return nil, err
		}

		source.ViewID = &group.ID
		source.OverrideSettings, source.OverrideFilters, source.OverrideSort, source.OverrideFields = true, true, true, true
		updatedSource, err := s.repository.UpdateView(ctx, *source)
		if err != nil {
			return nil, err
		}
		source = updatedSource
	}

	// Displays without a URL don't get one on the copy either.
	var cloneURLPath string
	if source.DisplayType.HasURL() {
		var err error
		cloneURLPath, err = s.uniqueViewSlug(ctx, source.URLPath+"-copy", s.pageURLTaken)
		if err != nil {
			return nil, err
		}
	}

	// Copy source's own per-section state, not Default's: an overridden
	// section becomes an independent override seeded with source's values;
	// an inherited section stays inherited.
	clone := entity.ViewDisplayDef{
		ViewID:             source.ViewID,
		IsDefault:          false,
		Label:              i18n.T(ctx, "@label (copy)", "@label", source.Label),
		MetaTitle:          source.MetaTitle,
		MetaDescription:    source.MetaDescription,
		URLPath:            cloneURLPath,
		BaseEntity:         source.BaseEntity,
		DisplayType:        source.DisplayType,
		AttachContentTypes: source.AttachContentTypes,
		AttachWeight:       source.AttachWeight,
		AttachTitle:        source.AttachTitle,
		AttachShowTitle:    source.AttachShowTitle,
		TermVocabularies:   source.TermVocabularies,
		ProfileTabPath:     "",
		ProfileTabAccess:   source.ProfileTabAccess,
		ThemeTemplate:      source.ThemeTemplate,
		Status:             0,
		CreatedBy:          createdBy,
		OverrideSettings:   source.OverrideSettings,
		OverrideFilters:    source.OverrideFilters,
		OverrideSort:       source.OverrideSort,
		OverrideFields:     source.OverrideFields,
	}
	if source.OverrideSettings {
		clone = clone.WithSettingsOf(*source)
	}

	created, err := s.createDisplay(ctx, clone)
	if err != nil {
		return nil, err
	}

	if source.OverrideFilters {
		if err := copyRows(ctx, s.filters(), source.ID, created.ID); err != nil {
			return nil, err
		}
	}
	if source.OverrideSort {
		if err := copyRows(ctx, s.sorts(), source.ID, created.ID); err != nil {
			return nil, err
		}
	}
	if source.OverrideFields {
		if err := copyRows(ctx, s.fields(), source.ID, created.ID); err != nil {
			return nil, err
		}
	}
	return created, nil
}

// AddDisplay - adds a sibling that inherits Settings/Filters/Sort/
// Fields from view's Default (all OverrideX flags false, nothing
// copied). Requires an existing group; CloneDisplay creates one.
func (s *DefaultService) AddDisplay(ctx context.Context, viewID int, createdBy int) (*entity.ViewDisplayDef, error) {
	group, err := s.repository.ViewContainerByID(ctx, viewID)
	if err != nil {
		return nil, err
	}

	// url_path is built from the human-readable group.Label (the
	// container's machine_name is id-based); machine_name itself is
	// assigned by createDisplay.
	base := slug.Generate(group.Label) + "-page"
	urlPath, err := s.uniqueViewSlug(ctx, base, s.pageURLTaken)
	if err != nil {
		return nil, err
	}

	// baseEntity is inherited from the View's Default: all displays of one
	// View share the same base table.
	var baseEntity entity.ViewBaseEntity
	if def, err := s.repository.DefaultDisplayOfView(ctx, viewID); err == nil {
		baseEntity = def.BaseEntity
	}

	return s.createDisplay(ctx, entity.ViewDisplayDef{
		ViewID:      &group.ID,
		IsDefault:   false,
		Label:       i18n.T(ctx, "@label - new display", "@label", group.Label),
		URLPath:     urlPath,
		BaseEntity:  baseEntity,
		DisplayType: entity.ViewDisplayTypePage,
		Status:      0,
		CreatedBy:   createdBy,
	})
}

// OverrideSettings forks the "Settings" section: snapshots its current
// effective (possibly inherited) values into display's own columns,
// then sets OverrideSettings so it stops following its View's Default.
func (s *DefaultService) OverrideSettings(ctx context.Context, displayID int) (*entity.ViewDisplayDef, error) {
	display, err := s.editableDisplay(ctx, displayID)
	if err != nil {
		return nil, err
	}
	effective, err := s.EffectiveSettings(ctx, *display)
	if err != nil {
		return nil, err
	}
	effective.OverrideSettings = true
	return s.repository.UpdateView(ctx, effective)
}

// RevertSettings discards display's own Settings columns (they simply
// stop being read, see effectiveOwnerID) and goes back to following
// its View's Default.
func (s *DefaultService) RevertSettings(ctx context.Context, displayID int) (*entity.ViewDisplayDef, error) {
	display, err := s.editableDisplay(ctx, displayID)
	if err != nil {
		return nil, err
	}
	display.OverrideSettings = false
	return s.repository.UpdateView(ctx, *display)
}
