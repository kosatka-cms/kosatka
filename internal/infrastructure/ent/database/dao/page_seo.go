package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToPageSeoEntity converts a generated PageSeo to entity.PageSEO.
func ToPageSeoEntity(c *generated.PageSeo) entity.PageSEO {
	return entity.PageSEO{
		Title:       c.Title,
		Description: c.Description,
	}
}
