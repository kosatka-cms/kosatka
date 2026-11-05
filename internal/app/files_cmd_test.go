package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// TestMigrateFiles - the originals are copied, the derivatives and
// dotfiles left out (the derivatives too when asked for), the summary
// says so; a second run finds everything there.
func TestMigrateFiles(t *testing.T) {
	src := filestore.NewLocal(filepath.Join(t.TempDir(), "files"))
	dst := filestore.NewLocal(filepath.Join(t.TempDir(), "bucket"))
	for _, key := range []string{"content/images/a.jpg", "u1/doc.pdf", "styles/thumb/content/images/a.webp", "resize/content/images/a-10x10.webp", "u1/.htaccess"} {
		if _, err := src.Put(t.Context(), key, strings.NewReader("12345"), filestore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	var out strings.Builder
	stats, err := migrateFiles(t.Context(), src, dst, filestore.CopyOptions{Skip: migrateSkip(false)}, &out)
	if err != nil || stats.Copied != 2 || stats.Skipped != 3 {
		t.Fatalf("%+v %v\n%s", stats, err, out.String())
	}
	if !strings.Contains(out.String(), "copied 2 (10 B), already there 0, left out 3") {
		t.Errorf("summary: %s", out.String())
	}
	if ok, _ := filestore.Exists(t.Context(), dst, "styles"); ok {
		t.Error("derivatives copied")
	}
	stats, err = migrateFiles(t.Context(), src, dst, filestore.CopyOptions{Skip: migrateSkip(true)}, &out)
	if err != nil || stats.Copied != 2 || stats.Existing != 2 || stats.Skipped != 1 {
		t.Errorf("with derivatives: %+v %v", stats, err)
	}
}
