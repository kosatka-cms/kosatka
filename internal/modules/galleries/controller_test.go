package galleries

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTemplatesMarkLightbox - every module template with image links
// (content-image-link) marks their container data-lightbox: app.js opens
// in the lightbox only the links inside such a container (the gallery
// page once lost it and its images stopped opening).
func TestTemplatesMarkLightbox(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("templates", "*", "*.html"))
	if err != nil || len(files) == 0 {
		t.Fatalf("templates: %v %v", files, err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if strings.Contains(s, "content-image-link") && !strings.Contains(s, "data-lightbox") {
			t.Errorf("%s: image links without a data-lightbox container", f)
		}
	}
}
