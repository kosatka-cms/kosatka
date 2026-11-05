package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToImageStyleDefEntity converts a generated ImageStyle to its domain entity.
func ToImageStyleDefEntity(v *generated.ImageStyle) entity.ImageStyleDef {
	if v == nil {
		return entity.ImageStyleDef{}
	}
	return entity.ImageStyleDef{
		ID:          v.ID,
		Name:        v.Name,
		Label:       v.Label,
		Description: v.Description,
		Width:       v.Width,
		Height:      v.Height,
		Mode:        v.Mode,
		Anchor:      v.Anchor,
		Quality:     v.Quality,
		Format:      v.Format,
		HiDPI:       v.Hidpi,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}
