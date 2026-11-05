package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"mime/multipart"
	"path"
	"time"

	"github.com/disintegration/imaging"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// validateAndProcessAvatar validates and resizes image for avatar (max 300x300)
func (s *FileUploadService) validateAndProcessAvatar(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
	targetKey string,
) (*UploadResult, error) {
	settings, err := s.settingsRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load upload settings: %w", err)
	}
	accountSettings, err := s.accountSettingsRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load account settings: %w", err)
	}
	avatarPolicy := entity.ResolveAvatarUploadPolicy(*accountSettings, *settings)
	if avatarPolicy.MaxFileSizeBytes > 0 && header.Size > int64(avatarPolicy.MaxFileSizeBytes) {
		return nil, errFileTooLarge(avatarPolicy.MaxFileSizeBytes)
	}

	allowed := imageFormatsAllowedLabel(avatarPolicy.AllowedExtensions, settings.AllowedMimeTypes)
	inspected, err := inspectImage(file)
	if errors.Is(err, errNotAnImage) {
		return nil, errImageFormats(allowed)
	}
	if err != nil {
		return nil, err
	}
	// An avatar extension allow-list replaces the sitewide MIME check;
	// unset falls back to it (an empty list - any supported format, as for
	// other images). Both are checked against the detected format.
	if len(avatarPolicy.AllowedExtensions) > 0 {
		if !inspected.allowedByExtensions(avatarPolicy.AllowedExtensions) {
			return nil, errImageFormats(allowed)
		}
	} else if len(settings.AllowedMimeTypes) > 0 && !isAllowedMimeType(inspected.mime, settings.AllowedMimeTypes) {
		return nil, errImageFormats(allowed)
	}

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errBrokenImage, err)
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	needsResize := width > MaxAvatarDimension || height > MaxAvatarDimension

	// Always stored as JPEG.
	if needsResize {
		img = imaging.Fit(img, MaxAvatarDimension, MaxAvatarDimension, imaging.Lanczos)
	}
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(90)); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}
	size, err := s.store.Put(ctx, targetKey, &buf, filestore.PutOptions{ContentType: "image/jpeg"})
	if err != nil {
		return nil, fmt.Errorf("failed to save image: %w", err)
	}
	finalWidth, finalHeight := img.Bounds().Dx(), img.Bounds().Dy()

	// Always stored as JPEG, whatever was sent.
	return &UploadResult{
		Width:     finalWidth,
		Height:    finalHeight,
		SizeBytes: int(size),
		MimeType:  "image/jpeg",
	}, nil
}

// UploadUserAvatar saves user avatar to user_pictures/, resized to max 300x300
func (s *FileUploadService) UploadUserAvatar(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
	userID int,
) (string, error) {
	// Named by the user and the time; another in the same second gets _1.
	filename, err := pickUniqueFilename(ctx, s.store, UserAvatarBasePath, fmt.Sprintf("%d-%d", userID, time.Now().Unix()), ".jpg")
	if err != nil {
		return "", err
	}
	key := path.Join(UserAvatarBasePath, filename)

	result, err := s.validateAndProcessAvatar(ctx, file, header, key)
	if err != nil {
		return "", err
	}

	uri := "public://" + key
	s.logger.Info("user avatar saved",
		"user_id", userID,
		"uri", uri,
		"width", result.Width,
		"height", result.Height,
	)
	return uri, nil
}
