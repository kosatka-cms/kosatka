package files

import (
	"bytes"
	"context"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

func TestUploadImageFieldNamesAndDirectory(t *testing.T) {
	svc, base := newTestUploadService(t)
	tokens := FileDirectoryTokens{Now: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC), ContentType: "article", UserID: 7}
	field := entity.FieldInstanceDef{ID: 1, FieldType: entity.FieldTypeImage}

	for _, want := range []string{"public://content/images/foto-poster.png", "public://content/images/foto-poster_1.png"} {
		// Cyrillic on purpose: checks transliteration of the file name.
		f, h := pngUpload(t, "Фото Poster.PNG")
		uri, _, _, _, _, err := svc.UploadImageField(context.Background(), f, h, tokens, field)
		if err != nil || uri != want {
			t.Fatalf("uri = %q, %v; want %q", uri, err, want)
		}
	}

	field.WidgetConfig = map[string]any{"file_directory": "[node:type]/[date:custom:Y]-[date:custom:m]"}
	f, h := pngUpload(t, "cover.png")
	uri, _, _, _, _, err := svc.UploadImageField(context.Background(), f, h, tokens, field)
	if err != nil || uri != "public://article/2026-10/cover.png" {
		t.Fatalf("configured directory: uri = %q, %v", uri, err)
	}
	if _, err := os.Stat(filepath.Join(base, "article", "2026-10", "cover.png")); err != nil {
		t.Fatal(err)
	}

	field.WidgetConfig = map[string]any{"file_directory": "styles/x"}
	f, h = pngUpload(t, "cover.png")
	if _, _, _, _, _, err := svc.UploadImageField(context.Background(), f, h, tokens, field); err == nil {
		t.Fatal("reserved directory must be rejected")
	}
}

func TestUploadFileFieldKeepsOriginalName(t *testing.T) {
	svc, _ := newTestUploadService(t)
	field := entity.FieldInstanceDef{ID: 2, FieldType: entity.FieldTypeFile, WidgetConfig: map[string]any{"allowed_extensions": []any{"pdf"}}}
	// Cyrillic on purpose: the stored name is transliterated, the original kept as is.
	body := readSeekCloser{Reader: bytes.NewReader([]byte("%PDF-1.4"))}
	uri, _, _, orig, err := svc.UploadFileField(context.Background(), body, &multipart.FileHeader{Filename: "Отчёт 2026.PDF", Size: 8}, FileDirectoryTokens{Now: time.Now()}, field)
	if err != nil || uri != "public://content/files/otchyot-2026.pdf" || orig != "Отчёт 2026.PDF" {
		t.Fatalf("uri=%q orig=%q err=%v", uri, orig, err)
	}
}

// TestUploadFileFieldPrivate - a field keeping its files private stores
// them in the private files' store under a private:// URI; removing the
// URI removes the file there.
func TestUploadFileFieldPrivate(t *testing.T) {
	svc, base := newTestUploadService(t)
	field := entity.FieldInstanceDef{ID: 4, WidgetConfig: map[string]any{"private_files": true}}
	f, h := uploadBody("Contract.pdf", []byte("%PDF-1.7\n"))
	if _, _, _, _, err := svc.UploadFileField(t.Context(), f, h, FileDirectoryTokens{}, field); err == nil {
		t.Fatal("a private field without the private store accepted the file")
	}

	private := filepath.Join(t.TempDir(), "private")
	svc.SetPrivateStore(filestore.NewLocal(private))
	f, h = uploadBody("Contract.pdf", []byte("%PDF-1.7\n"))
	uri, _, _, name, err := svc.UploadFileField(t.Context(), f, h, FileDirectoryTokens{}, field)
	if err != nil || uri != "private://content/files/contract.pdf" || name != "Contract.pdf" {
		t.Fatalf("upload: %q %q %v", uri, name, err)
	}
	if _, err := os.Stat(filepath.Join(private, "content", "files", "contract.pdf")); err != nil {
		t.Fatalf("not in the private store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "content", "files", "contract.pdf")); !os.IsNotExist(err) {
		t.Fatalf("in the public files: %v", err)
	}
	if err := svc.RemoveStoredFile(t.Context(), uri); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(private, "content", "files", "contract.pdf")); !os.IsNotExist(err) {
		t.Errorf("removed private file left: %v", err)
	}
	if err := svc.RemoveStoredFile(t.Context(), "private://../outside.pdf"); err != nil {
		t.Errorf("a key outside the store: %v", err)
	}
}

// TestUploadImageFieldPrivate - a field keeping its images private
// stores them in the private files' store; removing the URI removes the
// image and its derivatives there (named after the whole original).
func TestUploadImageFieldPrivate(t *testing.T) {
	svc, base := newTestUploadService(t)
	private := filepath.Join(t.TempDir(), "private")
	svc.SetPrivateStore(filestore.NewLocal(private))
	field := entity.FieldInstanceDef{ID: 5, WidgetConfig: map[string]any{"private_files": true}}
	f, h := pngUpload(t, "Plan.png")
	uri, w, _, _, _, err := svc.UploadImageField(t.Context(), f, h, FileDirectoryTokens{}, field)
	if err != nil || uri != "private://content/images/plan.png" || w != 10 {
		t.Fatalf("upload: %q %d %v", uri, w, err)
	}
	if _, err := os.Stat(filepath.Join(private, "content", "images", "plan.png")); err != nil {
		t.Fatalf("not in the private store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "content", "images", "plan.png")); !os.IsNotExist(err) {
		t.Fatalf("in the public files: %v", err)
	}
	derivative := filepath.Join(private, "styles", "card", "content", "images", "plan.png.webp")
	if err := os.MkdirAll(filepath.Dir(derivative), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(derivative, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveStoredFile(t.Context(), uri); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(private, "content", "images", "plan.png"), derivative} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s left: %v", p, err)
		}
	}
}
