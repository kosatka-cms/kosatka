package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

// ErrImageStyleDuplicate - the name is taken by another image style.
var ErrImageStyleDuplicate = errors.New("image style name already taken")

// ImageStyleService - the image styles: the sizes and formats the images'
// derivatives are made in.
type ImageStyleService interface {
	ImageStyles(ctx context.Context) (entity.ImageStyleDefs, error)
	ImageStyleByID(ctx context.Context, id int) (*entity.ImageStyleDef, error)
	ImageStyleByName(ctx context.Context, name string) (*entity.ImageStyleDef, error)
	CreateImageStyle(ctx context.Context, s entity.ImageStyleDef) (*entity.ImageStyleDef, error)
	UpdateImageStyle(ctx context.Context, s entity.ImageStyleDef) (*entity.ImageStyleDef, error)
	DeleteImageStyle(ctx context.Context, id int) error
}

// DefaultImageStyleService - the ImageStyleService on the image styles'
// repository; a saved or deleted style updates the derivatives' URLs
// (imageurl).
type DefaultImageStyleService struct {
	repository entity.ImageStyleRepository
	logger     *slog.Logger
}

// NewImageStyleService - the image style service on r (the styles) and
// baseLogger.
func NewImageStyleService(r entity.ImageStyleRepository, baseLogger *slog.Logger) ImageStyleService {
	return &DefaultImageStyleService{
		repository: r,
		logger:     baseLogger.With("component", "image_style.service"),
	}
}

// ImageStyles - every image style.
func (s *DefaultImageStyleService) ImageStyles(ctx context.Context) (entity.ImageStyleDefs, error) {
	styles, err := s.repository.ImageStyles(ctx)
	return entity.Wrapped(styles, err, "listing image styles")
}

// ImageStyleByID - the image style id.
func (s *DefaultImageStyleService) ImageStyleByID(ctx context.Context, id int) (*entity.ImageStyleDef, error) {
	style, err := s.repository.ImageStyleByID(ctx, id)
	return entity.Found(style, err, "finding image style by id (id %v)", id)
}

// ImageStyleByName - the image style named name.
func (s *DefaultImageStyleService) ImageStyleByName(ctx context.Context, name string) (*entity.ImageStyleDef, error) {
	style, err := s.repository.ImageStyleByName(ctx, name)
	return entity.Found(style, err, "finding image style by name (name %v)", name)
}

// CreateImageStyle saves the new image style (a taken name -
// ErrImageStyleDuplicate) and gives its derivatives' URLs its format,
// version and size.
func (s *DefaultImageStyleService) CreateImageStyle(ctx context.Context, style entity.ImageStyleDef) (*entity.ImageStyleDef, error) {
	created, err := s.repository.CreateImageStyle(ctx, style)
	if err != nil {
		if isDuplicateKeyErr(err) {
			return nil, fmt.Errorf("%w", ErrImageStyleDuplicate)
		}
		return nil, fmt.Errorf("creating image style (name %v): %w", style.Name, err)
	}
	imageurl.SetStyleFormat(created.Name, imagestyle.ParseFormat(created.Format))
	imageurl.SetStyleVersion(created.Name, ImageStyleVersion(*created))
	imageurl.SetStyleSize(created.Name, ImageStyleSize(*created))
	return created, nil
}

// UpdateImageStyle saves the image style and gives its derivatives' URLs
// its format, version and size.
func (s *DefaultImageStyleService) UpdateImageStyle(ctx context.Context, style entity.ImageStyleDef) (*entity.ImageStyleDef, error) {
	updated, err := s.repository.UpdateImageStyle(ctx, style)
	if err != nil {
		return nil, fmt.Errorf("updating image style (id %v): %w", style.ID, err)
	}
	imageurl.SetStyleFormat(updated.Name, imagestyle.ParseFormat(updated.Format))
	imageurl.SetStyleVersion(updated.Name, ImageStyleVersion(*updated))
	imageurl.SetStyleSize(updated.Name, ImageStyleSize(*updated))
	return updated, nil
}

// DeleteImageStyle deletes the image style id; its name's derivatives'
// URLs go back to the defaults.
func (s *DefaultImageStyleService) DeleteImageStyle(ctx context.Context, id int) error {
	existing, _ := s.repository.ImageStyleByID(ctx, id)
	if err := s.repository.DeleteImageStyle(ctx, id); err != nil {
		return fmt.Errorf("deleting image style (id %v): %w", id, err)
	}
	if existing != nil {
		imageurl.SetStyleFormat(existing.Name, imagestyle.FormatWebP)
		imageurl.SetStyleVersion(existing.Name, "")
		imageurl.SetStyleSize(existing.Name, imageurl.StyleSize{})
	}
	return nil
}

// ImageStyleVersion - the version of a style's settings in its
// derivatives' URLs (imageurl.SetStyleVersions): a hash of what the
// derivative is made by, so changing the style changes the URLs, and
// setting it back brings the old ones back.
func ImageStyleVersion(d entity.ImageStyleDef) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%d|%d|%s|%s|%d|%s", d.Width, d.Height, d.Mode, d.Anchor, d.Quality, imagestyle.ParseFormat(d.Format)))
	return hex.EncodeToString(sum[:4])
}

// ImageStyleSize - what a style's <img> is built from
// (imageurl.SetStyleSizes). HiDPI isn't in ImageStyleVersion: the 2x
// derivatives have their own folder, and the style's own don't change
// with it.
func ImageStyleSize(d entity.ImageStyleDef) imageurl.StyleSize {
	return imageurl.StyleSize{Width: d.Width, Height: d.Height, Mode: imagestyle.ResizeMode(d.Mode), HiDPI: d.HiDPI,
		Focal: d.Anchor == entity.ImageStyleAnchorFocal}
}

// LoadImageStyleFormats fills the style → format, style → version and
// style → size maps styled image URLs and <img> are built from
// (imageurl.SetStyleFormats, SetStyleVersions, SetStyleSizes) - called
// once at startup; every admin change keeps them in sync afterwards.
func LoadImageStyleFormats(ctx context.Context, styles ImageStyleService) error {
	defs, err := styles.ImageStyles(ctx)
	if err != nil {
		return err
	}
	formats := make(map[string]imagestyle.Format, len(defs))
	versions := make(map[string]string, len(defs))
	sizes := make(map[string]imageurl.StyleSize, len(defs))
	for _, d := range defs {
		formats[d.Name] = imagestyle.ParseFormat(d.Format)
		versions[d.Name] = ImageStyleVersion(d)
		sizes[d.Name] = ImageStyleSize(d)
	}
	imageurl.SetStyleFormats(formats)
	imageurl.SetStyleVersions(versions)
	imageurl.SetStyleSizes(sizes)
	return nil
}
