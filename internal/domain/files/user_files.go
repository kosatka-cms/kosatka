package files

import (
	"context"
	"errors"
	"fmt"
	"image"

	// Decoders image.Decode needs for these formats.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/fsname"
	_ "golang.org/x/image/webp" // the WebP decoder for image.Decode
)

// UserRichTextFilesPrefix - the prefix of a user's folder name
// (UserDirName).
const UserRichTextFilesPrefix = "u" // folder name is u{id}, e.g. u1234

// The errors of UserFilesService's operations on a path in a user's
// folder.
var (
	ErrUserFilesInvalidPath  = errors.New("invalid path")
	ErrUserFilesNotFound     = errors.New("path not found")
	ErrUserFilesNotDir       = errors.New("not a directory")
	ErrUserFilesDirNotEmpty  = errors.New("directory is not empty")
	ErrUserFilesIsDir        = errors.New("path is a directory")
	ErrUserFilesRootReserved = errors.New("cannot modify user root this way")
	ErrUserFilesExists       = errors.New("a file or folder with this name already exists")
)

// User file kinds (UserFilesEntry.Kind).
const (
	UserFileKindImage = "image"
	UserFileKindFile  = "file"
)

var userFilesSegmentRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// isSafeUserFileName allows uploaded/listable file names (ASCII letters/digits, one or more dots for extension).
func isSafeUserFileName(name string) bool {
	if len(name) < 1 || len(name) > 255 {
		return false
	}
	if strings.Contains(name, "..") {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			continue
		case r == '.', r == '_', r == '-':
			continue
		default:
			return false
		}
	}
	return true
}

// UserFilesEntry is one row in a directory listing.
type UserFilesEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    *int64 `json:"size,omitempty"` // files only
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	ModTime string `json:"mod_time,omitempty"` // RFC3339, files; dirs omitted in table UI
	// Kind - UserFileKindImage or UserFileKindFile; empty for folders.
	Kind string `json:"kind,omitempty"`
}

// FolderNode is one folder in the recursive tree (paths relative to user root, "/" separated).
type FolderNode struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Children []FolderNode `json:"children,omitempty"`
}

// UserFilesService manages per-user folders u{userID}/ in the public
// files' store (paths.PublicStore); paths given to it are relative to the
// user's folder.
type UserFilesService struct {
	store   filestore.Store
	uploads *FileUploadService
	logger  *slog.Logger
}

// NewUserFilesService - the users' folders in store; uploads checks the
// files uploaded to them.
func NewUserFilesService(store filestore.Store, uploads *FileUploadService, logger *slog.Logger) *UserFilesService {
	return &UserFilesService{
		store:   store,
		uploads: uploads,
		logger:  logger,
	}
}

// UserDirName returns the storage subdirectory name for a user, e.g. u42.
func UserDirName(userID int) string {
	return fmt.Sprintf("%s%d", UserRichTextFilesPrefix, userID)
}

// ensureUserRoot creates u{userID} if missing.
func (s *UserFilesService) ensureUserRoot(ctx context.Context, userID int) error {
	return s.store.Mkdir(ctx, UserDirName(userID))
}

// resolveDir returns the key of a folder under the user's tree, or error.
// rel uses forward slashes or backslashes; empty means user root.
func (s *UserFilesService) resolveDir(userID int, rel string) (string, error) {
	rel = strings.Trim(rel, `/\`)
	if rel == "" || rel == "." {
		return UserDirName(userID), nil
	}
	parts := splitPath(rel)
	for _, p := range parts {
		if !userFilesSegmentRe.MatchString(p) {
			return "", ErrUserFilesInvalidPath
		}
	}
	return path.Join(append([]string{UserDirName(userID)}, parts...)...), nil
}

// resolveUnderUser returns the key of a path under the user's tree.
// lastIsFile: true when rel points to a file - the last path segment may contain a dot (e.g. foo.jpg).
func (s *UserFilesService) resolveUnderUser(userID int, rel string, lastIsFile bool) (string, error) {
	rel = strings.Trim(rel, `/\`)
	if rel == "" {
		return "", ErrUserFilesInvalidPath
	}
	parts := splitPath(rel)
	if len(parts) == 0 {
		return "", ErrUserFilesInvalidPath
	}
	for i, p := range parts {
		last := i == len(parts)-1
		if last && lastIsFile {
			if !isSafeUserFileName(p) {
				return "", ErrUserFilesInvalidPath
			}
		} else {
			if !userFilesSegmentRe.MatchString(p) {
				return "", ErrUserFilesInvalidPath
			}
		}
	}
	return path.Join(append([]string{UserDirName(userID)}, parts...)...), nil
}

func splitPath(rel string) []string {
	raw := strings.FieldsFunc(rel, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// stat describes key: ErrUserFilesNotFound when there is nothing.
func (s *UserFilesService) stat(ctx context.Context, key string) (filestore.Info, error) {
	info, err := s.store.Stat(ctx, key)
	if errors.Is(err, filestore.ErrNotExist) {
		return info, ErrUserFilesNotFound
	}
	return info, err
}

// PublicWebPath returns the URL path (under paths.PublicFilesURL) of a
// key of the store.
func (s *UserFilesService) PublicWebPath(key string) string {
	return paths.PublicFilesURL() + key
}

// List returns directory entries for a path under the user's folder (relative to u{userID}).
func (s *UserFilesService) List(ctx context.Context, userID int, rel string) ([]UserFilesEntry, error) {
	if err := s.ensureUserRoot(ctx, userID); err != nil {
		return nil, err
	}
	dir, err := s.resolveDir(userID, rel)
	if err != nil {
		return nil, err
	}
	st, err := s.stat(ctx, dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir {
		return nil, ErrUserFilesNotDir
	}
	entries, err := s.store.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	out := make([]UserFilesEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir {
			if userFilesSegmentRe.MatchString(e.Name) {
				out = append(out, UserFilesEntry{Name: e.Name, IsDir: true})
			}
			continue
		}
		if !isSafeUserFileName(e.Name) {
			continue
		}
		sz := e.Size
		entry := UserFilesEntry{
			Name:    e.Name,
			Size:    &sz,
			ModTime: e.ModTime.UTC().Format(time.RFC3339),
			Kind:    UserFileKindFile,
		}
		if isImageFileName(e.Name) {
			entry.Kind = UserFileKindImage
			entry.Width, entry.Height = s.imageDimensions(ctx, e.Key)
		}
		out = append(out, entry)
	}
	return out, nil
}

func isImageFileName(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	default:
		return false
	}
}

func (s *UserFilesService) imageDimensions(ctx context.Context, key string) (w, h int) {
	f, _, err := s.store.Open(ctx, key)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// FolderTree returns nested folders under the user's directory (for sidebar navigation).
func (s *UserFilesService) FolderTree(ctx context.Context, userID int) ([]FolderNode, error) {
	if err := s.ensureUserRoot(ctx, userID); err != nil {
		return nil, err
	}
	return s.readFolderTree(ctx, UserDirName(userID), "")
}

func (s *UserFilesService) readFolderTree(ctx context.Context, dir, relFromUser string) ([]FolderNode, error) {
	entries, err := s.store.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	var nodes []FolderNode
	for _, e := range entries {
		if !e.IsDir || !userFilesSegmentRe.MatchString(e.Name) {
			continue
		}
		childRel := e.Name
		if relFromUser != "" {
			childRel = relFromUser + "/" + e.Name
		}
		children, err := s.readFolderTree(ctx, e.Key, childRel)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, FolderNode{
			Name:     e.Name,
			Path:     childRel,
			Children: children,
		})
	}
	if nodes == nil {
		nodes = []FolderNode{}
	}
	return nodes, nil
}

// Mkdir creates a subdirectory under parentRel (empty = user root). name is one segment.
// Cyrillic folder names are transliterated to Latin [a-z0-9_-].
func (s *UserFilesService) Mkdir(ctx context.Context, userID int, parentRel, name string) error {
	name = fsname.NormalizeFolderSegment(name)
	if name == "" || !userFilesSegmentRe.MatchString(name) {
		return ErrUserFilesInvalidPath
	}
	if err := s.ensureUserRoot(ctx, userID); err != nil {
		return err
	}
	parent, err := s.resolveDir(userID, parentRel)
	if err != nil {
		return err
	}
	st, err := s.stat(ctx, parent)
	if err != nil {
		return err
	}
	if !st.IsDir {
		return ErrUserFilesNotDir
	}
	key := path.Join(parent, name)
	if taken, err := filestore.Exists(ctx, s.store, key); err != nil {
		return err
	} else if taken {
		return ErrUserFilesExists
	}
	return s.store.Mkdir(ctx, key)
}

// RmdirEmpty removes an empty directory; rel is relative to user root, non-empty string.
func (s *UserFilesService) RmdirEmpty(ctx context.Context, userID int, rel string) error {
	rel = strings.Trim(rel, `/\`)
	if rel == "" {
		return ErrUserFilesRootReserved
	}
	dir, err := s.resolveUnderUser(userID, rel, false)
	if err != nil {
		return err
	}
	st, err := s.stat(ctx, dir)
	if err != nil {
		return err
	}
	if !st.IsDir {
		return ErrUserFilesNotDir
	}
	entries, err := s.store.List(ctx, dir)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return ErrUserFilesDirNotEmpty
	}
	return s.store.DeleteDir(ctx, dir)
}

// RemoveFile deletes a file under the user's tree.
func (s *UserFilesService) RemoveFile(ctx context.Context, userID int, rel string) error {
	rel = strings.Trim(rel, `/\`)
	if rel == "" {
		return ErrUserFilesInvalidPath
	}
	key, err := s.resolveUnderUser(userID, rel, true)
	if err != nil {
		return err
	}
	st, err := s.stat(ctx, key)
	if err != nil {
		return err
	}
	if st.IsDir {
		return ErrUserFilesIsDir
	}
	return s.store.Delete(ctx, key)
}

// Rename gives a file or folder (rel, relative to the user's root) a new
// name in the same folder. A folder name is transliterated like Mkdir; a
// file keeps its extension. Returns the new relative path.
func (s *UserFilesService) Rename(ctx context.Context, userID int, rel, newName string) (string, error) {
	rel = strings.Trim(rel, `/\`)
	if rel == "" {
		return "", ErrUserFilesRootReserved
	}
	src, err := s.resolveUnderUser(userID, rel, true)
	if err != nil {
		if src, err = s.resolveUnderUser(userID, rel, false); err != nil {
			return "", err
		}
	}
	st, err := s.stat(ctx, src)
	if err != nil {
		return "", err
	}
	var name string
	if st.IsDir {
		name = fsname.NormalizeFolderSegment(newName)
		if name == "" || !userFilesSegmentRe.MatchString(name) {
			return "", ErrUserFilesInvalidPath
		}
	} else {
		ext := path.Ext(src)
		stem := fsname.SanitizeImageStem(strings.TrimSuffix(strings.TrimSpace(newName), ext))
		if !isImageFileName(src) {
			stem = fsname.FileStem(strings.TrimSuffix(strings.TrimSpace(newName), ext), "file")
		}
		name = stem + ext
		if !isSafeUserFileName(name) {
			return "", ErrUserFilesInvalidPath
		}
	}
	dst := path.Join(path.Dir(src), name)
	root := UserDirName(userID)
	if dst == src {
		return strings.TrimPrefix(src, root+"/"), nil
	}
	if err := s.store.Rename(ctx, src, dst); err != nil {
		if errors.Is(err, filestore.ErrExist) {
			return "", ErrUserFilesExists
		}
		return "", err
	}
	return strings.TrimPrefix(dst, root+"/"), nil
}

// UserFolderUsage - what a user's folder holds (admin files overview).
type UserFolderUsage struct {
	UserID   int
	Files    int
	Bytes    int64
	Modified time.Time
}

var userFolderNameRe = regexp.MustCompile(`^` + UserRichTextFilesPrefix + `([0-9]+)$`)

// FolderUsages - every user folder (u{id}) holding at least one file, with
// the number of files, their total size and the latest change; largest
// first.
func (s *UserFilesService) FolderUsages(ctx context.Context) ([]UserFolderUsage, error) {
	entries, err := s.store.List(ctx, "")
	if err != nil {
		if errors.Is(err, filestore.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []UserFolderUsage
	for _, e := range entries {
		m := userFolderNameRe.FindStringSubmatch(e.Name)
		if m == nil || !e.IsDir {
			continue
		}
		id, err := strconv.Atoi(m[1])
		if err != nil || id <= 0 {
			continue
		}
		u, err := s.Usage(ctx, id)
		if err != nil {
			return nil, err
		}
		if u.Files > 0 {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return out[i].UserID < out[j].UserID
	})
	return out, nil
}

// Usage - the number of files in a user's folder (all subfolders), their
// total size and the latest change.
func (s *UserFilesService) Usage(ctx context.Context, userID int) (UserFolderUsage, error) {
	u := UserFolderUsage{UserID: userID}
	err := s.store.Walk(ctx, UserDirName(userID), func(in filestore.Info) error {
		u.Files++
		u.Bytes += in.Size
		if in.ModTime.After(u.Modified) {
			u.Modified = in.ModTime
		}
		return nil
	})
	return u, err
}
