package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToFileEntity converts a generated File to its domain entity, with the
// uploader when loaded.
func ToFileEntity(f *generated.File) entity.File {
	file := entity.File{
		ID:               f.ID,
		URI:              f.URI,
		MimeType:         f.MimeType,
		Width:            f.Width,
		Height:           f.Height,
		SizeBytes:        f.SizeBytes,
		Alt:              f.Alt,
		OriginalFilename: f.OriginalFilename,
		CreatedAt:        f.CreatedAt,
		UpdatedAt:        f.UpdatedAt,
	}

	if f.FocalX != nil && f.FocalY != nil {
		file.Focal = &entity.FocalPoint{X: *f.FocalX, Y: *f.FocalY}
	}

	if f.Edges.Users != nil {
		user := ToUserEntity(f.Edges.Users)
		file.CreatedBy = &user
	}

	return file
}
