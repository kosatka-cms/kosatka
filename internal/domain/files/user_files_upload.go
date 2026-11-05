package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/fsname"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// SaveUploadedImage writes an image into subdirRel (optional, relative to user root). Returns public URL path.
func (s *UserFilesService) SaveUploadedImage(
	ctx context.Context,
	userID int,
	subdirRel string,
	file multipart.File,
	header *multipart.FileHeader,
	formatID int,
) (string, error) {
	dir, err := s.uploadDir(ctx, userID, subdirRel)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	allowedExt := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !allowedExt[ext] {
		ext = ".jpg"
	}

	baseName := filepath.Base(header.Filename)
	extFromName := filepath.Ext(baseName)
	stem := baseName
	if extFromName != "" {
		stem = strings.TrimSuffix(baseName, extFromName)
	}
	stem = fsname.SanitizeImageStem(stem)

	var altExts []string
	if ext != ".jpg" {
		altExts = []string{".jpg"} // re-encoded to JPEG on resize
	}
	filename, err := pickUniqueFilename(ctx, s.store, dir, stem, ext, altExts...)
	if err != nil {
		return "", err
	}
	target := path.Join(dir, filename)

	result, err := s.uploads.ValidateAndProcessTextFormatImage(ctx, file, header, target, formatID)
	if err != nil {
		return "", err
	}
	// The saved file may differ from target (re-encoded to .jpg).
	if result != nil && result.Key != "" {
		target = result.Key
	}
	return s.PublicWebPath(target), nil
}

// SaveUploadedFile stores an upload in subdirRel: images through
// SaveUploadedImage (the text format's image policy); other files only
// when documents are allowed for the uploader and their extension is in
// the upload policy's UserFileExtensions. Returns the public URL path and
// the kind.
func (s *UserFilesService) SaveUploadedFile(
	ctx context.Context,
	userID int,
	subdirRel string,
	file multipart.File,
	header *multipart.FileHeader,
	formatID int,
	allowDocuments bool,
) (url, kind string, err error) {
	if isImageFileName(header.Filename) {
		url, err = s.SaveUploadedImage(ctx, userID, subdirRel, file, header, formatID)
		return url, UserFileKindImage, err
	}
	if !allowDocuments {
		return "", "", i18n.Errorf("You can upload images only (@formats).", "@formats", ImageFormatsLabel(s.ImageTypes(ctx)))
	}
	url, err = s.saveUploadedDocument(ctx, userID, subdirRel, file, header)
	return url, UserFileKindFile, err
}

// ImageTypes - the image MIME types the upload policy allows
// (UploadSettings.AllowedMimeTypes); nil if it can't be read.
func (s *UserFilesService) ImageTypes(ctx context.Context) []string {
	if s.uploads == nil || s.uploads.settingsRepo == nil {
		return nil
	}
	settings, err := s.uploads.settingsRepo.Get(ctx)
	if err != nil {
		return nil
	}
	return settings.AllowedMimeTypes
}

// ErrUserFileType - the upload's type isn't accepted for user files.
var ErrUserFileType = i18n.Errorf("This file type is not allowed.")

func (s *UserFilesService) saveUploadedDocument(ctx context.Context, userID int, subdirRel string, file multipart.File, header *multipart.FileHeader) (string, error) {
	if s.uploads == nil || s.uploads.settingsRepo == nil {
		return "", ErrUserFileType
	}
	settings, err := s.uploads.settingsRepo.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load upload settings: %w", err)
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(header.Filename), "."))
	if ext == "" || entity.ForbiddenUserFileExtensions[ext] || !isAllowedExtension(header.Filename, settings.UserFileExtensions) {
		if len(settings.UserFileExtensions) == 0 {
			return "", i18n.Errorf("Only images can be uploaded.")
		}
		return "", i18n.Errorf("Files of this type can't be uploaded. Allowed: images, @extensions.", "@extensions", strings.Join(settings.UserFileExtensions, ", "))
	}
	if settings.MaxFileSizeBytes > 0 && header.Size > int64(settings.MaxFileSizeBytes) {
		return "", errFileTooLarge(settings.MaxFileSizeBytes)
	}
	if err := checkDocumentContent(file); err != nil {
		return "", err
	}

	dir, err := s.uploadDir(ctx, userID, subdirRel)
	if err != nil {
		return "", err
	}
	filename, err := pickUniqueFilename(ctx, s.store, dir, fsname.FileStem(header.Filename, "file"), "."+ext)
	if err != nil {
		return "", err
	}
	target := path.Join(dir, filename)
	if _, err := s.store.Put(ctx, target, file, filestore.PutOptions{Exclusive: true}); err != nil {
		return "", err
	}
	return s.PublicWebPath(target), nil
}

// checkDocumentContent rejects content a browser could run as a page or
// a program, whatever the extension: HTML, SVG/XML, scripts, executables.
func checkDocumentContent(file multipart.File) error {
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return err
	}
	head = head[:n]
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if !DocumentAllowed("", head) {
		return ErrUserFileType
	}
	return nil
}

// DocumentAllowed - whether a file sent as a document (name - its name,
// "" - not to check; head - its first bytes, 512 suffice) may be kept:
// not of an extension never accepted (entity.ForbiddenUserFileExtensions:
// pages, scripts, programs), not content a browser could run as a page or
// a program whatever the name (HTML, SVG/XML, scripts, executables).
func DocumentAllowed(name string, head []byte) bool {
	if name != "" {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
		if ext == "" || entity.ForbiddenUserFileExtensions[ext] {
			return false
		}
	}
	head = head[:min(len(head), 512)]
	ct := http.DetectContentType(head)
	lower := bytes.ToLower(bytes.TrimSpace(head))
	switch {
	case strings.HasPrefix(ct, "text/html"), strings.HasPrefix(ct, "text/xml"),
		bytes.HasPrefix(head, []byte("MZ")), bytes.HasPrefix(head, []byte("\x7fELF")), bytes.HasPrefix(head, []byte("#!")),
		bytes.Contains(lower, []byte("<svg")), bytes.Contains(lower, []byte("<script")), bytes.Contains(lower, []byte("<?php")):
		return false
	}
	return true
}

// uploadDir - the key of the existing folder subdirRel under the user's
// root.
func (s *UserFilesService) uploadDir(ctx context.Context, userID int, subdirRel string) (string, error) {
	if err := s.ensureUserRoot(ctx, userID); err != nil {
		return "", err
	}
	dir, err := s.resolveDir(userID, subdirRel)
	if err != nil {
		return "", err
	}
	st, err := s.stat(ctx, dir)
	if err != nil {
		return "", err
	}
	if !st.IsDir {
		return "", ErrUserFilesNotDir
	}
	return dir, nil
}
