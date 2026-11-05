package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io/fs"

	// Decoders image.Decode needs for these formats.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"mime/multipart"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/fsname"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
	_ "golang.org/x/image/webp" // the WebP decoder for image.Decode
)

const (
	// DynamicContentImageBasePath - default directory for content image
	// uploads (image fields without their own FieldInstanceDef.FileDirectory,
	// and node main images).
	DynamicContentImageBasePath = "content/images"
	// DynamicContentFileBasePath - default directory for FieldTypeFile
	// uploads, kept apart from content images.
	DynamicContentFileBasePath = "content/files"
	// UserAvatarBasePath - the directory of the users' uploaded pictures
	// (UploadUserAvatar).
	UserAvatarBasePath = "user_pictures"
	// MaxAvatarDimension - a user's picture's largest width and height, in
	// pixels: a larger one is scaled down.
	MaxAvatarDimension = 300
	// SiteLogoBasePath - the site logo (entity.SiteInformation.Logo).
	// Uploaded via Upload, which re-encodes only when resizing, so a
	// transparent PNG logo keeps its format (the avatar path always
	// re-encodes to JPEG).
	SiteLogoBasePath = "site/logo"
)

// isAllowedMimeType reports whether contentType is in the admin-configured
// allow-list (entity.UploadSettings.AllowedMimeTypes).
func isAllowedMimeType(contentType string, allowed []string) bool {
	for _, a := range allowed {
		if a == contentType {
			return true
		}
	}
	return false
}

// UploadResult - a stored upload: an image's width and height, the size
// in bytes, the MIME type and the key it was stored at.
type UploadResult struct {
	Width     int
	Height    int
	SizeBytes int
	MimeType  string
	Key       string // the stored file's key (may become .jpg after a resize)
}

// FileUploadService checks uploads and keeps them in the public files'
// store (keys relative to it, public://<key> URIs) - a private file
// field's in the private files' one (private://<key>).
type FileUploadService struct {
	store               filestore.Store
	private             filestore.Store
	settingsRepo        entity.UploadSettingsRepository
	accountSettingsRepo entity.AccountSettingsRepository
	textFormatRepo      entity.TextFormatRepository
	logger              *slog.Logger
}

// NewFileUploadService - the uploads' service on store (the public
// files'): the sitewide upload settings, the account settings' (users'
// pictures) and the text formats' (rich-text editor uploads) policies.
func NewFileUploadService(
	store filestore.Store,
	settingsRepo entity.UploadSettingsRepository,
	accountSettingsRepo entity.AccountSettingsRepository,
	textFormatRepo entity.TextFormatRepository,
	logger *slog.Logger,
) *FileUploadService {
	return &FileUploadService{
		store:               store,
		settingsRepo:        settingsRepo,
		accountSettingsRepo: accountSettingsRepo,
		textFormatRepo:      textFormatRepo,
		logger:              logger,
	}
}

// SetPrivateStore sets the private files' store (wiring): the uploads of
// file fields keeping their files private (FieldInstanceDef.PrivateFiles).
func (s *FileUploadService) SetPrivateStore(store filestore.Store) { s.private = store }

// ValidateAndProcess validates file format/size against the sitewide
// policy and resizes if needed. See ValidateAndProcessImageField for the
// per-field variant.
func (s *FileUploadService) ValidateAndProcess(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
	targetKey string,
) (*UploadResult, error) {
	settings, err := s.settingsRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load upload settings: %w", err)
	}
	policy := entity.FieldUploadPolicy{
		MaxFileSizeBytes: settings.MaxFileSizeBytes,
		MaxWidth:         settings.MaxImageDimension,
		MaxHeight:        settings.MaxImageDimension,
	}
	return s.validateAndProcessCore(ctx, s.store, file, header, targetKey, settings.AllowedMimeTypes, policy)
}

// validateAndProcessCore is the shared body of ValidateAndProcess and
// ValidateAndProcessImageField. allowedMimeTypes is checked only when
// non-empty; a policy.AllowedExtensions list replaces it. Stores the
// image in store as targetKey (its extension following the format).
func (s *FileUploadService) validateAndProcessCore(
	ctx context.Context,
	store filestore.Store,
	file multipart.File,
	header *multipart.FileHeader,
	targetKey string,
	allowedMimeTypes []string,
	policy entity.FieldUploadPolicy,
) (*UploadResult, error) {
	// 1. Validate file size
	if policy.MaxFileSizeBytes > 0 && header.Size > int64(policy.MaxFileSizeBytes) {
		return nil, errFileTooLarge(policy.MaxFileSizeBytes)
	}

	// 2. What the content is (not the uploader's file name or
	// Content-Type): a supported image format within the size limits.
	allowed := imageFormatsAllowedLabel(policy.AllowedExtensions, allowedMimeTypes)
	inspected, err := inspectImage(file)
	if errors.Is(err, errNotAnImage) {
		return nil, errImageFormats(allowed)
	}
	if err != nil {
		return nil, err
	}

	// 3. The policy's extension allow-list or the sitewide MIME types,
	// checked against the detected format.
	if (len(policy.AllowedExtensions) > 0 && !inspected.allowedByExtensions(policy.AllowedExtensions)) ||
		(len(allowedMimeTypes) > 0 && !isAllowedMimeType(inspected.mime, allowedMimeTypes)) {
		return nil, errImageFormats(allowed)
	}
	contentType := inspected.mime

	// 4. The stored extension follows the format, never the uploader's
	// name (an image named x.html must not be served as a page).
	if targetKey, err = withExtension(ctx, store, targetKey, inspected.ext); err != nil {
		return nil, err
	}

	// 5. Decode image
	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errBrokenImage, err)
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to reset file pointer: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	s.logger.Info("image uploaded",
		"format", format,
		"width", width,
		"height", height,
		"size", header.Size,
	)

	// 6. Reject images smaller than the field's minimum resolution, if
	// set - no upscaling.
	if (policy.MinWidth > 0 && width < policy.MinWidth) || (policy.MinHeight > 0 && height < policy.MinHeight) {
		return nil, i18n.Errorf("The image is too small: @width × @height px, at least @min_width × @min_height px are needed.",
			"@width", width, "@height", height, "@min_width", policy.MinWidth, "@min_height", policy.MinHeight)
	}

	// 7. Check if resizing is needed
	needsResize := (policy.MaxWidth > 0 && width > policy.MaxWidth) || (policy.MaxHeight > 0 && height > policy.MaxHeight)

	var finalWidth, finalHeight int
	var size int64
	savedKey := targetKey

	if needsResize {
		s.logger.Info("resizing image",
			"original_width", width,
			"original_height", height,
			"max_width", policy.MaxWidth,
			"max_height", policy.MaxHeight,
		)

		resized := imaging.Fit(img, policy.MaxWidth, policy.MaxHeight, imaging.Lanczos)

		// imaging.Save picks the codec by extension and can't write .webp, so encode JPEG explicitly.
		savedKey = ensureJPEGExtension(targetKey)
		var buf bytes.Buffer
		if err := imaging.Encode(&buf, resized, imaging.JPEG, imaging.JPEGQuality(90)); err != nil {
			return nil, fmt.Errorf("failed to encode resized image: %w", err)
		}
		if size, err = store.Put(ctx, savedKey, &buf, filestore.PutOptions{ContentType: "image/jpeg"}); err != nil {
			return nil, fmt.Errorf("failed to save resized image: %w", err)
		}

		finalWidth = resized.Bounds().Dx()
		finalHeight = resized.Bounds().Dy()
	} else {
		if size, err = store.Put(ctx, targetKey, file, filestore.PutOptions{ContentType: contentType}); err != nil {
			return nil, fmt.Errorf("failed to save image: %w", err)
		}

		finalWidth = width
		finalHeight = height
	}

	// Determine final MIME type (always JPEG if resized)
	finalMimeType := contentType
	if needsResize {
		finalMimeType = "image/jpeg"
	}

	return &UploadResult{
		Width:     finalWidth,
		Height:    finalHeight,
		SizeBytes: int(size),
		MimeType:  finalMimeType,
		Key:       savedKey,
	}, nil
}

// uploadTarget picks the key an upload goes to: in dir (a key), the
// file name built from the uploader's one (fsname.FileStem, lower-case
// extension; ".jpg" when an image has none) and made unique with _1,
// _2…. For an image the ".jpg" variant must be free too -
// ValidateAndProcess re-encodes an oversized image to JPEG and changes
// the extension.
func (s *FileUploadService) uploadTarget(ctx context.Context, store filestore.Store, dir, originalName, fallbackStem string, image bool) (string, error) {
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" && image {
		ext = ".jpg"
	}
	var alt []string
	if image && ext != ".jpg" {
		alt = []string{".jpg"}
	}
	dir = filepath.ToSlash(dir)
	name, err := pickUniqueFilename(ctx, store, dir, fsname.FileStem(originalName, fallbackStem), ext, alt...)
	if err != nil {
		return "", err
	}
	return path.Join(dir, name), nil
}

// pickUniqueFilename picks stem+ext, or stem_1.ext, stem_2.ext, … - the
// first candidate for which neither stem+ext nor stem+altExt exists in
// the folder dir of store.
func pickUniqueFilename(ctx context.Context, store filestore.Store, dir, stem, ext string, altExts ...string) (string, error) {
	for i := 0; i < 100000; i++ {
		base := stem
		if i > 0 {
			base = fmt.Sprintf("%s_%d", stem, i)
		}
		free := true
		for _, e := range append([]string{ext}, altExts...) {
			taken, err := filestore.Exists(ctx, store, filestore.Join(dir, base+e))
			if err != nil {
				return "", err
			}
			if taken {
				free = false
				break
			}
		}
		if free {
			return base + ext, nil
		}
	}
	return "", fmt.Errorf("could not pick unique filename under %s", dir)
}

// isAllowedExtension reports whether filename's extension (case-
// insensitive, no leading dot) is in allowed.
func isAllowedExtension(filename string, allowed []string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	for _, a := range allowed {
		if strings.ToLower(a) == ext {
			return true
		}
	}
	return false
}

// ensureJPEGExtension returns key with .jpg extension for resized uploads (imaging cannot encode WebP).
func ensureJPEGExtension(key string) string {
	ext := strings.ToLower(path.Ext(key))
	if ext == ".jpg" || ext == ".jpeg" {
		return key
	}
	return strings.TrimSuffix(key, ext) + ".jpg"
}

// publicURIFromUpload - the public:// URI of the stored upload: its key,
// else the key it was meant for.
func publicURIFromUpload(key string, result *UploadResult) string {
	return uriFromUpload(entity.PublicScheme, key, result)
}

// uriFromUpload - the stored upload's URI in scheme (public://,
// private://): its key, else the key it was meant for.
func uriFromUpload(scheme, key string, result *UploadResult) string {
	if result.Key != "" {
		key = result.Key
	}
	return scheme + key
}

// storeFor - the store and URI scheme of field's uploads: the private
// files' for a field keeping its files private.
func (s *FileUploadService) storeFor(field entity.FieldInstanceDef) (filestore.Store, string, error) {
	if !field.PrivateFiles() {
		return s.store, entity.PublicScheme, nil
	}
	if s.private == nil {
		return nil, "", errors.New("no private files' store for a field keeping its files private")
	}
	return s.private, entity.PrivateScheme, nil
}

// Upload saves a generic image file and returns entity.File compatible data
func (s *FileUploadService) Upload(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
	basePath string,
) (string, int, int, int, string, error) {
	// Named by the time; a second upload in the same second gets _1.
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	dir := filepath.ToSlash(basePath)
	filename, err := pickUniqueFilename(ctx, s.store, dir, strconv.FormatInt(time.Now().Unix(), 10), ext)
	if err != nil {
		return "", 0, 0, 0, "", err
	}
	key := path.Join(dir, filename)

	result, err := s.ValidateAndProcess(ctx, file, header, key)
	if err != nil {
		return "", 0, 0, 0, "", err
	}

	uri := publicURIFromUpload(key, result)

	s.logger.Info("image uploaded successfully",
		"uri", uri,
		"width", result.Width,
		"height", result.Height,
		"size", result.SizeBytes,
	)

	return uri, result.Width, result.Height, result.SizeBytes, result.MimeType, nil
}

// ValidateAndProcessTextFormatImage validates a rich-text editor upload
// with the text format's policy (entity.ResolveTextFormatUploadPolicy).
// formatID <= 0 or an unknown ID falls back to the sitewide policy.
func (s *FileUploadService) ValidateAndProcessTextFormatImage(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
	targetKey string,
	formatID int,
) (*UploadResult, error) {
	settings, err := s.settingsRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load upload settings: %w", err)
	}
	var def entity.TextFormatDef
	if formatID > 0 {
		if f, err := s.textFormatRepo.TextFormatByID(ctx, formatID); err == nil && f != nil {
			def = *f
		}
	}
	policy := entity.ResolveTextFormatUploadPolicy(def, *settings)
	allowedMimeTypes := settings.AllowedMimeTypes
	if len(policy.AllowedExtensions) > 0 {
		allowedMimeTypes = nil
	}
	return s.validateAndProcessCore(ctx, s.store, file, header, targetKey, allowedMimeTypes, policy)
}

// ImageTypes - the image MIME types the upload policy allows
// (UploadSettings.AllowedMimeTypes; empty - every format the engine
// accepts, ImageFormatsLabel).
func (s *FileUploadService) ImageTypes(ctx context.Context) ([]string, error) {
	settings, err := s.settingsRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the upload settings: %w", err)
	}
	return settings.AllowedMimeTypes, nil
}

// UploadImage saves an image to dir (relative to the public files, e.g. a
// module's "galleries/12") under a unique name, checked by policy: its
// extensions replace the sitewide MIME check (none - the sitewide one), a
// zero size limit and no maximum width or height are the sitewide ones
// (an image above the maximum is scaled down). Returns its public:// URI.
func (s *FileUploadService) UploadImage(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
	dir string,
	policy entity.FieldUploadPolicy,
) (string, *UploadResult, error) {
	settings, err := s.settingsRepo.Get(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load upload settings: %w", err)
	}
	if policy.MaxFileSizeBytes == 0 {
		policy.MaxFileSizeBytes = settings.MaxFileSizeBytes
	}
	// No size of its own: the sitewide maximum, above which it is scaled
	// down.
	if policy.MaxWidth == 0 && policy.MaxHeight == 0 {
		policy.MaxWidth, policy.MaxHeight = settings.MaxImageDimension, settings.MaxImageDimension
	}
	allowedMimeTypes := settings.AllowedMimeTypes
	if len(policy.AllowedExtensions) > 0 {
		allowedMimeTypes = nil
	}

	// Must be unique per file: Unix seconds collide when several uploads
	// finish in the same second and would overwrite each other.
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	key := path.Join(filepath.ToSlash(dir), fmt.Sprintf("%d%s", time.Now().UnixNano(), ext))
	result, err := s.validateAndProcessCore(ctx, s.store, file, header, key, allowedMimeTypes, policy)
	if err != nil {
		return "", nil, err
	}
	uri := publicURIFromUpload(key, result)
	s.logger.Info("image uploaded",
		"uri", uri,
		"width", result.Width,
		"height", result.Height,
		"size", result.SizeBytes,
	)
	return uri, result, nil
}

// RemoveStoredFile deletes a stored public:// or private:// file together
// with its image-style derivatives in its store (styles/<style>/<rel>).
// Anything that isn't such a URI of a key
// inside its store is skipped silently; a file already missing isn't an
// error either.
func (s *FileUploadService) RemoveStoredFile(ctx context.Context, uri string) error {
	return s.removeStored(ctx, uri, true)
}

// RemoveDerivatives deletes only the image-style derivatives of a stored
// file (RemoveStoredFile without the file): they are made again, e.g.
// at its new focal point.
func (s *FileUploadService) RemoveDerivatives(ctx context.Context, uri string) error {
	return s.removeStored(ctx, uri, false)
}

// removeStored - RemoveStoredFile; withFile false - the derivatives only.
func (s *FileUploadService) removeStored(ctx context.Context, uri string, withFile bool) error {
	if key, private := strings.CutPrefix(uri, entity.PrivateScheme); private {
		if s.private == nil || !fs.ValidPath(key) || key == "." {
			return nil
		}
		// A private image's derivatives keep the original's whole name
		// (imagestyle.StyleService.GetStyledOriginal).
		return removeWithDerivatives(ctx, s.private, key, withFile, []string{
			key + imagestyle.FormatWebP.Extension(), key + imagestyle.FormatJPEG.Extension(),
		})
	}
	rel, ok := strings.CutPrefix(uri, entity.PublicScheme)
	if !ok || !fs.ValidPath(rel) || rel == "." {
		return nil
	}
	// Derivatives are .webp, or .jpg for a JPEG style
	// (imageurl.DerivativeRelPaths); the original-extension path is
	// also tried for older derivatives.
	derivatives := []string{rel}
	for _, d := range imageurl.DerivativeRelPaths(rel) {
		if d != rel {
			derivatives = append(derivatives, d)
		}
	}
	return removeWithDerivatives(ctx, s.store, rel, withFile, derivatives)
}

// removeWithDerivatives deletes rel (withFile) and its image-style
// derivatives (the paths derivatives under each style's folder, and under
// its 2x folder, imagestyle.DoublePath) from store.
func removeWithDerivatives(ctx context.Context, store filestore.Store, rel string, withFile bool, derivatives []string) error {
	var targets []string
	if withFile {
		targets = append(targets, rel)
	}
	var firstErr error
	list := func(dir string) []filestore.Info {
		entries, err := store.List(ctx, dir)
		if err != nil && !errors.Is(err, filestore.ErrNotExist) && firstErr == nil {
			firstErr = fmt.Errorf("listing the image styles' derivatives: %w", err)
		}
		return entries
	}
	for _, d := range list("styles") {
		if !d.IsDir {
			continue
		}
		for _, dir := range []string{d.Key, path.Join(d.Key, imagestyle.DoubleDir)} {
			for _, dr := range derivatives {
				targets = append(targets, path.Join(dir, dr))
			}
		}
	}
	for _, key := range targets {
		if err := store.Delete(ctx, key); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// RemoveUserDir deletes a user's own rich-text upload folder
// (UserDirName, see UserFilesService) with everything in it.
func (s *FileUploadService) RemoveUserDir(ctx context.Context, userID int) error {
	if userID <= 0 {
		return nil
	}
	return s.store.DeleteDir(ctx, UserDirName(userID))
}
