package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToViewEntity converts a generated View (the container of displays - see
// entity.View) to its domain entity.
func ToViewEntity(v *generated.View) entity.View {
	if v == nil {
		return entity.View{}
	}
	return entity.View{
		ID:          v.ID,
		MachineName: v.MachineName,
		Label:       v.Label,
		CreatedBy:   v.CreatedBy,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}

// ToViewDisplayDefEntity converts a generated ViewDisplayDef to its
// domain entity.
func ToViewDisplayDefEntity(v *generated.ViewDisplayDef) entity.ViewDisplayDef {
	if v == nil {
		return entity.ViewDisplayDef{}
	}
	return entity.ViewDisplayDef{
		ID:                         v.ID,
		BaseEntity:                 entity.ViewBaseEntity(v.BaseEntity),
		ViewID:                     v.ViewID,
		IsDefault:                  v.IsDefault,
		OverrideSettings:           v.OverrideSettings,
		OverrideFilters:            v.OverrideFilters,
		OverrideSort:               v.OverrideSort,
		OverrideFields:             v.OverrideFields,
		MachineName:                v.MachineName,
		Label:                      v.Label,
		MetaTitle:                  v.MetaTitle,
		MetaDescription:            v.MetaDescription,
		URLPath:                    v.URLPath,
		DisplayType:                entity.ViewDisplayType(v.DisplayType),
		DisplayStyle:               entity.ViewDisplayStyle(v.DisplayStyle),
		GridColumns:                v.GridColumns,
		OverflowAfter:              v.OverflowAfter,
		CarouselSlidesPerView:      v.CarouselSlidesPerView,
		CarouselAutoplay:           v.CarouselAutoplay,
		CarouselLoop:               v.CarouselLoop,
		CarouselShowNavigation:     v.CarouselShowNavigation,
		CarouselShowPagination:     v.CarouselShowPagination,
		ShowPagination:             v.ShowPagination,
		PaginationAjax:             v.PaginationAjax,
		ShowGlossary:               v.ShowGlossary,
		GlossaryFieldStorageID:     v.GlossaryFieldStorageID,
		CarouselLinkFieldStorageID: v.CarouselLinkFieldStorageID,
		CarouselEffect:             v.CarouselEffect,
		PageSize:                   v.PageSize,
		SkipItems:                  v.SkipItems,
		HeaderText:                 v.HeaderText,
		FooterText:                 v.FooterText,
		OverflowHeaderText:         v.OverflowHeaderText,
		AttachContentTypes:         v.AttachContentTypes,
		AttachWeight:               v.AttachWeight,
		TermVocabularies:           v.TermVocabularies,
		ProfileTabPath:             v.ProfileTabPath,
		ProfileTabAccess:           entity.ViewProfileTabAccess(v.ProfileTabAccess),
		ThemeTemplate:              v.ThemeTemplate,
		AttachTitle:                v.AttachTitle,
		AttachShowTitle:            v.AttachShowTitle,
		CustomCSS:                  v.CustomCSS,
		RowTemplate:                v.RowTemplate,
		GroupByField:               v.GroupByField,
		GroupDateFormat:            v.GroupDateFormat,
		PublishedSnapshot:          v.PublishedSnapshot,
		PublishedAt:                v.PublishedAt,
		Status:                     v.Status,
		CreatedBy:                  v.CreatedBy,
		CreatedAt:                  v.CreatedAt,
		UpdatedAt:                  v.UpdatedAt,
	}
}

// ToViewFieldDefEntity converts a generated ViewFieldDef to its domain entity.
func ToViewFieldDefEntity(f *generated.ViewFieldDef) entity.ViewFieldDef {
	if f == nil {
		return entity.ViewFieldDef{}
	}
	return entity.ViewFieldDef{
		ID:                 f.ID,
		ViewDisplayID:      f.ViewDisplayID,
		FieldStorageID:     f.FieldStorageID,
		BuiltinField:       entity.ViewFieldBuiltinField(f.BuiltinField),
		Label:              f.Label,
		ShowLabel:          f.ShowLabel,
		ImageStyle:         f.ImageStyle,
		DateFormat:         f.DateFormat,
		TagFormat:          entity.ViewTagFormat(f.TagFormat),
		ImageDisplay:       entity.ViewImageDisplay(f.ImageDisplay),
		Formatter:          f.Formatter,
		Layout:             entity.ParseFieldLayout(f.Layout),
		LinkTo:             entity.ViewFieldLinkTarget(f.LinkTo),
		CSSClass:           f.CSSClass,
		RewriteHTML:        f.RewriteHTML,
		TrimLength:         f.TrimLength,
		SortOrder:          f.SortOrder,
		ExcludeFromDisplay: f.ExcludeFromDisplay,
		HideInOverflow:     f.HideInOverflow,
		FallbackFieldKey:   f.FallbackFieldKey,
		Params:             viewParams(f.Params),
		CreatedAt:          f.CreatedAt,
		UpdatedAt:          f.UpdatedAt,
	}
}

// ToViewSortDefEntity converts a generated ViewSortDef to its domain entity.
func ToViewSortDefEntity(s *generated.ViewSortDef) entity.ViewSortDef {
	if s == nil {
		return entity.ViewSortDef{}
	}
	return entity.ViewSortDef{
		ID:             s.ID,
		ViewDisplayID:  s.ViewDisplayID,
		FieldStorageID: s.FieldStorageID,
		BuiltinField:   entity.ViewSortBuiltin(s.BuiltinField),
		Direction:      s.Direction,
		SortOrder:      s.SortOrder,
		Params:         viewParams(s.Params),
		ExposedKey:     s.ExposedKey,
		ExposedLabel:   s.ExposedLabel,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// viewParams - a View row's params, nil when it has none (so an empty
// map and none compare equal in a published snapshot).
func viewParams(p map[string]string) map[string]string {
	if len(p) == 0 {
		return nil
	}
	return p
}

// ToViewFilterDefEntity converts a generated ViewFilterDef to its domain entity.
func ToViewFilterDefEntity(f *generated.ViewFilterDef) entity.ViewFilterDef {
	if f == nil {
		return entity.ViewFilterDef{}
	}
	return entity.ViewFilterDef{
		ID:                f.ID,
		ViewDisplayID:     f.ViewDisplayID,
		FieldStorageID:    f.FieldStorageID,
		ViaFieldStorageID: f.ViaFieldStorageID,
		BuiltinField:      entity.ViewFilterBuiltinField(f.BuiltinField),
		Operator:          entity.ViewFilterOperator(f.Operator),
		Value:             f.Value,
		ValueIsRelative:   f.ValueIsRelative,
		Exposed:           f.Exposed,
		ExposedParam:      f.ExposedParam,
		ExposedLabel:      f.ExposedLabel,
		Contextual:        f.Contextual,
		SortOrder:         f.SortOrder,
		CreatedAt:         f.CreatedAt,
		UpdatedAt:         f.UpdatedAt,
	}
}
