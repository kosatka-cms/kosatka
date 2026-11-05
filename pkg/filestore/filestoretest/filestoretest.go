// Package filestoretest checks that a filestore.Store keeps the
// package's contract: every store runs the same tests (Run).
package filestoretest

import (
	"errors"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// Run runs the contract tests; newStore returns an empty store for each.
func Run(t *testing.T, newStore func(t *testing.T) filestore.Store) {
	t.Helper()
	tests := []struct {
		name string
		fn   func(t *testing.T, s filestore.Store)
	}{
		{"PutOpenStat", testPutOpenStat},
		{"PutReplacesAndExclusive", testPutReplacesAndExclusive},
		{"Missing", testMissing},
		{"InvalidKeys", testInvalidKeys},
		{"ListAndMkdir", testListAndMkdir},
		{"Walk", testWalk},
		{"Delete", testDelete},
		{"DeleteDir", testDeleteDir},
		{"Rename", testRename},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn(t, newStore(t))
		})
	}
}

func put(t *testing.T, s filestore.Store, key, body string) {
	t.Helper()
	ctx := t.Context()
	n, err := s.Put(ctx, key, strings.NewReader(body), filestore.PutOptions{})
	if err != nil {
		t.Fatalf("Put(%q): %v", key, err)
	}
	if n != int64(len(body)) {
		t.Fatalf("Put(%q) = %d bytes, want %d", key, n, len(body))
	}
}

func read(t *testing.T, s filestore.Store, key string) string {
	t.Helper()
	ctx := t.Context()
	data, err := filestore.ReadFile(ctx, s, key)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", key, err)
	}
	return string(data)
}

func names(infos []filestore.Info) string {
	var out []string
	for _, in := range infos {
		n := in.Name
		if in.IsDir {
			n += "/"
		}
		out = append(out, n)
	}
	return strings.Join(out, " ")
}

func testPutOpenStat(t *testing.T, s filestore.Store) {
	ctx := t.Context()
	put(t, s, "a/b/c.txt", "hello, world")
	f, info, err := s.Open(ctx, "a/b/c.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if info.Key != "a/b/c.txt" || info.Name != "c.txt" || info.Size != 12 || info.IsDir || info.ModTime.IsZero() {
		t.Errorf("Open info = %+v", info)
	}
	if _, err := f.Seek(7, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if rest, _ := io.ReadAll(f); string(rest) != "world" {
		t.Errorf("after Seek(7): %q", rest)
	}
	st, err := s.Stat(ctx, "a/b/c.txt")
	if err != nil || st.Size != 12 || st.IsDir {
		t.Errorf("Stat file = %+v, %v", st, err)
	}
	st, err = s.Stat(ctx, "a/b")
	if err != nil || !st.IsDir || st.Name != "b" {
		t.Errorf("Stat folder = %+v, %v", st, err)
	}
	if ok, err := filestore.Exists(ctx, s, "a"); !ok || err != nil {
		t.Errorf("Exists(a) = %v, %v", ok, err)
	}
}

func testPutReplacesAndExclusive(t *testing.T, s filestore.Store) {
	ctx := t.Context()
	put(t, s, "f.txt", "one")
	put(t, s, "f.txt", "two")
	if got := read(t, s, "f.txt"); got != "two" {
		t.Errorf("replaced = %q", got)
	}
	_, err := s.Put(ctx, "f.txt", strings.NewReader("three"), filestore.PutOptions{Exclusive: true})
	if !errors.Is(err, filestore.ErrExist) {
		t.Errorf("exclusive Put over a file: %v, want ErrExist", err)
	}
	if got := read(t, s, "f.txt"); got != "two" {
		t.Errorf("after a refused exclusive Put = %q", got)
	}
	if _, err := s.Put(ctx, "g.txt", strings.NewReader("new"), filestore.PutOptions{Exclusive: true}); err != nil {
		t.Errorf("exclusive Put of a free key: %v", err)
	}
}

func testMissing(t *testing.T, s filestore.Store) {
	ctx := t.Context()
	put(t, s, "dir/f.txt", "x")
	if _, _, err := s.Open(ctx, "nope.txt"); !errors.Is(err, filestore.ErrNotExist) {
		t.Errorf("Open missing: %v", err)
	}
	if _, _, err := s.Open(ctx, "dir"); !errors.Is(err, filestore.ErrNotExist) {
		t.Errorf("Open a folder: %v", err)
	}
	if _, err := s.Stat(ctx, "nope"); !errors.Is(err, filestore.ErrNotExist) {
		t.Errorf("Stat missing: %v", err)
	}
	if _, err := s.List(ctx, "nope"); !errors.Is(err, filestore.ErrNotExist) {
		t.Errorf("List missing: %v", err)
	}
	if ok, err := filestore.Exists(ctx, s, "nope"); ok || err != nil {
		t.Errorf("Exists(missing) = %v, %v", ok, err)
	}
	// A prefix of a name isn't a folder.
	if _, err := s.Stat(ctx, "di"); !errors.Is(err, filestore.ErrNotExist) {
		t.Errorf("Stat a name's prefix: %v", err)
	}
}

func testInvalidKeys(t *testing.T, s filestore.Store) {
	ctx := t.Context()
	for _, key := range []string{"../x", "/x", "a//b", "a/../b", "a/", ""} {
		if _, err := s.Put(ctx, key, strings.NewReader("x"), filestore.PutOptions{}); !errors.Is(err, filestore.ErrInvalidKey) {
			t.Errorf("Put(%q): %v, want ErrInvalidKey", key, err)
		}
	}
	if err := s.DeleteDir(ctx, ""); !errors.Is(err, filestore.ErrInvalidKey) {
		t.Errorf("DeleteDir(root): %v, want ErrInvalidKey", err)
	}
}

func testListAndMkdir(t *testing.T, s filestore.Store) {
	ctx := t.Context()
	put(t, s, "b.txt", "bb")
	put(t, s, "a/x.txt", "x")
	put(t, s, "a/sub/y.txt", "y")
	if err := s.Mkdir(ctx, "empty/inner"); err != nil {
		t.Fatal(err)
	}
	if err := s.Mkdir(ctx, "a"); err != nil {
		t.Errorf("Mkdir of an existing folder: %v", err)
	}
	root, err := s.List(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := names(root); got != "a/ b.txt empty/" {
		t.Errorf("List(root) = %q", got)
	}
	for _, in := range root {
		if in.Name == "b.txt" && (in.Size != 2 || in.Key != "b.txt") {
			t.Errorf("List file info = %+v", in)
		}
		if in.Name == "a" && in.Key != "a" {
			t.Errorf("List folder info = %+v", in)
		}
	}
	a, err := s.List(ctx, "a")
	if err != nil || names(a) != "sub/ x.txt" {
		t.Errorf("List(a) = %q, %v", names(a), err)
	}
	if a[1].Key != "a/x.txt" {
		t.Errorf("List(a) key = %q", a[1].Key)
	}
	inner, err := s.List(ctx, "empty/inner")
	if err != nil || len(inner) != 0 {
		t.Errorf("List(empty folder) = %q, %v", names(inner), err)
	}
	if st, err := s.Stat(ctx, "empty/inner"); err != nil || !st.IsDir {
		t.Errorf("Stat(empty folder) = %+v, %v", st, err)
	}
}

func testWalk(t *testing.T, s filestore.Store) {
	ctx := t.Context()
	put(t, s, "w/1.txt", "1")
	put(t, s, "w/d/2.txt", "22")
	put(t, s, "w-other/3.txt", "333")
	if err := s.Mkdir(ctx, "w/empty"); err != nil {
		t.Fatal(err)
	}
	var got []string
	var size int64
	err := s.Walk(ctx, "w", func(in filestore.Info) error {
		if in.IsDir {
			t.Errorf("Walk gave a folder: %+v", in)
		}
		got = append(got, in.Key)
		size += in.Size
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if strings.Join(got, " ") != "w/1.txt w/d/2.txt" || size != 3 {
		t.Errorf("Walk(w) = %v (%d bytes)", got, size)
	}
	stop := errors.New("stop")
	if err := s.Walk(ctx, "w", func(filestore.Info) error { return stop }); !errors.Is(err, stop) {
		t.Errorf("Walk returns fn's error: %v", err)
	}
	if err := s.Walk(ctx, "missing", func(filestore.Info) error { return stop }); err != nil {
		t.Errorf("Walk(missing) = %v", err)
	}
}

func testDelete(t *testing.T, s filestore.Store) {
	ctx := t.Context()
	put(t, s, "d/f.txt", "x")
	put(t, s, "d/g.txt", "x")
	if err := s.Delete(ctx, "d/f.txt"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := filestore.Exists(ctx, s, "d/f.txt"); ok {
		t.Error("deleted file still exists")
	}
	if err := s.Delete(ctx, "d/f.txt"); err != nil {
		t.Errorf("Delete(missing) = %v", err)
	}
	if got := read(t, s, "d/g.txt"); got != "x" {
		t.Errorf("the other file = %q", got)
	}
}

func testDeleteDir(t *testing.T, s filestore.Store) {
	ctx := t.Context()
	put(t, s, "t/a.txt", "x")
	put(t, s, "t/s/b.txt", "x")
	put(t, s, "t-keep/c.txt", "x")
	if err := s.Mkdir(ctx, "t/empty"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDir(ctx, "t"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := filestore.Exists(ctx, s, "t"); ok {
		t.Error("deleted folder still exists")
	}
	if root, err := s.List(ctx, ""); err != nil || names(root) != "t-keep/" {
		t.Errorf("List(root) after DeleteDir = %q, %v", names(root), err)
	}
	if got := read(t, s, "t-keep/c.txt"); got != "x" {
		t.Errorf("a folder with a common prefix = %q", got)
	}
	if err := s.DeleteDir(ctx, "t"); err != nil {
		t.Errorf("DeleteDir(missing) = %v", err)
	}
}

func testRename(t *testing.T, s filestore.Store) {
	ctx := t.Context()
	put(t, s, "r/a.txt", "aa")
	put(t, s, "r/b.txt", "bb")
	if err := s.Rename(ctx, "r/a.txt", "r/b.txt"); !errors.Is(err, filestore.ErrExist) {
		t.Errorf("Rename onto a file: %v, want ErrExist", err)
	}
	if err := s.Rename(ctx, "r/a.txt", "r/c.txt"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s, "r/c.txt"); got != "aa" {
		t.Errorf("renamed = %q", got)
	}
	if ok, _ := filestore.Exists(ctx, s, "r/a.txt"); ok {
		t.Error("the old name still exists")
	}
	if err := s.Rename(ctx, "r/missing.txt", "r/d.txt"); !errors.Is(err, filestore.ErrNotExist) {
		t.Errorf("Rename(missing) = %v", err)
	}

	put(t, s, "r/sub/x.txt", "xx")
	if err := s.Mkdir(ctx, "r/sub/empty"); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(ctx, "r/sub", "r/moved"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s, "r/moved/x.txt"); got != "xx" {
		t.Errorf("a file of the renamed folder = %q", got)
	}
	if st, err := s.Stat(ctx, "r/moved/empty"); err != nil || !st.IsDir {
		t.Errorf("the renamed folder's empty folder = %+v, %v", st, err)
	}
	if ok, _ := filestore.Exists(ctx, s, "r/sub"); ok {
		t.Error("the renamed folder's old name still exists")
	}
	if err := s.Rename(ctx, "r/moved", "r/b.txt"); !errors.Is(err, filestore.ErrExist) {
		t.Errorf("Rename a folder onto a file: %v, want ErrExist", err)
	}
}
