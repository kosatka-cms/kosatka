package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToBlockDefEntity converts a generated BlockDef to its domain entity.
func ToBlockDefEntity(v *generated.BlockDef) entity.BlockDef {
	if v == nil {
		return entity.BlockDef{}
	}
	b := entity.BlockDef{
		ID:           v.ID,
		Title:        v.Title,
		ShowTitle:    v.ShowTitle,
		Body:         v.Body,
		TextFormatID: v.TextFormatID,
		Status:       v.Status,
		CreatedAt:    v.CreatedAt,
		UpdatedAt:    v.UpdatedAt,
	}
	if v.MachineName != nil {
		b.MachineName = *v.MachineName
	}
	return b
}

// ToBlockPlacementDefEntity converts a generated BlockPlacementDef to
// its domain entity.
func ToBlockPlacementDefEntity(v *generated.BlockPlacementDef) entity.BlockPlacementDef {
	if v == nil {
		return entity.BlockPlacementDef{}
	}
	var pluginID *entity.PluginBlockID
	if v.PluginID != nil {
		id := entity.PluginBlockID(*v.PluginID)
		pluginID = &id
	}
	return entity.BlockPlacementDef{
		ID:                    v.ID,
		BlockID:               v.BlockID,
		PlacementType:         entity.BlockPlacementType(v.PlacementType),
		PluginID:              pluginID,
		PluginConfig:          v.PluginConfig,
		ViewDisplayID:         v.ViewDisplayID,
		MenuID:                v.MenuID,
		MenuStartLevel:        v.MenuStartLevel,
		MenuLevelsToShow:      v.MenuLevelsToShow,
		Theme:                 v.Theme,
		Region:                entity.Region(v.Region),
		Weight:                v.Weight,
		Title:                 v.Title,
		ShowTitle:             v.ShowTitle,
		VisibilityScope:       entity.BlockVisibilityScope(v.VisibilityScope),
		VisibilityContentType: v.VisibilityContentType,
		VisibilityPages:       v.VisibilityPages,
		VisibilityPagesMode:   entity.BlockVisibilityPagesMode(v.VisibilityPagesMode),
		VisibilityRoles:       v.VisibilityRoles,
		Lazy:                  v.Lazy,
		CreatedAt:             v.CreatedAt,
		UpdatedAt:             v.UpdatedAt,
	}
}
