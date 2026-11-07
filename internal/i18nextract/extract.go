// Package i18nextract finds the interface strings of the code so the
// catalogs can be checked for missing translations (cmd/i18n-extract and
// the completeness test): t, tc, plural and tjs in templates;
// Kosatka.t, Kosatka.tc and Kosatka.formatPlural in JavaScript (static
// files and inline scripts); i18n.T, i18n.TC, i18n.Plural, i18n.N, i18n.NC
// and i18n.Errorf in Go.
package i18nextract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Message - one interface string found in the code.
type Message struct {
	Context string
	Source  string
	// Plural - the plural source form ("" for a plain string); Source is
	// the singular, the catalog key.
	Plural string
	// Theme - the theme the string belongs to ("" - the engine).
	Theme string
	// Module - the module whose code has the string (its directory
	// internal/modules/<name>/; "" - the core).
	Module string
	// JS - translated by JavaScript (Kosatka.t, Kosatka.tc,
	// Kosatka.formatPlural), so it goes into the /i18n/<lang>.js script.
	JS   bool
	File string
	Line int
}

const str = `"((?:[^"\\]|\\.)*)"`

var (
	tmplT      = regexp.MustCompile(`(?:\{\{-?\s*|\(\s*)(?:t|tjs)\s+` + str)
	tmplTC     = regexp.MustCompile(`(?:\{\{-?\s*|\(\s*)tc\s+` + str + `\s+` + str)
	tmplPlural = regexp.MustCompile(`(?:\{\{-?\s*|\(\s*)plural\s+(?:\([^)]*\)|[^\s"]+)\s+` + str + `\s+` + str)
	goT        = regexp.MustCompile(`i18n\.T\(\s*[^,]+,\s*` + str)
	goN        = regexp.MustCompile(`i18n\.(?:N|Errorf)\(\s*` + str)
	goNC       = regexp.MustCompile(`i18n\.NC\(\s*` + str + `,\s*` + str)
	goTC       = regexp.MustCompile(`i18n\.TC\(\s*[^,]+,\s*` + str + `,\s*` + str)
	// JavaScript: 'single' or "double" quoted literals.
	jsStr    = `(?:'((?:[^'\\]|\\.)*)'|"((?:[^"\\]|\\.)*)")`
	jsT      = regexp.MustCompile(`Kosatka\.t\(\s*` + jsStr)
	jsTC     = regexp.MustCompile(`Kosatka\.tc\(\s*` + jsStr + `\s*,\s*` + jsStr)
	jsPlural = regexp.MustCompile(`Kosatka\.formatPlural\(\s*[^,]+,\s*` + jsStr + `\s*,\s*` + jsStr)
	// manifestLabel - a region's label in a theme manifest (theme.yaml),
	// a plain or quoted YAML scalar.
	manifestLabel = regexp.MustCompile(`^\s*(?:-\s+)?label:\s*(.*?)\s*$`)
	// manifestName - the theme's name or description in its manifest
	// (top-level keys).
	manifestName = regexp.MustCompile(`^(?:name|description):\s*(.*?)\s*$`)
	// manifestTags - the theme's tags in its manifest, a flow list
	// ("tags: [dark, retro]").
	manifestTags = regexp.MustCompile(`^tags:\s*\[(.*)\]\s*$`)

	// starterText - a label, description or title in the starter set
	// (internal/app/starter/starter.yaml), a plain or quoted YAML scalar.
	starterText = regexp.MustCompile(`^\s*(?:-\s+)?(?:label|description|title):\s*(.*?)\s*$`)

	goPlural = regexp.MustCompile(`i18n\.Plural\(\s*[^,]+,\s*[^,]+,\s*` + str + `,\s*` + str)
)

// jsUnquote returns a JS literal's content from the single- or double-quoted
// submatch pair; escapes as in JSON, plus \'.
func jsUnquote(single, double string) string {
	s := double
	if single != "" {
		s = strings.ReplaceAll(single, `\'`, "'")
		s = strings.ReplaceAll(s, `"`, `\"`)
	}
	return unquote(s)
}

func unquote(s string) string {
	if u, err := strconv.Unquote(`"` + s + `"`); err == nil {
		return u
	}
	return s
}

// Extract scans the repository at root: templates under web/templates
// (theme strings tagged with their theme), JavaScript under web/static,
// the modules' own templates and JavaScript (internal/modules/<name>/
// templates, static), Go code under internal, pkg and cmd (tests
// skipped) and the starter set's texts (starterSet); a string in a
// module's directory is tagged with the module.
func Extract(root string) ([]Message, error) {
	out, err := extractWeb(root)
	if err != nil {
		return nil, err
	}
	for _, dir := range []string{"internal", "pkg", "cmd"} {
		// A missing directory is skipped, as in extractWeb.
		if _, err := os.Stat(filepath.Join(root, dir)); err != nil {
			continue
		}
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == "generated" {
				return filepath.SkipDir
			}
			if d.IsDir() || filepath.Ext(p) != ".go" || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			return scan(p, "", kindGo, &out)
		})
		if err != nil {
			return nil, err
		}
	}
	if p := filepath.Join(root, filepath.FromSlash(starterSet)); fileExists(p) {
		if err := scan(p, "", kindStarter, &out); err != nil {
			return nil, err
		}
	}
	tagModules(root, out)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

// tagModules sets Module of the strings found in a module's directory.
func tagModules(root string, msgs []Message) {
	base := filepath.Join(root, "internal", "modules")
	for i := range msgs {
		rel, err := filepath.Rel(base, msgs[i].File)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if name, _, ok := strings.Cut(filepath.ToSlash(rel), "/"); ok {
			msgs[i].Module = name
		}
	}
}

// ExtractJS - the strings the JavaScript translates (Kosatka.t, tc,
// formatPlural in web/static/**/*.js and inline scripts of templates),
// for i18n.SetJSKeys at startup: under root, and in the modules' embedded
// files (modules: templates/ and static/ by module, modules.Assets) - a
// site has no module sources on disk.
func ExtractJS(root string, modules map[string]fs.FS) ([]i18n.JSKey, error) {
	msgs, err := extractWeb(root)
	if err != nil {
		return nil, err
	}
	embedded, err := extractAssets(modules)
	if err != nil {
		return nil, err
	}
	msgs = append(msgs, embedded...)
	seen := map[i18n.JSKey]bool{}
	var keys []i18n.JSKey
	for _, m := range msgs {
		k := i18n.JSKey{Context: m.Context, Source: m.Source}
		if m.JS && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return keys, nil
}

type fileKind int

const (
	kindTemplate fileKind = iota
	kindGo
	kindJS
	kindManifest
	kindStarter
)

// starterSet - the starter set of a new site, its texts English sources.
const starterSet = "internal/app/starter/starter.yaml"

// extractWeb scans templates, the themes' manifests (theme.yaml name,
// description, tags and region labels) and the JavaScript of web/static (a missing directory is
// skipped). Minified files and bundled libraries (tinymce,
// vendor, lib) are not scanned.
func extractWeb(root string) ([]Message, error) {
	var out []Message
	// webDir - a directory to scan; theme - the theme its strings are of
	// ("" - the engine's or a module's).
	type webDir struct {
		path  string
		ext   string
		kind  fileKind
		theme string
	}
	dirs := []webDir{
		{filepath.Join(root, "web", "templates"), ".html", kindTemplate, ""},
		{filepath.Join(root, "web", "static"), ".js", kindJS, ""},
	}
	if modules, err := os.ReadDir(filepath.Join(root, "internal", "modules")); err == nil {
		for _, m := range modules {
			if m.IsDir() {
				dir := filepath.Join(root, "internal", "modules", m.Name())
				dirs = append(dirs, webDir{filepath.Join(dir, "templates"), ".html", kindTemplate, ""}, webDir{filepath.Join(dir, "static"), ".js", kindJS, ""})
			}
		}
	}
	for _, dir := range dirs {
		if err := scanWebDir(dir.path, dir.ext, dir.kind, dir.theme, &out); err != nil {
			return nil, err
		}
	}
	// The themes (themes/<theme>/): their manifest, templates and scripts.
	if themes, err := os.ReadDir(filepath.Join(root, "themes")); err == nil {
		for _, th := range themes {
			if !th.IsDir() {
				continue
			}
			msgs, err := ExtractTheme(filepath.Join(root, "themes", th.Name()), th.Name())
			if err != nil {
				return nil, err
			}
			out = append(out, msgs...)
		}
	}
	return out, nil
}

// ExtractTheme - the strings of the theme in dir, tagged with name: its
// manifest's (theme.yaml), its templates' and its scripts'.
func ExtractTheme(dir, name string) ([]Message, error) {
	var out []Message
	if manifest := filepath.Join(dir, "theme.yaml"); fileExists(manifest) {
		if err := scan(manifest, name, kindManifest, &out); err != nil {
			return nil, err
		}
	}
	if err := scanWebDir(filepath.Join(dir, "templates"), ".html", kindTemplate, name, &out); err != nil {
		return nil, err
	}
	if err := scanWebDir(filepath.Join(dir, "static"), ".js", kindJS, name, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// scanWebDir scans the files with ext under path (a missing directory is
// skipped; minified files and bundled libraries - tinymce, vendor, lib -
// too); a theme.yaml among templates is a manifest.
func scanWebDir(path, ext string, kind fileKind, theme string, out *[]Message) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "tinymce", "vendor", "lib":
				return filepath.SkipDir
			}
			return nil
		}
		k := kind
		switch {
		case kind == kindTemplate && d.Name() == "theme.yaml":
			k = kindManifest
		case filepath.Ext(p) != ext || strings.HasSuffix(p, ".min.js"):
			return nil
		}
		return scan(p, theme, k, out)
	})
}

// extractAssets scans the modules' embedded templates and scripts as
// extractWeb scans them on disk.
func extractAssets(modules map[string]fs.FS) ([]Message, error) {
	var out []Message
	for name, fsys := range modules {
		for _, dir := range []struct {
			path, ext string
			kind      fileKind
		}{{"templates", ".html", kindTemplate}, {"static", ".js", kindJS}} {
			err := fs.WalkDir(fsys, dir.path, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						return nil
					}
					return err
				}
				if d.IsDir() {
					switch d.Name() {
					case "tinymce", "vendor", "lib":
						return fs.SkipDir
					}
					return nil
				}
				if path.Ext(p) != dir.ext || strings.HasSuffix(p, ".min.js") {
					return nil
				}
				b, err := fs.ReadFile(fsys, p)
				if err != nil {
					return err
				}
				return scanBytes(name+":"+p, b, "", dir.kind, &out)
			})
			if err != nil {
				return nil, fmt.Errorf("module %s: %w", name, err)
			}
		}
	}
	return out, nil
}

func scan(path, theme string, kind fileKind, out *[]Message) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return scanBytes(path, b, theme, kind, out)
}

// scanBytes - the strings of b, the content of the file at path.
func scanBytes(path string, b []byte, theme string, kind fileKind, out *[]Message) error {
	for i, line := range strings.Split(string(b), "\n") {
		add := func(ctx, src, plural string) {
			*out = append(*out, Message{Context: unquote(ctx), Source: unquote(src), Plural: unquote(plural), Theme: theme, File: path, Line: i + 1})
		}
		addJS := func(ctx, src, plural string) {
			*out = append(*out, Message{Context: ctx, Source: src, Plural: plural, Theme: theme, File: path, Line: i + 1, JS: true})
		}
		if kind == kindStarter {
			if m := starterText.FindStringSubmatch(line); m != nil && !strings.HasPrefix(strings.TrimSpace(line), "#") {
				if src := strings.Trim(m[1], `"'`); src != "" {
					*out = append(*out, Message{Source: src, File: path, Line: i + 1})
				}
			}
			continue
		}
		if kind == kindManifest {
			m := manifestLabel.FindStringSubmatch(line)
			if m == nil {
				m = manifestName.FindStringSubmatch(line)
			}
			if m != nil && m[1] != "" {
				*out = append(*out, Message{Source: strings.Trim(m[1], `"'`), Theme: theme, File: path, Line: i + 1})
			}
			if m := manifestTags.FindStringSubmatch(line); m != nil {
				for _, tag := range strings.Split(m[1], ",") {
					if tag = strings.Trim(strings.TrimSpace(tag), `"'`); tag != "" {
						*out = append(*out, Message{Source: tag, Theme: theme, File: path, Line: i + 1})
					}
				}
			}
			continue
		}
		if kind == kindTemplate || kind == kindJS {
			if kind == kindJS && strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for _, m := range jsT.FindAllStringSubmatch(line, -1) {
				addJS("", jsUnquote(m[1], m[2]), "")
			}
			for _, m := range jsTC.FindAllStringSubmatch(line, -1) {
				addJS(jsUnquote(m[1], m[2]), jsUnquote(m[3], m[4]), "")
			}
			for _, m := range jsPlural.FindAllStringSubmatch(line, -1) {
				addJS("", jsUnquote(m[1], m[2]), jsUnquote(m[3], m[4]))
			}
		}
		if kind == kindJS {
			continue
		}
		if kind == kindTemplate {
			for _, m := range tmplT.FindAllStringSubmatch(line, -1) {
				add("", m[1], "")
			}
			for _, m := range tmplTC.FindAllStringSubmatch(line, -1) {
				add(m[1], m[2], "")
			}
			for _, m := range tmplPlural.FindAllStringSubmatch(line, -1) {
				add("", m[1], m[2])
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		for _, m := range goT.FindAllStringSubmatch(line, -1) {
			add("", m[1], "")
		}
		for _, m := range goN.FindAllStringSubmatch(line, -1) {
			add("", m[1], "")
		}
		for _, m := range goNC.FindAllStringSubmatch(line, -1) {
			add(m[1], m[2], "")
		}
		for _, m := range goTC.FindAllStringSubmatch(line, -1) {
			add(m[1], m[2], "")
		}
		for _, m := range goPlural.FindAllStringSubmatch(line, -1) {
			add("", m[1], m[2])
		}
	}
	return nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// Sources - the source strings' list of msgs (translations/sources): a
// string once, with its plural form, the engine's when the core uses it,
// else the first module's that does; a theme's own strings once per
// theme. A text only the starter set has is marked Starter: it becomes a
// new site's data in its language (a language pack translates it), not
// its interface.
func Sources(msgs []Message) []i18n.Source {
	type k struct{ ctx, src string }
	byKey := map[k]*i18n.Source{}
	themes := map[k]map[string]bool{}
	var starter []Message
	for _, m := range msgs {
		if strings.HasSuffix(filepath.ToSlash(m.File), starterSet) {
			starter = append(starter, m)
			continue
		}
		key := k{m.Context, m.Source}
		if m.Theme != "" {
			if themes[key] == nil {
				themes[key] = map[string]bool{}
			}
			themes[key][m.Theme] = true
		}
		s := byKey[key]
		if s == nil {
			s = &i18n.Source{Context: m.Context, Source: m.Source, Module: m.Module, Theme: m.Theme}
			byKey[key] = s
		}
		if s.Plural == "" {
			s.Plural = m.Plural
		}
		switch {
		case m.Theme != "":
		case m.Module == "":
			s.Module, s.Theme = "", ""
		case s.Theme != "" || (s.Module != "" && m.Module < s.Module):
			s.Module, s.Theme = m.Module, ""
		}
	}
	for _, m := range starter {
		if key := (k{m.Context, m.Source}); byKey[key] == nil {
			byKey[key] = &i18n.Source{Context: m.Context, Source: m.Source, Starter: true}
		}
	}
	var out []i18n.Source
	for key, s := range byKey {
		if s.Theme == "" {
			out = append(out, *s)
			continue
		}
		for theme := range themes[key] {
			t := *s
			t.Theme = theme
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Context != b.Context {
			return a.Context < b.Context
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Theme < b.Theme
	})
	return out
}

// SourcesJSON - the source strings' list as translations/sources keeps
// it: one string a line, so a change reads as a diff.
func SourcesJSON(sources []i18n.Source) ([]byte, error) {
	var b strings.Builder
	b.WriteString("[\n")
	for i, s := range sources {
		var line bytes.Buffer
		enc := json.NewEncoder(&line)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(s); err != nil {
			return nil, err
		}
		b.WriteString("  ")
		b.Write(bytes.TrimSuffix(line.Bytes(), []byte("\n")))
		if i < len(sources)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]\n")
	return []byte(b.String()), nil
}
