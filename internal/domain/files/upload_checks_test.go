package files

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// gifWithHTML - a valid GIF followed by an HTML payload (a polyglot).
func gifWithHTML(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White})
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	buf.WriteString("<html><script>alert(document.cookie)</script></html>")
	return buf.Bytes()
}

// pngHeader - a PNG that declares width×height but carries no pixel data.
func pngHeader(width, height uint32) []byte {
	var buf bytes.Buffer
	buf.Write([]byte("\x89PNG\r\n\x1a\n"))
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], width)
	binary.BigEndian.PutUint32(ihdr[4:], height)
	ihdr[8], ihdr[9] = 8, 2 // 8-bit RGB
	chunk := append([]byte("IHDR"), ihdr...)
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(ihdr)))
	buf.Write(chunk)
	_ = binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return buf.Bytes()
}

func newTestUploads(t *testing.T) (*FileUploadService, string) {
	t.Helper()
	base := t.TempDir()
	return NewFileUploadService(filestore.NewLocal(base), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, slog.New(slog.NewTextHandler(io.Discard, nil))), base
}

func upload(name, contentType string, content []byte) (multipart.File, *multipart.FileHeader) {
	h := &multipart.FileHeader{Filename: name, Size: int64(len(content)), Header: map[string][]string{"Content-Type": {contentType}}}
	return readSeekCloser{Reader: bytes.NewReader(content)}, h
}

// TestImageUploadTakesFormatFromContent - an image named and typed as a
// web page is stored with its real format's extension; the uploader's
// Content-Type is ignored.
func TestImageUploadTakesFormatFromContent(t *testing.T) {
	svc, base := newTestUploads(t)
	ctx := context.Background()
	for _, name := range []string{"x.html", "x.svg", "x.jpg"} {
		f, h := upload(name, "image/jpeg", gifWithHTML(t))
		uri, _, _, _, mimeType, err := svc.UploadImageField(ctx, f, h, FileDirectoryTokens{}, entity.FieldInstanceDef{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.HasSuffix(uri, ".gif") || mimeType != "image/gif" {
			t.Errorf("%s: stored as %q (%s)", name, uri, mimeType)
		}
		if _, err := os.Stat(filepath.Join(base, strings.TrimPrefix(uri, "public://"))); err != nil {
			t.Errorf("%s: file missing: %v", name, err)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(base, "content", "images", "*.html"))
	svgs, _ := filepath.Glob(filepath.Join(base, "content", "images", "*.svg"))
	if len(matches)+len(svgs) != 0 {
		t.Fatalf("page-like files stored: %v %v", matches, svgs)
	}

	// Not an image at all, whatever the header says.
	f, h := upload("photo.jpg", "image/jpeg", []byte("<html>not an image</html>"))
	_, _, _, _, _, err := svc.UploadImageField(ctx, f, h, FileDirectoryTokens{}, entity.FieldInstanceDef{})
	if err == nil || !strings.Contains(i18n.ErrorText(ctx, err), "Only images in these formats can be uploaded: JPEG, PNG, GIF, WebP.") {
		t.Fatalf("non-image: %v", err)
	}
}

// TestImageUploadRejectsHugeDimensions - an image declaring a huge size is
// rejected from its header, before it is decoded.
func TestImageUploadRejectsHugeDimensions(t *testing.T) {
	svc, _ := newTestUploads(t)
	for _, size := range [][2]uint32{{30000, 30000}, {8000, 8000}, {25000, 10}} {
		f, h := upload("big.png", "image/png", pngHeader(size[0], size[1]))
		_, _, _, _, _, err := svc.UploadImageField(context.Background(), f, h, FileDirectoryTokens{}, entity.FieldInstanceDef{})
		if !errors.Is(err, errImageTooLarge) {
			t.Errorf("%dx%d: %v", size[0], size[1], err)
		}
	}
	if _, err := inspectImage(bytes.NewReader(pngHeader(4000, 3000))); err != nil {
		t.Errorf("12 MP image rejected: %v", err)
	}
}

// TestFileFieldRejectsPagesAndScripts - a File field never stores web
// pages, scripts or programs, even if its allow-list is empty or the
// content hides behind an allowed extension.
func TestFileFieldRejectsPagesAndScripts(t *testing.T) {
	svc, _ := newTestUploads(t)
	field := entity.FieldInstanceDef{FieldType: entity.FieldTypeFile}
	for name, content := range map[string]string{
		"page.html":  "<p>hi</p>",
		"logo.svg":   "<svg/>",
		"report.pdf": "<!DOCTYPE html><script>x</script>",
		"notes.txt":  "#!/bin/sh\nrm -rf /",
	} {
		f, h := upload(name, "application/pdf", []byte(content))
		if _, _, _, _, err := svc.UploadFileField(context.Background(), f, h, FileDirectoryTokens{}, field); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	f, h := upload("report.pdf", "text/html", []byte("%PDF-1.7\n"))
	_, _, mimeType, _, err := svc.UploadFileField(context.Background(), f, h, FileDirectoryTokens{}, field)
	if err != nil || mimeType != "application/pdf" {
		t.Fatalf("pdf: %q %v", mimeType, err)
	}
}

// TestImageFormatsFromPolicy - the formats named to people come from the
// upload policy, not a fixed list.
func TestImageFormatsFromPolicy(t *testing.T) {
	if got := ImageFormatsLabel([]string{"image/png", "image/webp"}); got != "PNG, WebP" {
		t.Errorf("label: %q", got)
	}
	if got := ImageFormatsLabel(nil); got != "JPEG, PNG, GIF, WebP" {
		t.Errorf("all formats: %q", got)
	}
	if got := imageFormatsAllowedLabel([]string{"jpg", ".png"}, []string{"image/gif"}); got != "JPG, PNG" {
		t.Errorf("extensions first: %q", got)
	}
	// A format the engine decodes but the policy doesn't allow is refused
	// with the policy's formats.
	svc, _ := newTestUploads(t)
	var buf bytes.Buffer
	if err := gif.Encode(&buf, image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black}), nil); err != nil {
		t.Fatal(err)
	}
	field := entity.FieldInstanceDef{WidgetConfig: map[string]any{"allowed_extensions": []any{"png"}}}
	f, h := upload("pic.gif", "image/gif", buf.Bytes())
	_, _, _, _, _, err := svc.UploadImageField(context.Background(), f, h, FileDirectoryTokens{}, field)
	if err == nil || !strings.Contains(i18n.ErrorText(context.Background(), err), ": PNG.") {
		t.Fatalf("gif in a png-only field: %v", err)
	}
}
