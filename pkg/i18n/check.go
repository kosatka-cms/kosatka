package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Source - an interface string of the code (the source strings' list,
// translations/sources): its context, source (a plural string's
// singular, its catalog key) and plural form, and whose it is - the
// engine's, a module's or a theme's. Starter - a text only a new site's
// starter content has: written once, in the site's language, as its data
// (a language pack translates it; it isn't the interface's).
type Source struct {
	Context string `json:"context,omitempty"`
	Source  string `json:"source"`
	Plural  string `json:"plural,omitempty"`
	Module  string `json:"module,omitempty"`
	Theme   string `json:"theme,omitempty"`
	Starter bool   `json:"starter,omitempty"`
}

// ParseSources - the source strings' list (JSON).
func ParseSources(b []byte) ([]Source, error) {
	var out []Source
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("i18n: the source strings: %w", err)
	}
	return out, nil
}

// placeholderRe - a placeholder: @name, %name or :name at the start or
// after a character that isn't a letter, a digit or "_" (the colon of
// "{{field:name}}" and the "@" of an address aren't placeholders).
var placeholderRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])([@%:][A-Za-z_][A-Za-z0-9_]*)`)

// tagStart - what a browser takes for a tag's start: "<" before a Latin
// letter, "/" and a letter, "!" or "?" (a comment, a declaration). Any
// other "<" ("< ", "<имя>", "<\"") is text, as is ">".
const tagStart = `<(?:[A-Za-z]|/[A-Za-z]|!|\?)`

var (
	// tagRe - an HTML tag (a comment, a declaration).
	tagRe = regexp.MustCompile(tagStart + `[^<>]*>`)
	// tagStartRe - a tag's start, left once the tags are out: one not
	// closed.
	tagStartRe = regexp.MustCompile(tagStart)
)

// TranslationError - why a translation can't stand for its source: a
// placeholder of the source it drops (Placeholder), or HTML the source
// doesn't have (Tag; "<" - a tag not closed).
type TranslationError struct {
	Placeholder string
	Tag         string
}

func (e *TranslationError) Error() string {
	if e.Placeholder != "" {
		return "the translation drops the placeholder " + e.Placeholder
	}
	return "the translation has HTML its source hasn't: " + e.Tag
}

// CheckTranslation - whether text may stand for source: it keeps every
// placeholder of the source (@count may go - a plural form may spell the
// number out) and has no HTML but the source's own tags, as they are (a
// translation is output as HTML, unescaped); a "<" or ">" a browser takes
// for text is text.
func CheckTranslation(source, text string) error {
	for _, m := range placeholderRe.FindAllStringSubmatch(source, -1) {
		if p := m[1]; p != "@count" && !strings.Contains(text, p) {
			return &TranslationError{Placeholder: p}
		}
	}
	allowed := tagRe.FindAllString(normalizeTags(source), -1)
	for _, tag := range tagRe.FindAllString(normalizeTags(text), -1) {
		if !slices.Contains(allowed, tag) {
			return &TranslationError{Tag: tag}
		}
	}
	if tagStartRe.MatchString(tagRe.ReplaceAllString(text, "")) {
		return &TranslationError{Tag: "<"}
	}
	return nil
}

// spaceRe - a run of white space.
var spaceRe = regexp.MustCompile(`\s+`)

// normalizeTags - s with each tag's white space collapsed, so a tag
// compares as written whatever its spacing.
func normalizeTags(s string) string {
	return tagRe.ReplaceAllStringFunc(s, func(tag string) string {
		tag = spaceRe.ReplaceAllString(tag, " ")
		return strings.NewReplacer("< ", "<", " >", ">", " />", "/>").Replace(tag)
	})
}

// CatalogSources - the strings the catalogs of dir (<lang>.json files)
// translate, each once: the strings of a theme or module the sources list
// hasn't (Plural unknown; a plural entry's forms tell it).
func CatalogSources(dir fs.FS) ([]Source, error) {
	if dir == nil {
		return nil, nil
	}
	names, err := fs.Glob(dir, "*.json")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Source
	for _, name := range names {
		b, err := fs.ReadFile(dir, name)
		if err != nil {
			return nil, err
		}
		c, err := parseCatalog(b)
		if err != nil {
			return nil, fmt.Errorf("i18n: %s: %w", name, err)
		}
		for k := range c {
			if seen[k] {
				continue
			}
			seen[k] = true
			src := Source{Source: k}
			if ctx, s, ok := strings.Cut(k, "\x04"); ok {
				src = Source{Context: ctx, Source: s}
			}
			out = append(out, src)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Context != out[j].Context {
			return out[i].Context < out[j].Context
		}
		return out[i].Source < out[j].Source
	})
	return out, nil
}
