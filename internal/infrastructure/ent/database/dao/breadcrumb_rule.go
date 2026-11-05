package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToBreadcrumbRuleDefEntity converts a generated BreadcrumbRuleDef to its
// domain entity.
func ToBreadcrumbRuleDefEntity(v *generated.BreadcrumbRuleDef) entity.BreadcrumbRuleDef {
	return entity.BreadcrumbRuleDef{
		ID:             v.ID,
		ContentType:    v.ContentType,
		FieldStorageID: v.FieldStorageID,
		TermIDs:        v.TermIds,
		MenuLinkID:     v.MenuLinkID,
		Weight:         v.Weight,
		CreatedAt:      v.CreatedAt,
		UpdatedAt:      v.UpdatedAt,
	}
}
