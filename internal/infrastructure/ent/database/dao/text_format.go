package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToTextFormatDefEntity converts a generated TextFormatDef to its domain entity.
func ToTextFormatDefEntity(v *generated.TextFormatDef) entity.TextFormatDef {
	if v == nil {
		return entity.TextFormatDef{}
	}
	return entity.TextFormatDef{
		ID:                  v.ID,
		MachineName:         v.MachineName,
		Label:               v.Label,
		Weight:              v.Weight,
		AllowedTags:         v.AllowedTags,
		AllowClassAttribute: v.AllowClassAttribute,
		HasRichEditor:       v.HasRichEditor,
		TinyMCEPlugins:      v.TinymcePlugins,
		TinyMCEToolbar:      v.TinymceToolbar,
		AllowedExtensions:   v.AllowedExtensions,
		MaxFileSizeBytes:    v.MaxFileSizeBytes,
		Status:              v.Status,
		Filters:             v.Filters,
		CreatedBy:           v.CreatedBy,
		CreatedAt:           v.CreatedAt,
		UpdatedAt:           v.UpdatedAt,
	}
}
