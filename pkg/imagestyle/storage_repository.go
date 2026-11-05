package imagestyle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// StorageRepository keeps the derivatives beside the originals in the
// public files' store: styles/<style>/<path> and resize/<path>.
type StorageRepository struct {
	store filestore.Store
}

// NewStorageRepository - a StorageRepository keeping the derivatives in
// store.
func NewStorageRepository(store filestore.Store) *StorageRepository {
	return &StorageRepository{store: store}
}

// cleanRelative - relativePath without its leading slash, if it is a
// path inside the files (no "..").
func cleanRelative(relativePath string) (string, bool) {
	rel := strings.TrimPrefix(relativePath, "/")
	return rel, rel != "" && fs.ValidPath(rel)
}

// OpenOriginal opens the original for relativePath (images/foo.jpg, or a
// derivative's path such as …/foo.webp); ErrOriginalNotFound if there is
// none.
func (r *StorageRepository) OpenOriginal(ctx context.Context, relativePath string) (filestore.File, filestore.Info, error) {
	rel, ok := cleanRelative(relativePath)
	if !ok {
		return nil, filestore.Info{}, fmt.Errorf("%w: %s", ErrOriginalNotFound, relativePath)
	}
	for _, candidate := range extensionCandidates(rel) {
		f, info, err := r.store.Open(ctx, candidate)
		if err == nil {
			return f, info, nil
		}
		if !errors.Is(err, filestore.ErrNotExist) && !errors.Is(err, filestore.ErrInvalidKey) {
			return nil, filestore.Info{}, fmt.Errorf("opening the original %s: %w", candidate, err)
		}
	}
	return nil, filestore.Info{}, fmt.Errorf("%w: %s", ErrOriginalNotFound, relativePath)
}

// OpenExact opens the file at key itself (no extension candidates);
// ErrOriginalNotFound if there is none.
func (r *StorageRepository) OpenExact(ctx context.Context, key string) (filestore.File, filestore.Info, error) {
	rel, ok := cleanRelative(key)
	if !ok {
		return nil, filestore.Info{}, fmt.Errorf("%w: %s", ErrOriginalNotFound, key)
	}
	f, info, err := r.store.Open(ctx, rel)
	if errors.Is(err, filestore.ErrNotExist) {
		return nil, filestore.Info{}, fmt.Errorf("%w: %s", ErrOriginalNotFound, key)
	}
	return f, info, err
}

// extensionCandidates - the paths a source is tried at: relativePath and,
// for a derivative's extension (.webp, or .jpg for a JPEG style), the same
// name with the sources' extensions.
func extensionCandidates(relativePath string) []string {
	ext := strings.ToLower(path.Ext(relativePath))
	if ext != ".webp" && ext != ".jpg" {
		return []string{relativePath}
	}
	base := strings.TrimSuffix(relativePath, path.Ext(relativePath))
	seen := map[string]bool{relativePath: true}
	var out []string
	out = append(out, relativePath)
	for _, tryExt := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp"} {
		cand := base + tryExt
		if !seen[cand] {
			seen[cand] = true
			out = append(out, cand)
		}
	}
	return out
}

// cachedKey - the derivative's key: styles/<style>/<path>.
func cachedKey(styleName, relativePath string) (string, error) {
	rel, ok := cleanRelative(relativePath)
	if !ok || styleName == "" || strings.ContainsAny(styleName, "/.") {
		return "", fmt.Errorf("%w: styles/%s/%s", filestore.ErrInvalidKey, styleName, relativePath)
	}
	return "styles/" + styleName + "/" + rel, nil
}

// OpenCached opens the style's derivative of relativePath;
// filestore.ErrNotExist if it hasn't been made.
func (r *StorageRepository) OpenCached(ctx context.Context, styleName, relativePath string) (filestore.File, filestore.Info, error) {
	key, err := cachedKey(styleName, relativePath)
	if err != nil {
		return nil, filestore.Info{}, err
	}
	return r.store.Open(ctx, key)
}

// SaveToCache stores the style's derivative of relativePath.
func (r *StorageRepository) SaveToCache(ctx context.Context, styleName, relativePath string, img ProcessedImage) error {
	key, err := cachedKey(styleName, relativePath)
	if err != nil {
		return err
	}
	_, err = r.store.Put(ctx, key, bytes.NewReader(img.Data), filestore.PutOptions{ContentType: img.ContentType})
	return err
}

// ClearStyleCache removes a style's derivatives (styles/<style>).
func (r *StorageRepository) ClearStyleCache(ctx context.Context, styleName string) error {
	if styleName == "" || strings.ContainsAny(styleName, "/.") {
		return fmt.Errorf("%w: style %q", filestore.ErrInvalidKey, styleName)
	}
	return r.store.DeleteDir(ctx, "styles/"+styleName)
}

// ClearAllCache removes all image-style derivatives (styles/).
func (r *StorageRepository) ClearAllCache(ctx context.Context) error {
	return r.store.DeleteDir(ctx, "styles")
}

// ClearResizeDir removes the generated WxH previews under resize/.
func (r *StorageRepository) ClearResizeDir(ctx context.Context) error {
	return r.store.DeleteDir(ctx, "resize")
}

// ClearAllGeneratedImages clears the resize previews and all style
// derivatives.
func (r *StorageRepository) ClearAllGeneratedImages(ctx context.Context) error {
	if err := r.ClearResizeDir(ctx); err != nil {
		return err
	}
	return r.ClearAllCache(ctx)
}

// DerivativeCaches - clearing the derivatives: a style's (its settings
// changed), or all of them.
type DerivativeCaches interface {
	ClearStyleCache(ctx context.Context, styleName string) error
	ClearAllGeneratedImages(ctx context.Context) error
}

// Repositories clears the derivatives of every storage repository it
// holds (the public files', the private files').
type Repositories []ImageStorageRepository

// ClearStyleCache removes the derivatives of the style styleName from
// every repository.
func (rs Repositories) ClearStyleCache(ctx context.Context, styleName string) error {
	var errs []error
	for _, r := range rs {
		errs = append(errs, r.ClearStyleCache(ctx, styleName))
	}
	return errors.Join(errs...)
}

// ClearAllGeneratedImages removes every derivative from every
// repository.
func (rs Repositories) ClearAllGeneratedImages(ctx context.Context) error {
	var errs []error
	for _, r := range rs {
		errs = append(errs, r.ClearAllGeneratedImages(ctx))
	}
	return errors.Join(errs...)
}
