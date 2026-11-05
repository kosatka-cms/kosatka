// Package embedtag reads the embed tags of a text: [[<kind>:<name>]]
// stands for something embedded in its place when the text is shown (a
// block, a View's list), and is left out of the text's excerpts.
package embedtag

import (
	"regexp"
	"strings"
)

// Tag - one embed tag: what kind of thing it embeds and its name (a
// machine name or an ID).
type Tag struct {
	Kind, Name string
}

// String - the tag as written.
func (t Tag) String() string { return "[[" + t.Kind + ":" + t.Name + "]]" }

// tagRe - an embed tag: a kind ([a-z][a-z0-9_]*) and a name.
var tagRe = regexp.MustCompile(`\[\[([a-z][a-z0-9_]*):([a-z0-9][a-z0-9_-]*)\]\]`)

// Parse reads s, "<kind>:<name>" (a template's form, without the
// brackets) or a whole tag.
func Parse(s string) (Tag, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[[") {
		s = "[[" + s + "]]"
	}
	m := tagRe.FindStringSubmatch(s)
	if m == nil || m[0] != s {
		return Tag{}, false
	}
	return Tag{Kind: m[1], Name: m[2]}, true
}

// Replace returns s with each tag replaced by what repl returns for it
// (ok=false - the tag stays as written).
func Replace(s string, repl func(Tag) (string, bool)) string {
	if !strings.Contains(s, "[[") {
		return s
	}
	return tagRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := tagRe.FindStringSubmatch(m)
		if out, ok := repl(Tag{Kind: sub[1], Name: sub[2]}); ok {
			return out
		}
		return m
	})
}

// Strip - s without its tags: an excerpt shows the text, not them.
func Strip(s string) string {
	return Replace(s, func(Tag) (string, bool) { return "", true })
}
