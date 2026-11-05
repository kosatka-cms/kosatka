package filestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ErrIsDir - a file operation (Delete) on a folder.
var ErrIsDir = errors.New("filestore: is a folder")

// tempPrefix - Put's file while it is written; never listed.
const tempPrefix = ".filestore-"

// Local keeps the files in a directory on disk: files 0644, folders 0755.
type Local struct {
	root string
}

// NewLocal returns the store of the directory root, made on the first
// write.
func NewLocal(root string) *Local {
	return &Local{root: root}
}

// Root - the store's directory.
func (s *Local) Root() string { return s.root }

// String - the store's directory (the status report shows it).
func (s *Local) String() string { return s.root }

func (s *Local) path(key string) string {
	return filepath.Join(s.root, filepath.FromSlash(key))
}

func (s *Local) info(key string, fi fs.FileInfo) Info {
	in := Info{Key: key, Name: path.Base(key), ModTime: fi.ModTime(), IsDir: fi.IsDir()}
	if key == "" {
		in.Name = ""
	}
	if !in.IsDir {
		in.Size = fi.Size()
	}
	return in
}

// Put writes into a temporary file beside the target and renames it, so
// a reader never sees a partial file; with Exclusive it creates the
// target itself (O_EXCL).
func (s *Local) Put(_ context.Context, key string, r io.Reader, opts PutOptions) (int64, error) {
	key, err := checkKey(key)
	if err != nil {
		return 0, err
	}
	target := s.path(key)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, err
	}
	if opts.Exclusive {
		f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return 0, err
		}
		n, err := io.Copy(f, r)
		if err = errors.Join(err, f.Close()); err != nil {
			_ = os.Remove(target)
			return 0, err
		}
		return n, nil
	}
	f, err := os.CreateTemp(filepath.Dir(target), tempPrefix+"*")
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, r)
	err = errors.Join(err, f.Chmod(0o644), f.Close())
	if err == nil {
		err = os.Rename(f.Name(), target)
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return 0, err
	}
	return n, nil
}

// Open opens a regular file; anything else is ErrNotExist.
func (s *Local) Open(_ context.Context, key string) (File, Info, error) {
	key, err := checkKey(key)
	if err != nil {
		return nil, Info{}, err
	}
	f, err := os.Open(s.path(key))
	if err != nil {
		return nil, Info{}, err
	}
	fi, err := f.Stat()
	if err == nil && !fi.Mode().IsRegular() {
		err = fmt.Errorf("%s: not a file: %w", key, ErrNotExist)
	}
	if err != nil {
		_ = f.Close()
		return nil, Info{}, err
	}
	return f, s.info(key, fi), nil
}

// Stat describes the file or directory key.
func (s *Local) Stat(_ context.Context, key string) (Info, error) {
	key, err := checkDir(key)
	if err != nil {
		return Info{}, err
	}
	fi, err := os.Stat(s.path(key))
	if err != nil {
		return Info{}, err
	}
	return s.info(key, fi), nil
}

// List lists the directory dir a level deep: its regular files and
// directories, Put's temporary files left out.
func (s *Local) List(_ context.Context, dir string) ([]Info, error) {
	dir, err := checkDir(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.path(dir))
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), tempPrefix) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue // removed meanwhile
		}
		if !fi.IsDir() && !fi.Mode().IsRegular() {
			continue
		}
		out = append(out, s.info(Join(dir, e.Name()), fi))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Walk calls fn for every regular file under the directory dir, in
// lexical order, Put's temporary files left out.
func (s *Local) Walk(_ context.Context, dir string, fn func(Info) error) error {
	dir, err := checkDir(dir)
	if err != nil {
		return err
	}
	root := s.path(dir)
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.Type().IsRegular() || strings.HasPrefix(d.Name(), tempPrefix) {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil // removed meanwhile
		}
		rel, err := filepath.Rel(s.root, p)
		if err != nil {
			return err
		}
		return fn(s.info(filepath.ToSlash(rel), fi))
	})
	return err
}

// Delete removes the file key; a missing one is no error, a directory
// is ErrIsDir.
func (s *Local) Delete(_ context.Context, key string) error {
	key, err := checkKey(key)
	if err != nil {
		return err
	}
	p := s.path(key)
	fi, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return fmt.Errorf("%s: %w", key, ErrIsDir)
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// DeleteDir removes the directory dir with everything in it.
func (s *Local) DeleteDir(_ context.Context, dir string) error {
	dir, err := checkKey(dir)
	if err != nil {
		return err
	}
	return os.RemoveAll(s.path(dir))
}

// Mkdir makes the directory dir and its parents.
func (s *Local) Mkdir(_ context.Context, dir string) error {
	dir, err := checkDir(dir)
	if err != nil {
		return err
	}
	return os.MkdirAll(s.path(dir), 0o755)
}

// Rename renames the file or directory from to the free name to
// (ErrExist when taken), making its parents.
func (s *Local) Rename(_ context.Context, from, to string) error {
	from, err := checkKey(from)
	if err != nil {
		return err
	}
	if to, err = checkKey(to); err != nil {
		return err
	}
	if _, err := os.Lstat(s.path(from)); err != nil {
		return err
	}
	if _, err := os.Lstat(s.path(to)); err == nil {
		return fmt.Errorf("%s: %w", to, ErrExist)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path(to)), 0o755); err != nil {
		return err
	}
	return os.Rename(s.path(from), s.path(to))
}
