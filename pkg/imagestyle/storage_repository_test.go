package imagestyle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// openOriginalKey - the key of the original OpenOriginal finds.
func openOriginalKey(r *StorageRepository, rel string) (string, error) {
	f, info, err := r.OpenOriginal(context.Background(), rel)
	if err != nil {
		return "", err
	}
	_ = f.Close()
	return info.Key, nil
}

func TestOpenOriginal_resolvesWebPToPNG(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "news")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cover.png"), []byte("png"), 0644); err != nil {
		t.Fatal(err)
	}

	repo := NewStorageRepository(filestore.NewLocal(base))
	got, err := openOriginalKey(repo, "news/cover.webp")
	if err != nil {
		t.Fatal(err)
	}
	want := "news/cover.png"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// TestExtensionCandidates_jpegDerivative - a JPEG style's derivative
// (.jpg) tries the .jpg original first, then the source's other
// extensions (a PNG original gets a .jpg derivative URL).
func TestExtensionCandidates_jpegDerivative(t *testing.T) {
	cands := extensionCandidates("articles/foo.jpg")
	if len(cands) < 2 || cands[0] != "articles/foo.jpg" {
		t.Fatalf("unexpected candidates: %v", cands)
	}
	found := false
	for _, c := range cands {
		if c == "articles/foo.png" {
			found = true
		}
	}
	if !found {
		t.Fatalf("png source not tried: %v", cands)
	}
}

func TestExtensionCandidates_otherExtUnchanged(t *testing.T) {
	cands := extensionCandidates("articles/foo.png")
	if len(cands) != 1 || cands[0] != "articles/foo.png" {
		t.Fatalf("unexpected candidates: %v", cands)
	}
}

// TestOpenOriginal - a derivative's path finds its source (the .webp of
// a .jpg one); an old derivative URL's scheme segment (public/<path>) is
// a path as any other: a file there is found, none - not found.
func TestOpenOriginal(t *testing.T) {
	base := t.TempDir()
	for _, f := range []string{"news/pack001/cover.jpg", "public/own.png"} {
		full := filepath.Join(base, f)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := NewStorageRepository(filestore.NewLocal(base))
	for path, want := range map[string]string{
		"news/pack001/cover.jpg":  "news/pack001/cover.jpg",
		"news/pack001/cover.webp": "news/pack001/cover.jpg",
		"public/own.png":          "public/own.png",
	} {
		got, err := openOriginalKey(r, path)
		if err != nil || got != want {
			t.Errorf("%s: %q %v, want %s", path, got, err, want)
		}
	}
	for _, rel := range []string{"public/news/pack001/cover.jpg", "public/missing.jpg", "../outside.jpg", "/news/../../x.jpg"} {
		if _, err := openOriginalKey(r, rel); !errors.Is(err, ErrOriginalNotFound) {
			t.Errorf("%s: %v, want ErrOriginalNotFound", rel, err)
		}
	}
}
