package filestore_test

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

func putAll(t *testing.T, s filestore.Store, files map[string]string) {
	t.Helper()
	for key, body := range files {
		if _, err := s.Put(t.Context(), key, strings.NewReader(body), filestore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCopy - every file but the skipped ones reaches the destination;
// one there already with the same size isn't copied again, one of
// another size is; a dry run copies nothing.
func TestCopy(t *testing.T) {
	ctx := context.Background()
	src := filestore.NewLocal(filepath.Join(t.TempDir(), "src"))
	dst := filestore.NewLocal(filepath.Join(t.TempDir(), "dst"))
	putAll(t, src, map[string]string{
		"a.jpg": "aaaa", "d/b.pdf": "bb", "d/e/c.txt": "c",
		"styles/s/a.webp": "w", "same.txt": "same", "changed.txt": "new!",
	})
	putAll(t, dst, map[string]string{"same.txt": "SAME", "changed.txt": "old"})
	skip := func(in filestore.Info) bool { return strings.HasPrefix(in.Key, "styles/") }

	dry, err := filestore.Copy(ctx, src, dst, filestore.CopyOptions{Skip: skip, DryRun: true}, nil)
	if err != nil || dry.Copied != 4 || dry.Existing != 1 || dry.Skipped != 1 {
		t.Fatalf("dry run: %+v %v", dry, err)
	}
	if ok, _ := filestore.Exists(ctx, dst, "a.jpg"); ok {
		t.Fatal("a dry run copied a file")
	}

	var mu sync.Mutex
	var reported []string
	stats, err := filestore.Copy(ctx, src, dst, filestore.CopyOptions{Skip: skip, Jobs: 3}, func(in filestore.Info, err error) {
		mu.Lock()
		defer mu.Unlock()
		reported = append(reported, in.Key)
	})
	if err != nil || stats.Copied != 4 || stats.Bytes != 11 || stats.Existing != 1 || stats.Skipped != 1 || stats.Failed != 0 {
		t.Fatalf("copy: %+v %v", stats, err)
	}
	if len(reported) != 4 {
		t.Errorf("reported %v", reported)
	}
	for key, want := range map[string]string{"a.jpg": "aaaa", "d/e/c.txt": "c", "changed.txt": "new!", "same.txt": "SAME"} {
		if got, err := filestore.ReadFile(ctx, dst, key); err != nil || string(got) != want {
			t.Errorf("%s = %q, %v", key, got, err)
		}
	}
	if ok, _ := filestore.Exists(ctx, dst, "styles/s/a.webp"); ok {
		t.Error("a skipped file was copied")
	}

	again, err := filestore.Copy(ctx, src, dst, filestore.CopyOptions{Skip: skip}, nil)
	if err != nil || again.Copied != 0 || again.Existing != 5 {
		t.Errorf("second run: %+v %v", again, err)
	}
}

// refusingStore refuses every write.
type refusingStore struct{ filestore.Store }

var errRefused = errors.New("refused")

func (refusingStore) Put(context.Context, string, io.Reader, filestore.PutOptions) (int64, error) {
	return 0, errRefused
}

func TestCopyReportsFailures(t *testing.T) {
	src := filestore.NewLocal(t.TempDir())
	putAll(t, src, map[string]string{"a.txt": "a", "b.txt": "b"})
	dst := refusingStore{filestore.NewLocal(t.TempDir())}
	var failed int
	stats, err := filestore.Copy(t.Context(), src, dst, filestore.CopyOptions{Jobs: 1}, func(_ filestore.Info, err error) {
		if errors.Is(err, errRefused) {
			failed++
		}
	})
	if err == nil || stats.Failed != 2 || failed != 2 || stats.Copied != 0 {
		t.Errorf("%+v %v (reported %d)", stats, err, failed)
	}
}

func TestHasDotSegment(t *testing.T) {
	for key, want := range map[string]bool{"a/.htaccess": true, ".x": true, "a/.hidden/b.jpg": true, "a/b.jpg": false, "a.b/c": false} {
		if got := filestore.HasDotSegment(key); got != want {
			t.Errorf("%s: %v", key, got)
		}
	}
}
