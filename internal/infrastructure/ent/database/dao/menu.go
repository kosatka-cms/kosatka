package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToMenuDefEntity converts a generated MenuDef to its domain entity.
func ToMenuDefEntity(v *generated.MenuDef) entity.MenuDef {
	if v == nil {
		return entity.MenuDef{}
	}
	return entity.MenuDef{
		ID:          v.ID,
		MachineName: v.MachineName,
		Label:       v.Label,
		Description: v.Description,
		InSitemap:   v.InSitemap,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}

// ToMenuLinkDefEntity converts a generated MenuLinkDef to its domain entity.
func ToMenuLinkDefEntity(v *generated.MenuLinkDef) entity.MenuLinkDef {
	if v == nil {
		return entity.MenuLinkDef{}
	}
	return entity.MenuLinkDef{
		ID:         v.ID,
		MenuID:     v.MenuID,
		ParentID:   v.ParentID,
		Title:      v.Title,
		LinkType:   entity.MenuLinkType(v.LinkType),
		LinkURL:    v.LinkURL,
		LinkNodeID: v.LinkNodeID,
		LinkTermID: v.LinkTermID,
		LinkViewID: v.LinkViewID,
		Weight:     v.Weight,
		Enabled:    v.Enabled,
		CreatedAt:  v.CreatedAt,
		UpdatedAt:  v.UpdatedAt,
	}
}
