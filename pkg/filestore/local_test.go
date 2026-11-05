package filestore_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/filestore/filestoretest"
)

func TestLocal(t *testing.T) {
	filestoretest.Run(t, func(t *testing.T) filestore.Store {
		return filestore.NewLocal(filepath.Join(t.TempDir(), "files"))
	})
}

func TestLocalPermissionsAndTempFiles(t *testing.T) {
	root := t.TempDir()
	s := filestore.NewLocal(root)
	ctx := context.Background()
	if _, err := s.Put(ctx, "a/b.txt", strings.NewReader("x"), filestore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(root, "a", "b.txt"))
	if err != nil || fi.Mode().Perm() != 0o644 {
		t.Errorf("file mode = %v, %v", fi.Mode(), err)
	}
	// A Put cut short leaves its temporary file; it is never listed.
	if err := os.WriteFile(filepath.Join(root, "a", ".filestore-123"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx, "a")
	if err != nil || len(list) != 1 || list[0].Name != "b.txt" {
		t.Errorf("List = %+v, %v", list, err)
	}
	n := 0
	_ = s.Walk(ctx, "", func(filestore.Info) error { n++; return nil })
	if n != 1 {
		t.Errorf("Walk saw %d files, want 1", n)
	}
	if err := s.Delete(ctx, "a"); err == nil {
		t.Error("Delete of a folder succeeded")
	}
}
