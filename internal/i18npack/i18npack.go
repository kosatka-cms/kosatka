// Package i18npack builds a translations catalog - the index.json the
// sites download languages by (service.TranslationsCatalog) and a catalog
// file per language next to it - from a translations repository:
//
//	<lang>/language.yaml    the language's version and whether it is a draft
//	<lang>/<component>.json a component's translations (the catalog format)
//
// Each string is checked against the code's strings (i18n.Source lists,
// the engine's translations/sources/strings.json and others): one that
// isn't the code's any more is left out (stale), one that can't stand for
// its source (i18n.CheckTranslation, the language's number of plural
// forms) stops the build; the strings no file translates are counted
// (missing).
package i18npack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

var (
	// langRe - a language's directory (a code; "sources" and the like
	// are other directories).
	langRe    = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$`)
	versionRe = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,31}$`)
)

// Language - a language of the repository: its code, version, whether it
// is a draft; its strings that stand (Entries, each once), the ones left
// out as not the code's (Stale), the code's strings it doesn't translate
// (Missing).
type Language struct {
	Code, Version string
	Draft         bool
	Entries       []i18n.Entry
	Stale         []i18n.Entry
	Missing       []i18n.Source
}

// Problem - a string that can't stand for its source: where it is (the
// language's file) and why.
type Problem struct {
	File   string
	Source string
	Err    error
}

func (p Problem) Error() string { return fmt.Sprintf("%s: %q: %v", p.File, p.Source, p.Err) }

// manifest - a language's language.yaml.
type manifest struct {
	Version string `yaml:"version"`
	Draft   bool   `yaml:"draft"`
}

// Build reads the repository src: its languages (the directories named as
// a language, English aside), their strings checked against sources. A
// broken file or manifest is an error; the strings that can't stand are
// the problems.
func Build(src fs.FS, sources []i18n.Source) ([]Language, []Problem, error) {
	known := make(map[string]i18n.Source, len(sources))
	for _, s := range sources {
		known[s.Context+"\x04"+s.Source] = s
	}
	dirs, err := fs.ReadDir(src, ".")
	if err != nil {
		return nil, nil, err
	}
	var langs []Language
	var problems []Problem
	for _, d := range dirs {
		if !d.IsDir() || !langRe.MatchString(d.Name()) || d.Name() == i18n.SourceLanguage {
			continue
		}
		l, p, err := buildLanguage(src, d.Name(), known)
		if err != nil {
			return nil, nil, err
		}
		langs = append(langs, l)
		problems = append(problems, p...)
	}
	return langs, problems, nil
}

// buildLanguage reads code's directory.
func buildLanguage(src fs.FS, code string, known map[string]i18n.Source) (Language, []Problem, error) {
	l := Language{Code: code}
	b, err := fs.ReadFile(src, path.Join(code, "language.yaml"))
	if err != nil {
		return l, nil, fmt.Errorf("%s: %w", code, err)
	}
	var m manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return l, nil, fmt.Errorf("%s/language.yaml: %w", code, err)
	}
	if !versionRe.MatchString(m.Version) {
		return l, nil, fmt.Errorf("%s/language.yaml: version %q isn't a version (0.1.0)", code, m.Version)
	}
	l.Version, l.Draft = m.Version, m.Draft
	files, err := fs.Glob(src, path.Join(code, "*.json"))
	if err != nil {
		return l, nil, err
	}
	forms := len(i18n.PluralCategories(code))
	seen := map[string]i18n.Entry{}
	var problems []Problem
	for _, file := range files {
		b, err := fs.ReadFile(src, file)
		if err != nil {
			return l, nil, err
		}
		entries, err := i18n.ParseCatalog(code, b)
		if err != nil {
			return l, nil, fmt.Errorf("%s: %w", file, err)
		}
		for _, e := range entries {
			k := e.Context + "\x04" + e.Source
			if _, ok := known[k]; !ok {
				l.Stale = append(l.Stale, e)
				continue
			}
			if err := check(e, forms); err != nil {
				problems = append(problems, Problem{File: file, Source: e.Source, Err: err})
				continue
			}
			if prev, ok := seen[k]; ok {
				if prev.Text != e.Text || !slices.Equal(prev.Forms, e.Forms) {
					problems = append(problems, Problem{File: file, Source: e.Source, Err: errors.New("another file of the language translates it differently")})
				}
				continue
			}
			seen[k] = e
			l.Entries = append(l.Entries, e)
		}
	}
	for k, s := range known {
		if _, ok := seen[k]; !ok {
			l.Missing = append(l.Missing, s)
		}
	}
	slices.SortFunc(l.Entries, compareEntries)
	slices.SortFunc(l.Missing, func(a, b i18n.Source) int {
		return strings.Compare(a.Context+"\x04"+a.Source, b.Context+"\x04"+b.Source)
	})
	return l, problems, nil
}

func compareEntries(a, b i18n.Entry) int {
	return strings.Compare(a.Context+"\x04"+a.Source, b.Context+"\x04"+b.Source)
}

// check - why e can't stand for its source: an empty translation, plural
// forms not the language's number (or with a text too), a translation
// failing i18n.CheckTranslation.
func check(e i18n.Entry, forms int) error {
	texts := []string{e.Text}
	if len(e.Forms) > 0 {
		if e.Text != "" || len(e.Forms) != forms {
			return fmt.Errorf("%d plural forms, the language has %d", len(e.Forms), forms)
		}
		texts = e.Forms
	}
	for _, t := range texts {
		if t == "" {
			return errors.New("the translation is empty")
		}
		if err := i18n.CheckTranslation(e.Source, t); err != nil {
			return err
		}
	}
	return nil
}

// Index - the catalog's index.json (as service.TranslationsCatalog
// reads it).
type Index struct {
	Generated time.Time    `json:"generated"`
	Languages []IndexEntry `json:"languages"`
}

// IndexEntry - a language of the index.
type IndexEntry struct {
	Code    string `json:"code"`
	Version string `json:"version"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
	Strings int    `json:"strings"`
	Draft   bool   `json:"draft,omitempty"`
}

// Write writes the catalog to the directory out: <code>.json per language
// (its strings that stand) and index.json, generated then.
func Write(out string, langs []Language, generated time.Time) (Index, error) {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return Index{}, err
	}
	index := Index{Generated: generated.UTC()}
	for _, l := range langs {
		b, err := CatalogJSON(l.Entries)
		if err != nil {
			return Index{}, fmt.Errorf("%s: %w", l.Code, err)
		}
		file := l.Code + ".json"
		if err := os.WriteFile(filepath.Join(out, file), b, 0o644); err != nil {
			return Index{}, err
		}
		sum := sha256.Sum256(b)
		index.Languages = append(index.Languages, IndexEntry{Code: l.Code, Version: l.Version, File: file,
			SHA256: hex.EncodeToString(sum[:]), Strings: len(l.Entries), Draft: l.Draft})
	}
	slices.SortFunc(index.Languages, func(a, b IndexEntry) int { return strings.Compare(a.Code, b.Code) })
	b, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return Index{}, err
	}
	return index, os.WriteFile(filepath.Join(out, "index.json"), append(b, '\n'), 0o644)
}

// CatalogJSON - entries as a catalog file (the <lang>.json format: the
// strings, the contexts' strings; a plural one - its forms), keys sorted,
// HTML as it is: the same entries give the same bytes.
func CatalogJSON(entries []i18n.Entry) ([]byte, error) {
	strs := map[string]any{}
	contexts := map[string]map[string]any{}
	for _, e := range entries {
		var v any = e.Text
		if len(e.Forms) > 0 {
			v = e.Forms
		}
		if e.Context == "" {
			strs[e.Source] = v
			continue
		}
		if contexts[e.Context] == nil {
			contexts[e.Context] = map[string]any{}
		}
		contexts[e.Context][e.Source] = v
	}
	file := map[string]any{"strings": strs}
	if len(contexts) > 0 {
		file["contexts"] = contexts
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(file); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
