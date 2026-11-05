package files

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"path"
	"strings"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

// Limits on an uploaded image's size, checked from its header before it
// is decoded: a small file can declare a huge image that would take
// gigabytes of memory to decode. The image styles' own: no derivative of
// a larger one is made.
const (
	maxImageSide   = imagestyle.MaxSourceSide
	maxImagePixels = imagestyle.MaxSourcePixels
)

// uploadImageFormats - image formats the engine can accept: format name
// (as image.DecodeConfig reports it) → MIME type, stored extension and
// the name people know it by. Which of them a site accepts is its upload
// policy (UploadSettings.AllowedMimeTypes, per-field extensions).
var uploadImageFormats = map[string]struct{ mime, ext, label string }{
	"jpeg": {"image/jpeg", ".jpg", "JPEG"},
	"png":  {"image/png", ".png", "PNG"},
	"gif":  {"image/gif", ".gif", "GIF"},
	"webp": {"image/webp", ".webp", "WebP"},
}

var (
	// errNotAnImage - not an image in a format the engine decodes; callers
	// turn it into errImageFormats with the formats their policy allows.
	errNotAnImage    = errors.New("not an image")
	errImageTooLarge = i18n.Errorf("The image is too large: at most @side px per side and @mp megapixels.", "@side", maxImageSide, "@mp", maxImagePixels/1_000_000)
	// errBrokenImage - its header is an image's, the rest can't be decoded.
	errBrokenImage = i18n.Errorf("The image can't be read: the file is damaged.")
)

// errFileTooLarge - the upload is over maxBytes.
func errFileTooLarge(maxBytes int) error {
	return i18n.Errorf("The file is too large: the maximum is @size MB.", "@size", fmt.Sprintf("%.1f", float64(maxBytes)/1024/1024))
}

// errImageFormats - the upload isn't an image in one of the allowed
// formats (formats: as ImageFormatsLabel gives them).
func errImageFormats(formats string) error {
	return i18n.Errorf("Only images in these formats can be uploaded: @formats.", "@formats", formats)
}

// ImageFormatsLabel - the image formats of a MIME allow-list as people
// know them ("JPEG, PNG"); an empty list means every format the engine
// accepts.
func ImageFormatsLabel(mimes []string) string {
	var labels []string
	if len(mimes) == 0 {
		for _, f := range []string{"jpeg", "png", "gif", "webp"} {
			labels = append(labels, uploadImageFormats[f].label)
		}
		return strings.Join(labels, ", ")
	}
	for _, m := range mimes {
		label := m
		for _, f := range uploadImageFormats {
			if f.mime == m {
				label = f.label
			}
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, ", ")
}

// imageFormatsAllowedLabel - what an image upload may be under its
// policy: the extension allow-list when it has one, else the MIME list.
func imageFormatsAllowedLabel(extensions, mimes []string) string {
	if len(extensions) > 0 {
		up := make([]string, len(extensions))
		for i, e := range extensions {
			up[i] = strings.ToUpper(strings.TrimPrefix(e, "."))
		}
		return strings.Join(up, ", ")
	}
	return ImageFormatsLabel(mimes)
}

// inspectedImage - what an upload's content says it is.
type inspectedImage struct {
	format        string
	mime, ext     string
	width, height int
}

// inspectImage reads the image's header from its content (the uploader's
// file name and Content-Type are not trusted): the format must be one of
// uploadImageFormats and the size within the limits. The file is rewound.
func inspectImage(file io.ReadSeeker) (inspectedImage, error) {
	cfg, format, err := image.DecodeConfig(file)
	if _, serr := file.Seek(0, io.SeekStart); serr != nil {
		return inspectedImage{}, serr
	}
	if err != nil {
		return inspectedImage{}, errNotAnImage
	}
	f, ok := uploadImageFormats[format]
	if !ok {
		return inspectedImage{}, errNotAnImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxImageSide || cfg.Height > maxImageSide ||
		int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return inspectedImage{}, errImageTooLarge
	}
	return inspectedImage{format: format, mime: f.mime, ext: f.ext, width: cfg.Width, height: cfg.Height}, nil
}

// allowedByExtensions - whether an image format is in an extension
// allow-list ("jpg"/"jpeg" both mean JPEG).
func (img inspectedImage) allowedByExtensions(allowed []string) bool {
	names := []string{strings.TrimPrefix(img.ext, ".")}
	if img.format == "jpeg" {
		names = append(names, "jpeg")
	}
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimPrefix(a, "."))
		for _, n := range names {
			if a == n {
				return true
			}
		}
	}
	return false
}

// withExtension - key with its extension replaced by ext; when that file
// already exists, a free name with a _1, _2… suffix.
func withExtension(ctx context.Context, store filestore.Store, key, ext string) (string, error) {
	if strings.EqualFold(path.Ext(key), ext) {
		return key, nil
	}
	dir := path.Dir(key)
	if dir == "." {
		dir = ""
	}
	stem := strings.TrimSuffix(path.Base(key), path.Ext(key))
	name, err := pickUniqueFilename(ctx, store, dir, stem, ext)
	if err != nil {
		return "", fmt.Errorf("pick a file name: %w", err)
	}
	return filestore.Join(dir, name), nil
}
