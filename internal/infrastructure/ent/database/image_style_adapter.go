package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/disintegration/imaging"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

// ImageStyleRepositoryAdapter implements pkg/imagestyle.ImageStyleRepository
// (context-free) on top of the DB-backed entity.ImageStyleRepository, so the
// resize/serve pipeline (imagestyle.StyleService) resolves styles managed
// in the admin UI.
type ImageStyleRepositoryAdapter struct {
	repo entity.ImageStyleRepository
}

// NewImageStyleRepositoryAdapter - the adapter resolving styles through
// repo.
func NewImageStyleRepositoryAdapter(repo entity.ImageStyleRepository) *ImageStyleRepositoryAdapter {
	return &ImageStyleRepositoryAdapter{repo: repo}
}

// FindByName - the style named name for the resize pipeline;
// imagestyle.ErrStyleNotFound if there is none.
func (a *ImageStyleRepositoryAdapter) FindByName(name string) (*imagestyle.ImageStyle, error) {
	def, err := a.repo.ImageStyleByName(context.Background(), name)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, fmt.Errorf("%w: %w", imagestyle.ErrStyleNotFound, err)
		}
		return nil, err
	}
	return toPkgImageStyle(*def)
}

func toPkgImageStyle(def entity.ImageStyleDef) (*imagestyle.ImageStyle, error) {
	style, err := imagestyle.NewImageStyle(
		def.Name, def.Label, def.Width, def.Height,
		imagestyle.ResizeMode(def.Mode), stringToAnchor(def.Anchor), def.Quality,
	)
	if err != nil {
		return nil, fmt.Errorf("image style %q: %w", def.Name, err)
	}
	style.SetDescription(def.Description)
	style.SetFormat(imagestyle.ParseFormat(def.Format))
	style.SetHiDPI(def.HiDPI)
	style.SetFocal(def.Anchor == entity.ImageStyleAnchorFocal)
	return style, nil
}

// FileFocalPoints implements imagestyle.FocalPoints over the files table:
// the focal point of the file stored at Scheme + key (public://,
// private://).
type FileFocalPoints struct {
	Files  entity.FileRepository
	Scheme string
}

// FocalPoint - the focal point of the file stored under key (as a share
// of its width and height); nil when the file isn't known or has none.
func (f FileFocalPoints) FocalPoint(ctx context.Context, key string) (*imagestyle.FocalPoint, error) {
	file, err := f.Files.ByURI(ctx, f.Scheme+key)
	if errors.Is(err, entity.ErrEntityNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if file.Focal == nil {
		return nil, nil
	}
	return &imagestyle.FocalPoint{X: float64(file.Focal.X) / 100, Y: float64(file.Focal.Y) / 100}, nil
}

func stringToAnchor(s string) imaging.Anchor {
	switch s {
	case "top-left":
		return imaging.TopLeft
	case "top":
		return imaging.Top
	case "top-right":
		return imaging.TopRight
	case "left":
		return imaging.Left
	case "right":
		return imaging.Right
	case "bottom-left":
		return imaging.BottomLeft
	case "bottom":
		return imaging.Bottom
	case "bottom-right":
		return imaging.BottomRight
	default:
		return imaging.Center
	}
}
