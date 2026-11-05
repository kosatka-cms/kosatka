package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

func TestUserFilesListShowsImageFiles(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	up := NewFileUploadService(filestore.NewLocal(tmp), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, log)
	s := NewUserFilesService(filestore.NewLocal(tmp), up, log)
	uid := 3
	root := filepath.Join(tmp, UserDirName(uid))
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "1234567890123456789.jpg"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List(ctx, uid, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].IsDir || entries[0].Name != "1234567890123456789.jpg" {
		t.Fatalf("entries: %+v", entries)
	}
}

func TestMkdirRussianName(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	up := NewFileUploadService(filestore.NewLocal(tmp), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, log)
	s := NewUserFilesService(filestore.NewLocal(tmp), up, log)
	// Cyrillic on purpose: checks transliteration of the directory name.
	if err := s.Mkdir(ctx, 1, "", "  Рисунки  "); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "risunki" {
		t.Fatalf("entries: %+v", entries)
	}
}

func TestFolderTree(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	up := NewFileUploadService(filestore.NewLocal(tmp), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, log)
	s := NewUserFilesService(filestore.NewLocal(tmp), up, log)
	_ = s.Mkdir(ctx, 5, "", "a")
	_ = s.Mkdir(ctx, 5, "a", "b")
	tree, err := s.FolderTree(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Name != "a" || len(tree[0].Children) != 1 {
		t.Fatalf("tree: %+v", tree)
	}
	if tree[0].Children[0].Name != "b" || tree[0].Children[0].Path != "a/b" {
		t.Fatalf("child: %+v", tree[0].Children[0])
	}
}

func TestUserFilesMkdirAndList(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	up := NewFileUploadService(filestore.NewLocal(tmp), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, log)
	s := NewUserFilesService(filestore.NewLocal(tmp), up, log)
	uid := 42

	if err := s.Mkdir(ctx, uid, "", "album"); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List(ctx, uid, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].IsDir || entries[0].Name != "album" {
		t.Fatalf("entries: %+v", entries)
	}
	if err := s.Mkdir(ctx, uid, "", "album"); !errors.Is(err, ErrUserFilesExists) {
		t.Errorf("second Mkdir: %v, want ErrUserFilesExists", err)
	}
	if err := s.Mkdir(ctx, uid, "missing", "x"); !errors.Is(err, ErrUserFilesNotFound) {
		t.Errorf("Mkdir in a missing folder: %v, want ErrUserFilesNotFound", err)
	}
}

func TestUserFilesRejectTraversal(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	up := NewFileUploadService(filestore.NewLocal(tmp), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, log)
	s := NewUserFilesService(filestore.NewLocal(tmp), up, log)

	err := s.Mkdir(ctx, 1, "..", "x")
	if !errors.Is(err, ErrUserFilesInvalidPath) {
		t.Fatalf("expected ErrUserFilesInvalidPath, got %v", err)
	}

	_, err = s.resolveDir(1, "foo/../bar")
	if !errors.Is(err, ErrUserFilesInvalidPath) {
		t.Fatalf("expected invalid path, got %v", err)
	}
}

func TestUserFilesRmdirRoot(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	up := NewFileUploadService(filestore.NewLocal(tmp), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, log)
	s := NewUserFilesService(filestore.NewLocal(tmp), up, log)

	err := s.RmdirEmpty(ctx, 7, "")
	if !errors.Is(err, ErrUserFilesRootReserved) {
		t.Fatalf("expected ErrUserFilesRootReserved, got %v", err)
	}
}

func newTestUserFiles(t *testing.T) (*UserFilesService, string) {
	t.Helper()
	tmp := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	up := NewFileUploadService(filestore.NewLocal(tmp), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, log)
	return NewUserFilesService(filestore.NewLocal(tmp), up, log), tmp
}

func uploadBody(name string, content []byte) (multipart.File, *multipart.FileHeader) {
	return readSeekCloser{Reader: bytes.NewReader(content)}, &multipart.FileHeader{Filename: name, Size: int64(len(content))}
}

// TestUserFilesRename - files keep their extension, folders are
// transliterated, names can't collide or leave the user's folder.
func TestUserFilesRename(t *testing.T) {
	ctx := context.Background()
	s, tmp := newTestUserFiles(t)
	root := filepath.Join(tmp, "u5")
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"docs/a.pdf", "docs/b.pdf", "photo.jpg"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if got, err := s.Rename(ctx, 5, "docs/a.pdf", "Price List.exe"); err != nil || got != "docs/price-list.pdf" {
		t.Fatalf("file rename: %q %v", got, err)
	}
	if got, err := s.Rename(ctx, 5, "photo.jpg", "Summer Photo"); err != nil || got != "Summer_Photo.jpg" {
		t.Fatalf("image rename: %q %v", got, err)
	}
	if got, err := s.Rename(ctx, 5, "docs", "Documents 2026"); err != nil || got != "documents_2026" {
		t.Fatalf("folder rename: %q %v", got, err)
	}
	if _, err := s.Rename(ctx, 5, "documents_2026/b.pdf", "price-list"); !errors.Is(err, ErrUserFilesExists) {
		t.Fatalf("collision: %v", err)
	}
	for _, bad := range []string{"../u6/x.pdf", "/etc/passwd", "", "missing.pdf"} {
		if _, err := s.Rename(ctx, 5, bad, "x"); err == nil {
			t.Errorf("%q: renamed", bad)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "documents_2026", "price-list.pdf")); err != nil {
		t.Fatal("renamed file missing")
	}
}

// TestUserFolderUsages - folders with files are listed, largest first,
// with nested files counted; empty and foreign folders are skipped.
func TestUserFolderUsages(t *testing.T) {
	ctx := context.Background()
	s, tmp := newTestUserFiles(t)
	write := func(rel string, size int) {
		p := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, bytes.Repeat([]byte("x"), size), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("u3/a.pdf", 10)
	write("u3/docs/b.pdf", 30)
	write("u8/c.jpg", 100)
	write("content/images/d.jpg", 500)
	if err := os.MkdirAll(filepath.Join(tmp, "u9"), 0755); err != nil {
		t.Fatal(err)
	}
	got, err := s.FolderUsages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].UserID != 8 || got[0].Bytes != 100 || got[1].UserID != 3 || got[1].Files != 2 || got[1].Bytes != 40 {
		t.Fatalf("usages: %+v", got)
	}
}
