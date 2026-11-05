package files

import (
	"context"
	"fmt"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// ValidateAndProcessImageField is ValidateAndProcess with the image
// field's own upload policy (entity.ResolveFieldUploadPolicy). A setting
// the field defines is used as-is, not clamped to the sitewide policy;
// unset settings fall back to the sitewide defaults.
func (s *FileUploadService) ValidateAndProcessImageField(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
	targetKey string,
	field entity.FieldInstanceDef,
) (*UploadResult, error) {
	settings, err := s.settingsRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load upload settings: %w", err)
	}
	policy := entity.ResolveFieldUploadPolicy(field, *settings)
	// A field extension allow-list replaces the sitewide MIME check;
	// unset falls back to it.
	allowedMimeTypes := settings.AllowedMimeTypes
	if len(policy.AllowedExtensions) > 0 {
		allowedMimeTypes = nil
	}
	store, _, err := s.storeFor(field)
	if err != nil {
		return nil, err
	}
	return s.validateAndProcessCore(ctx, store, file, header, targetKey, allowedMimeTypes, policy)
}

// UploadImageField saves an image for a FieldTypeImage field, validated
// with the field's policy (ValidateAndProcessImageField), into the field's
// directory (FieldInstanceDef.FileDirectory with tokens resolved; empty =
// DynamicContentImageBasePath) under the uploader's file name - of the
// private files' store when the field keeps its files private.
func (s *FileUploadService) UploadImageField(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
	tokens FileDirectoryTokens,
	field entity.FieldInstanceDef,
) (string, int, int, int, string, error) {
	dir, err := ResolveFileDirectory(field.FileDirectory(), DynamicContentImageBasePath, tokens)
	if err != nil {
		return "", 0, 0, 0, "", err
	}
	store, scheme, err := s.storeFor(field)
	if err != nil {
		return "", 0, 0, 0, "", err
	}
	key, err := s.uploadTarget(ctx, store, dir, header.Filename, "image", true)
	if err != nil {
		return "", 0, 0, 0, "", err
	}

	result, err := s.ValidateAndProcessImageField(ctx, file, header, key, field)
	if err != nil {
		return "", 0, 0, 0, "", err
	}

	uri := uriFromUpload(scheme, key, result)

	s.logger.Info("image field uploaded successfully",
		"field_id", field.ID,
		"uri", uri,
		"width", result.Width,
		"height", result.Height,
		"size", result.SizeBytes,
	)

	return uri, result.Width, result.Height, result.SizeBytes, result.MimeType, nil
}

// UploadFileField saves a non-image file for an entity.FieldTypeFile field
// into the field's directory (FieldInstanceDef.FileDirectory; empty =
// DynamicContentFileBasePath) - of the private files' store when the
// field keeps its files private. Only extension and size are validated
// against the field's policy; the file is saved byte-for-byte. Returns the
// uploader's filename as originalFilename.
func (s *FileUploadService) UploadFileField(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
	tokens FileDirectoryTokens,
	field entity.FieldInstanceDef,
) (uri string, size int, mimeType, originalFilename string, err error) {
	settings, err := s.settingsRepo.Get(ctx)
	if err != nil {
		return "", 0, "", "", fmt.Errorf("failed to load upload settings: %w", err)
	}
	policy := entity.ResolveFieldUploadPolicy(field, *settings)

	if policy.MaxFileSizeBytes > 0 && header.Size > int64(policy.MaxFileSizeBytes) {
		return "", 0, "", "", errFileTooLarge(policy.MaxFileSizeBytes)
	}
	if len(policy.AllowedExtensions) > 0 && !isAllowedExtension(header.Filename, policy.AllowedExtensions) {
		return "", 0, "", "", i18n.Errorf("This file type is not allowed. Allowed: @extensions.", "@extensions", strings.Join(policy.AllowedExtensions, ", "))
	}

	// Pages, scripts and programs are never stored, whatever the field
	// allows: by extension and by what the content starts with.
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(header.Filename), "."))
	if ext == "" || entity.ForbiddenUserFileExtensions[ext] {
		return "", 0, "", "", ErrUserFileType
	}
	if err := checkDocumentContent(file); err != nil {
		return "", 0, "", "", err
	}
	// The stored MIME type follows the extension, not the uploader's header.
	contentType := mime.TypeByExtension("." + ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	dir, err := ResolveFileDirectory(field.FileDirectory(), DynamicContentFileBasePath, tokens)
	if err != nil {
		return "", 0, "", "", err
	}
	// A field keeping its files private: in the private files' store.
	store, scheme, err := s.storeFor(field)
	if err != nil {
		return "", 0, "", "", err
	}
	key, err := s.uploadTarget(ctx, store, dir, header.Filename, "file", false)
	if err != nil {
		return "", 0, "", "", err
	}

	written, err := store.Put(ctx, key, file, filestore.PutOptions{ContentType: contentType})
	if err != nil {
		return "", 0, "", "", fmt.Errorf("failed to save file: %w", err)
	}

	uri = scheme + key

	s.logger.Info("file field uploaded successfully",
		"field_id", field.ID,
		"uri", uri,
		"size", written,
		"original_filename", header.Filename,
	)

	return uri, int(written), contentType, header.Filename, nil
}
