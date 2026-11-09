package demoset

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // the formats a set's own picture may be in
	_ "image/png"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	_ "golang.org/x/image/webp" // a WebP picture too

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
)

// A set's own files, in its directory (a theme's demo/), put in the
// site's files on install: an image field's value {file: <path>, alt: …}
// names a picture (in place of a generated one: a number, {picture: N}),
// a file field's {file: <path>, name: …} a document (a PDF…; name - the
// name its link shows, the file's by default).

// pictureExts - the formats a set's own picture may be in.
var pictureExts = []string{".jpg", ".jpeg", ".png", ".webp"}

// documentExts - the formats a set's own document may be in: those a file
// field allows unless told otherwise (entity.ResolveFieldUploadPolicy).
func documentExts() []string {
	return entity.ResolveFieldUploadPolicy(entity.FieldInstanceDef{FieldType: entity.FieldTypeFile}, entity.UploadSettings{}).AllowedExtensions
}

// pictureFile - the file of the set an image field's value names
// ({file: …}) and its alternative text; "" - not a file's value.
func pictureFile(raw any) (file, alt string) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", ""
	}
	file, _ = m["file"].(string)
	alt, _ = m["alt"].(string)
	return file, alt
}

// documentFile - the file of the set a file field's value names
// ({file: …}) and the name its link shows ("" - the file's); "" - not a
// file's value.
func documentFile(raw any) (file, name string) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", ""
	}
	file, _ = m["file"].(string)
	name, _ = m["name"].(string)
	return file, name
}

// checkPictureName - a set's own picture: a path inside its directory,
// a .jpg, .png or .webp.
func checkPictureName(name string) error {
	if !fs.ValidPath(name) || name == "." {
		return fmt.Errorf("picture %q: a path inside the set's directory", name)
	}
	if !slices.Contains(pictureExts, strings.ToLower(path.Ext(name))) {
		return fmt.Errorf("picture %q: a .jpg, .png or .webp", name)
	}
	return nil
}

// checkDocumentName - a set's own document: a path inside its directory,
// of a format a file field allows (documentExts).
func checkDocumentName(name string) error {
	if !fs.ValidPath(name) || name == "." {
		return fmt.Errorf("document %q: a path inside the set's directory", name)
	}
	exts := documentExts()
	if !slices.Contains(exts, strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")) {
		return fmt.Errorf("document %q: one of .%s", name, strings.Join(exts, ", ."))
	}
	return nil
}

// storageType - the type of the set's field storage name; "" - not the
// set's (the starter's: field_image, field_body).
func (s *Set) storageType(name string) entity.ContentTypeFieldType {
	for _, fs := range s.FieldStorages {
		if fs.Name == name {
			return fs.Type
		}
	}
	return ""
}

// fieldItems - a node's field value as its items: a list's, else the one.
func fieldItems(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return []any{v}
}

// Pictures - the set's own pictures its nodes' image fields and its
// modules' content name ({file: …} anywhere in it: a gallery's pictures),
// sorted, each once.
func (s *Set) Pictures() []string {
	var out []string
	add := func(f string) {
		if !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	s.nodeFiles(func(field string) bool { return s.storageType(field) != entity.FieldTypeFile }, add)
	for _, v := range s.Modules {
		moduleFiles(v, add)
	}
	slices.Sort(out)
	return out
}

// Documents - the set's own documents its nodes' file fields name,
// sorted, each once.
func (s *Set) Documents() []string {
	var out []string
	s.nodeFiles(func(field string) bool { return s.storageType(field) == entity.FieldTypeFile }, func(f string) {
		if !slices.Contains(out, f) {
			out = append(out, f)
		}
	})
	slices.Sort(out)
	return out
}

// nodeFiles calls found with every file ({file: …}) of the nodes' fields
// of which is true.
func (s *Set) nodeFiles(of func(field string) bool, found func(string)) {
	for _, n := range s.Nodes {
		for field, v := range n.Fields {
			if !of(field) {
				continue
			}
			for _, item := range fieldItems(v) {
				if f, _ := pictureFile(item); f != "" {
					found(f)
				}
			}
		}
	}
}

// moduleFiles calls found with every file a module's content names: the
// "file" of each mapping in it.
func moduleFiles(v any, found func(string)) {
	switch v := v.(type) {
	case map[string]any:
		if f, ok := v["file"].(string); ok && f != "" {
			found(f)
		}
		for _, k := range slices.Sorted(maps.Keys(v)) {
			moduleFiles(v[k], found)
		}
	case []any:
		for _, item := range v {
			moduleFiles(item, found)
		}
	}
}

// CheckFiles - what is wrong with the set's own files in fsys (the set's
// directory): each is there, a picture is an image, a document isn't a
// page, a script or a program (files.DocumentAllowed, as an upload).
func (s *Set) CheckFiles(fsys fs.FS) []error {
	var errs []error
	for _, name := range s.Pictures() {
		f, err := fsys.Open(name)
		if err != nil {
			errs = append(errs, fmt.Errorf("picture %q: not in the set's directory", name))
			continue
		}
		_, _, err = image.DecodeConfig(f)
		_ = f.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("picture %q: not an image: %w", name, err))
		}
	}
	for _, name := range s.Documents() {
		f, err := fsys.Open(name)
		if err != nil {
			errs = append(errs, fmt.Errorf("document %q: not in the set's directory", name))
			continue
		}
		head := make([]byte, 512)
		n, err := io.ReadFull(f, head)
		_ = f.Close()
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
			errs = append(errs, fmt.Errorf("document %q: %w", name, err))
			continue
		}
		if !files.DocumentAllowed(name, head[:n]) {
			errs = append(errs, fmt.Errorf("document %q: a page, a script or a program", name))
		}
	}
	return errs
}
