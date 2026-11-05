package imagestyle

import (
	"context"
	"errors"
	"fmt"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// StyleService serves image style derivatives, generating and caching them on demand.
type StyleService struct {
	styleRepo   ImageStyleRepository
	storageRepo ImageStorageRepository
	processor   ImageProcessor
	focal       FocalPoints
	failed      failedOriginals
}

// FocalPoints - the focal points of the originals, by their keys in the
// service's store (nil - none: not found, or none set).
type FocalPoints interface {
	FocalPoint(ctx context.Context, key string) (*FocalPoint, error)
}

// SetFocalPoints - where a style cropping at the focal point
// (ImageStyle.UsesFocal) finds it; none - at the style's anchor.
func (s *StyleService) SetFocalPoints(f FocalPoints) {
	s.focal = f
}

// NewStyleService - a StyleService with the styles from styleRepo, the
// derivatives kept by storageRepo and rendered by processor.
func NewStyleService(
	styleRepo ImageStyleRepository,
	storageRepo ImageStorageRepository,
	processor ImageProcessor,
) *StyleService {
	return &StyleService{
		styleRepo:   styleRepo,
		storageRepo: storageRepo,
		processor:   processor,
	}
}

// GetStyledImage opens the derivative of relativePath for styleName,
// generating it first if needed; relativePath may be a DoublePath (the
// 2x derivative of a HiDPI style; of another - ErrStyleNotFound). The
// caller closes the file.
func (s *StyleService) GetStyledImage(ctx context.Context, styleName, relativePath string) (filestore.File, filestore.Info, error) {
	style, err := s.styleRepo.FindByName(styleName)
	if err != nil {
		return nil, filestore.Info{}, err
	}
	double, source := SplitDouble(relativePath)
	if style, err = doubled(style, double); err != nil {
		return nil, filestore.Info{}, err
	}

	f, info, err := s.storageRepo.OpenCached(ctx, styleName, relativePath)
	if err == nil || !errors.Is(err, filestore.ErrNotExist) {
		return f, info, err
	}

	if err := s.generate(ctx, style, source, relativePath); err != nil {
		return nil, filestore.Info{}, err
	}
	return s.storageRepo.OpenCached(ctx, styleName, relativePath)
}

// doubled - style at double size (false - style itself);
// ErrStyleNotFound for a style without it.
func doubled(style *ImageStyle, double bool) (*ImageStyle, error) {
	if !double {
		return style, nil
	}
	v, ok := style.Double()
	if !ok {
		return nil, fmt.Errorf("%w: %s at 2x", ErrStyleNotFound, style.Name())
	}
	return v, nil
}

// GetStyledOriginal opens styleName's derivative of the original at key
// exactly - never another file whose name maps to the same derivative's
// (a.png and a.jpg both to a.webp): it is cached under the original's
// whole name, <key><format's extension> (double - the 2x one of a HiDPI
// style, under DoublePath). For files whose access is checked by their
// key (the private files). The caller closes the file.
func (s *StyleService) GetStyledOriginal(ctx context.Context, styleName string, double bool, key string) (filestore.File, filestore.Info, error) {
	style, err := s.styleRepo.FindByName(styleName)
	if err != nil {
		return nil, filestore.Info{}, err
	}
	if style, err = doubled(style, double); err != nil {
		return nil, filestore.Info{}, err
	}
	derivative := key + style.Format().Extension()
	if double {
		derivative = DoublePath(derivative)
	}
	f, info, err := s.storageRepo.OpenCached(ctx, styleName, derivative)
	if err == nil || !errors.Is(err, filestore.ErrNotExist) {
		return f, info, err
	}
	src, srcInfo, err := s.storageRepo.OpenExact(ctx, key)
	if err != nil {
		return nil, filestore.Info{}, err
	}
	defer func() { _ = src.Close() }()
	if err := s.render(ctx, style, src, srcInfo, derivative); err != nil {
		return nil, filestore.Info{}, err
	}
	return s.storageRepo.OpenCached(ctx, styleName, derivative)
}

// generate renders the style's derivative of the original source
// (its path, or a derivative's) and stores it as relativePath.
func (s *StyleService) generate(ctx context.Context, style *ImageStyle, source, relativePath string) error {
	src, info, err := s.storageRepo.OpenOriginal(ctx, source)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	return s.render(ctx, style, src, info, relativePath)
}

// render processes src (the original info) in style and stores the
// derivative as relativePath; an original that failed to render lately
// isn't decoded again (ErrNotRenderable).
func (s *StyleService) render(ctx context.Context, style *ImageStyle, src filestore.File, info filestore.Info, relativePath string) error {
	if s.failed.has(info) {
		return fmt.Errorf("%w: %s failed lately", ErrNotRenderable, info.Key)
	}
	var focal *FocalPoint
	if style.UsesFocal() && s.focal != nil {
		var err error
		if focal, err = s.focal.FocalPoint(ctx, info.Key); err != nil {
			return fmt.Errorf("the focal point of %s: %w", info.Key, err)
		}
	}
	processed, err := s.processor.Process(src, info.Name, style, style.Anchor(), focal)
	if err != nil {
		if errors.Is(err, ErrNotRenderable) {
			s.failed.add(info)
		}
		return fmt.Errorf("rendering %s in style %s: %w", info.Key, style.Name(), err)
	}
	return s.storageRepo.SaveToCache(ctx, style.Name(), relativePath, processed)
}
