package interfaces

import (
	"context"
	"mime/multipart"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
)

// fieldImageUploader is narrower than *files.FileUploadService -
// SaveFieldImageWithPolicy only ever calls this one method.
type fieldImageUploader interface {
	UploadImageField(ctx context.Context, file multipart.File, header *multipart.FileHeader, tokens files.FileDirectoryTokens, field entity.FieldInstanceDef) (string, int, int, int, string, error)
}

// SaveFieldImageWithPolicy saves an image upload for a specific FieldTypeImage
// field, validated against that field's upload settings (see
// files.FileUploadService.ValidateAndProcessImageField) rather than only the
// sitewide policy. Used for field values and field default images.
func SaveFieldImageWithPolicy(ctx context.Context, fileUploadService fieldImageUploader, fileRepo entity.FileRepository, file multipart.File, header *multipart.FileHeader, tokens files.FileDirectoryTokens, field entity.FieldInstanceDef) (int, error) {
	defer func() { _ = file.Close() }()
	uri, width, height, size, mime, err := fileUploadService.UploadImageField(ctx, file, header, tokens, field)
	if err != nil {
		return 0, err
	}
	fileRecord, err := fileRepo.Create(ctx, uri, mime, width, height, size, tokens.UserID)
	if err != nil {
		return 0, err
	}
	return fileRecord.ID, nil
}

// fieldFileUploader is narrower than *files.FileUploadService -
// SaveFieldFileWithPolicy only ever calls this one method.
type fieldFileUploader interface {
	UploadFileField(ctx context.Context, file multipart.File, header *multipart.FileHeader, tokens files.FileDirectoryTokens, field entity.FieldInstanceDef) (uri string, size int, mimeType, originalFilename string, err error)
}

// SaveFieldFileWithPolicy saves a generic (non-image) upload for a
// FieldTypeFile field, validated against the field's extension and size
// settings (see files.FileUploadService.UploadFileField). Unlike image
// uploads it keeps entity.File.OriginalFilename for the download link.
func SaveFieldFileWithPolicy(ctx context.Context, fileUploadService fieldFileUploader, fileRepo entity.FileRepository, file multipart.File, header *multipart.FileHeader, tokens files.FileDirectoryTokens, field entity.FieldInstanceDef) (int, error) {
	defer func() { _ = file.Close() }()
	uri, size, mime, originalFilename, err := fileUploadService.UploadFileField(ctx, file, header, tokens, field)
	if err != nil {
		return 0, err
	}
	fileRecord, err := fileRepo.Create(ctx, uri, mime, 0, 0, size, tokens.UserID)
	if err != nil {
		return 0, err
	}
	if err := fileRepo.SetOriginalFilename(ctx, fileRecord.ID, originalFilename); err != nil {
		return 0, err
	}
	return fileRecord.ID, nil
}

// FieldUploadTokens - token values for a field upload's directory
// (files.ResolveFileDirectory): now, the node's content type, the
// uploading user.
func FieldUploadTokens(contentType string, userID int) files.FileDirectoryTokens {
	return files.FileDirectoryTokens{Now: time.Now(), ContentType: contentType, UserID: userID}
}
