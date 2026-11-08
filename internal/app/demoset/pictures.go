package demoset

import (
	"fmt"
	"image"
	_ "image/jpeg" // the formats a set's own picture may be in
	_ "image/png"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	_ "golang.org/x/image/webp" // a WebP picture too
)

// A set's own pictures: an image field's value {file: <path>, alt: …}
// names a file of the set's directory (a theme's demo/), put in the
// site's files on install in place of a generated picture (a number,
// {picture: N}).

// pictureExts - the formats a set's own picture may be in.
var pictureExts = []string{".jpg", ".jpeg", ".png", ".webp"}

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

// fieldItems - a node's field value as its items: a list's, else the one.
func fieldItems(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return []any{v}
}

// Pictures - the set's own pictures its nodes and its modules' content
// name ({file: …} anywhere in it: a gallery's pictures), sorted, each
// once.
func (s *Set) Pictures() []string {
	var out []string
	add := func(f string) {
		if !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	for _, n := range s.Nodes {
		for _, v := range n.Fields {
			for _, item := range fieldItems(v) {
				if f, _ := pictureFile(item); f != "" {
					add(f)
				}
			}
		}
	}
	for _, v := range s.Modules {
		moduleFiles(v, add)
	}
	slices.Sort(out)
	return out
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

// CheckPictures - what is wrong with the set's own pictures in fsys (the
// set's directory): each is there and is an image.
func (s *Set) CheckPictures(fsys fs.FS) []error {
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
	return errs
}
