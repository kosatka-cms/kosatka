package imagestyle

import (
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

func TestGetResizedImageURL_outputsWebP(t *testing.T) {
	base := t.TempDir()
	imgPath := filepath.Join(base, "foo.jpg")
	img := imaging.New(200, 300, color.White)
	if err := imaging.Save(img, imgPath, imaging.JPEGQuality(90)); err != nil {
		t.Fatal(err)
	}

	svc := NewResizeService(NewImagingProcessor(), filestore.NewLocal(base), "/uploads/", "")
	url, err := svc.GetResizedImageURL("/uploads/foo.jpg", 100, 150)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(url, "foo-100x150.webp") {
		t.Fatalf("unexpected url: %s", url)
	}

	resizedPath := filepath.Join(base, "resize", "foo-100x150.webp")
	if _, err := os.Stat(resizedPath); err != nil {
		t.Fatalf("resized file missing: %v", err)
	}
}

// TestGetResizedImageURL_staysInFiles - a path leaving the files (..) is
// no original.
func TestGetResizedImageURL_staysInFiles(t *testing.T) {
	svc := NewResizeService(NewImagingProcessor(), filestore.NewLocal(t.TempDir()), "/uploads/", "")
	if _, err := svc.GetResizedImageURL("/uploads/../x/foo.jpg", 10, 10); !errors.Is(err, ErrOriginalNotFound) {
		t.Errorf("err = %v, want ErrOriginalNotFound", err)
	}
}
