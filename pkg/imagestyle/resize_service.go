package imagestyle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"

	// Decoders image.Decode needs for these formats.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// ResizeService generates WxH image previews, with the size in the file
// name, in the public files' store.
type ResizeService struct {
	processor ImageProcessor
	store     filestore.Store
	urlPrefix string // public URL prefix of the store's files, e.g. /uploads/
	// storeURL - where the store serves its files itself ("" - the site
	// serves them at urlPrefix).
	storeURL string
	failed   failedOriginals
}

// NewResizeService - previews of the files of store, which the site
// serves at urlPrefix; storeURL - the store's own address for them, when
// it serves them itself ("" - none).
func NewResizeService(processor ImageProcessor, store filestore.Store, urlPrefix, storeURL string) *ResizeService {
	return &ResizeService{
		processor: processor,
		store:     store,
		urlPrefix: urlPrefix,
		storeURL:  storeURL,
	}
}

// fileURL - the address of the store's file key: the store's own when it
// serves its files, else the site's.
func (s *ResizeService) fileURL(key string) string {
	if s.storeURL != "" {
		return s.storeURL + key
	}
	return s.urlPrefix + key
}

// OriginalURL - the address the file at originalPath (under urlPrefix)
// is served at; originalPath itself when it isn't one of the files.
func (s *ResizeService) OriginalURL(originalPath string) string {
	rel, err := s.relativePath(originalPath)
	if err != nil {
		return originalPath
	}
	return s.fileURL(rel)
}

// URLPrefix - the public URL prefix of the files directory.
func (s *ResizeService) URLPrefix() string { return s.urlPrefix }

// relativePath - the key of the file at originalPath (a URL path under
// urlPrefix, with or without its leading slash).
func (s *ResizeService) relativePath(originalPath string) (string, error) {
	rel := strings.TrimPrefix(originalPath, s.urlPrefix)
	rel = strings.TrimPrefix(rel, strings.TrimPrefix(s.urlPrefix, "/"))
	rel, ok := cleanRelative(rel)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrOriginalNotFound, originalPath)
	}
	return rel, nil
}

// GetResizedImageURL returns the preview URL for originalPath
// ({urlPrefix}images/foo.jpg) at width x height, generating the file if
// needed: {urlPrefix}resize/images/foo-180x275.webp (at storeURL when the
// store serves its files itself: the preview exists by then). Called while a page
// renders, without its context.
func (s *ResizeService) GetResizedImageURL(originalPath string, width, height int) (string, error) {
	if originalPath == "" || width <= 0 || height <= 0 {
		return "", fmt.Errorf("invalid parameters: originalPath=%s, width=%d, height=%d", originalPath, width, height)
	}
	ctx := context.Background()
	relativePath, err := s.relativePath(originalPath)
	if err != nil {
		return "", err
	}

	// The derivative is always .webp.
	name := path.Base(relativePath)
	resizedName := fmt.Sprintf("%s-%dx%d%s", strings.TrimSuffix(name, path.Ext(name)), width, height, ThumbnailExtension)
	resizedKey := path.Join("resize", path.Dir(relativePath), resizedName)

	if ok, err := filestore.Exists(ctx, s.store, resizedKey); err != nil {
		return "", fmt.Errorf("checking the preview %s: %w", resizedKey, err)
	} else if ok {
		return s.fileURL(resizedKey), nil
	}

	src, info, err := s.store.Open(ctx, relativePath)
	if errors.Is(err, filestore.ErrNotExist) {
		return "", fmt.Errorf("%w: %s", ErrOriginalNotFound, relativePath)
	}
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", relativePath, err)
	}
	defer func() { _ = src.Close() }()
	if s.failed.has(info) {
		return "", fmt.Errorf("%w: %s failed lately", ErrNotRenderable, relativePath)
	}

	style, err := NewImageStyle(
		"resize_temp",
		"Temporary resize",
		width,
		height,
		ResizeModeFill,
		imaging.Top,
		85,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create image style: %w", err)
	}

	processed, err := s.processor.Process(src, info.Name, style, imaging.Center, nil)
	if err != nil {
		if errors.Is(err, ErrNotRenderable) {
			s.failed.add(info)
		}
		return "", fmt.Errorf("rendering %s at %dx%d: %w", relativePath, width, height, err)
	}

	if _, err := s.store.Put(ctx, resizedKey, bytes.NewReader(processed.Data), filestore.PutOptions{ContentType: processed.ContentType}); err != nil {
		return "", fmt.Errorf("failed to write resized image: %w", err)
	}

	return s.fileURL(resizedKey), nil
}

// GetOriginalDimensions returns the original image's size (e.g. for
// data-pswp-width/data-pswp-height).
func (s *ResizeService) GetOriginalDimensions(originalPath string) (width, height int, err error) {
	relativePath, err := s.relativePath(originalPath)
	if err != nil {
		return 0, 0, err
	}
	file, _, err := s.store.Open(context.Background(), relativePath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open image: %w", err)
	}
	defer func() { _ = file.Close() }()

	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode image config: %w", err)
	}

	return cfg.Width, cfg.Height, nil
}
