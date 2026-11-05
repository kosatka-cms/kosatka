// Package searchtext turns text into the words a search index keeps and
// a query into the words it looks for: folded (lowercase, "ё" as "е")
// and stemmed in a language (Snowball), so "cats" finds "cat".
package searchtext

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/blevesearch/snowballstem"
	"github.com/blevesearch/snowballstem/arabic"
	"github.com/blevesearch/snowballstem/danish"
	"github.com/blevesearch/snowballstem/dutch"
	"github.com/blevesearch/snowballstem/english"
	"github.com/blevesearch/snowballstem/finnish"
	"github.com/blevesearch/snowballstem/french"
	"github.com/blevesearch/snowballstem/german"
	"github.com/blevesearch/snowballstem/hungarian"
	"github.com/blevesearch/snowballstem/irish"
	"github.com/blevesearch/snowballstem/italian"
	"github.com/blevesearch/snowballstem/norwegian"
	"github.com/blevesearch/snowballstem/portuguese"
	"github.com/blevesearch/snowballstem/romanian"
	"github.com/blevesearch/snowballstem/russian"
	"github.com/blevesearch/snowballstem/spanish"
	"github.com/blevesearch/snowballstem/swedish"
	"github.com/blevesearch/snowballstem/tamil"
	"github.com/blevesearch/snowballstem/turkish"
)

// MaxWordLen - the longest word kept, in bytes (the index column's size);
// a longer one is cut at a letter.
const MaxWordLen = 64

// MaxQueryWords - the words of a query looked for; the rest are ignored.
const MaxQueryWords = 8

// minWordLen - the shortest word kept, in letters.
const minWordLen = 2

// stemmers - the Snowball stemmer of each language (its ISO 639-1 code).
var stemmers = map[string]func(*snowballstem.Env) bool{
	"ar": arabic.Stem, "da": danish.Stem, "de": german.Stem, "en": english.Stem,
	"es": spanish.Stem, "fi": finnish.Stem, "fr": french.Stem, "ga": irish.Stem,
	"hu": hungarian.Stem, "it": italian.Stem, "nb": norwegian.Stem, "nl": dutch.Stem,
	"nn": norwegian.Stem, "no": norwegian.Stem, "pt": portuguese.Stem, "ro": romanian.Stem,
	"ru": russian.Stem, "sv": swedish.Stem, "ta": tamil.Stem, "tr": turkish.Stem,
}

// Fold - s lowercased, "ё" as "е": how the search compares text.
func Fold(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "ё", "е")
}

// Stem - the stem of the folded word in the language lang ("pt-BR" -
// Portuguese); the word as it is in a language without a stemmer.
func Stem(word, lang string) string {
	lang, _, _ = strings.Cut(strings.ToLower(lang), "-")
	lang, _, _ = strings.Cut(lang, "_")
	stem, ok := stemmers[lang]
	if !ok {
		return word
	}
	env := snowballstem.NewEnv(word)
	stem(env)
	if s := env.Current(); s != "" {
		return s
	}
	return word
}

// Tokens - the folded words of text in their order: runs of letters and
// digits, at least 2 letters long.
func Tokens(text string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(Fold(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.Is(unicode.Mn, r)
	}) {
		if utf8.RuneCountInString(w) >= minWordLen {
			out = append(out, w)
		}
	}
	return out
}

// Words - the words of text (plain, not HTML) an index keeps: its tokens
// stemmed in lang, each once, cut to MaxWordLen.
func Words(text, lang string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range Tokens(text) {
		w := cut(Stem(t, lang))
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

// cut - w cut to MaxWordLen bytes, at a letter.
func cut(w string) string {
	if len(w) <= MaxWordLen {
		return w
	}
	n := MaxWordLen
	for n > 0 && !utf8.RuneStart(w[n]) {
		n--
	}
	return w[:n]
}

// Query - a search query: the phrase (folded, single spaces) and its
// words, each as the forms an indexed word may have.
type Query struct {
	Phrase string
	// Words - per word of the query (Tokens, each once, at most
	// MaxQueryWords) its forms: the word and its stems in the languages.
	Words [][]string
}

// ParseQuery - the query q in the languages langs (the site's: the text
// may be in any of them).
func ParseQuery(q string, langs []string) Query {
	out := Query{Phrase: Fold(strings.Join(strings.Fields(q), " "))}
	var seen []string
	for _, t := range Tokens(q) {
		if slices.Contains(seen, t) {
			continue
		}
		seen = append(seen, t)
		forms := []string{cut(t)}
		for _, l := range langs {
			if s := cut(Stem(t, l)); utf8.RuneCountInString(s) >= minWordLen && !slices.Contains(forms, s) {
				forms = append(forms, s)
			}
		}
		out.Words = append(out.Words, forms)
		if len(out.Words) == MaxQueryWords {
			break
		}
	}
	return out
}

// Prefix - whether the i-th word is matched as the beginning of a word:
// the last one, likely still being typed.
func (q Query) Prefix(i int) bool { return i == len(q.Words)-1 }

// Matches - whether the indexed words have every word of the query: one
// of its forms, or (Prefix) a word beginning with one.
func (q Query) Matches(words []string) bool {
	if len(q.Words) == 0 {
		return false
	}
	for i, forms := range q.Words {
		if !slices.ContainsFunc(words, func(w string) bool {
			return slices.ContainsFunc(forms, func(f string) bool {
				return w == f || q.Prefix(i) && strings.HasPrefix(w, f)
			})
		}) {
			return false
		}
	}
	return true
}
