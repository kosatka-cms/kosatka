// Package filestore keeps files under keys - slash-separated paths
// relative to the store's root ("content/images/a.jpg") - whatever holds
// them: a directory on disk (Local) or a bucket of an object storage.
//
// A folder is a key prefix: it exists while it holds a file or was made
// with Mkdir. Keys are checked with fs.ValidPath: no "..", no leading or
// trailing slash; "" and "." are the root (for the folder methods).
package filestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"time"
)

var (
	// ErrNotExist - no file (or folder) under the key.
	ErrNotExist = fs.ErrNotExist
	// ErrExist - the key is taken (Put with Exclusive, Rename).
	ErrExist = fs.ErrExist
	// ErrInvalidKey - the key isn't a valid path under the root.
	ErrInvalidKey = errors.New("filestore: invalid key")
)

// Info describes a file or a folder.
type Info struct {
	// Key - the full key; Name - its last element.
	Key, Name string
	Size      int64
	ModTime   time.Time
	IsDir     bool
}

// File - an opened file: read, seek (http.ServeContent serves ranges of
// it), close.
type File = io.ReadSeekCloser

// PutOptions - how Put writes a file.
type PutOptions struct {
	// ContentType - the file's media type, for a store that serves the
	// files itself ("" - by its extension).
	ContentType string
	// Exclusive - fail with ErrExist when the key is taken instead of
	// replacing the file.
	Exclusive bool
}

// Store keeps the files. Every method is safe for concurrent use.
type Store interface {
	// Put writes r as the file key (its folders are made as needed) and
	// returns the bytes written. A failed Put leaves no partial file.
	Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (int64, error)
	// Open opens the file key; a folder is ErrNotExist.
	Open(ctx context.Context, key string) (File, Info, error)
	// Stat describes the file or folder key.
	Stat(ctx context.Context, key string) (Info, error)
	// List returns the files and folders directly in the folder dir,
	// sorted by name; a missing folder is ErrNotExist.
	List(ctx context.Context, dir string) ([]Info, error)
	// Walk calls fn for every file under the folder dir (subfolders
	// included); a missing folder calls nothing. The order is the
	// store's.
	Walk(ctx context.Context, dir string, fn func(Info) error) error
	// Delete removes the file key; a missing one is no error.
	Delete(ctx context.Context, key string) error
	// DeleteDir removes the folder dir with everything in it; a missing
	// one is no error. The root can't be removed.
	DeleteDir(ctx context.Context, dir string) error
	// Mkdir makes the folder dir (and its parents); an existing one is no
	// error.
	Mkdir(ctx context.Context, dir string) error
	// Rename moves the file or folder from to the key to, which must be
	// free (ErrExist).
	Rename(ctx context.Context, from, to string) error
}

// Exists reports whether key is a file or folder of s.
func Exists(ctx context.Context, s Store, key string) (bool, error) {
	_, err := s.Stat(ctx, key)
	if errors.Is(err, ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// ReadFile returns the contents of the file key.
func ReadFile(ctx context.Context, s Store, key string) ([]byte, error) {
	f, _, err := s.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// Join joins key elements with slashes, like path.Join; "" when all are
// empty.
func Join(elem ...string) string {
	k := path.Join(elem...)
	if k == "." {
		return ""
	}
	return k
}

// validPath - p is a valid key: fs.ValidPath, not the root.
func validPath(p string) bool { return p != "" && p != "." && fs.ValidPath(p) }

// checkKey returns key if it names a file: a valid path, not the root.
func checkKey(key string) (string, error) {
	if !validPath(key) {
		return "", fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	return key, nil
}

// checkDir returns dir as a key ("" - the root) if it names a folder.
func checkDir(dir string) (string, error) {
	if dir == "" || dir == "." {
		return "", nil
	}
	return checkKey(dir)
}
