package searchtext

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Part - a piece of a snippet: text, a word of the query (Match) or not.
type Part struct {
	Text  string
	Match bool
}

// Snippet - a piece of a text around a query's word in it.
type Snippet struct {
	Parts []Part
	// Before, After - the text goes on before (after) the piece.
	Before, After bool
}

// snippetLead - how many letters of the text a snippet shows before the
// first word of the query.
const snippetLead = 60

// isWordRune - a letter of a word (Tokens).
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
}

// span - a word's bytes in a text.
type span struct{ from, to int }

// wordSpans - the words of text (Tokens' runs of letters), as spans.
func wordSpans(text string) []span {
	var out []span
	from := -1
	for i, r := range text {
		switch {
		case isWordRune(r) && from < 0:
			from = i
		case !isWordRune(r) && from >= 0:
			out = append(out, span{from, i})
			from = -1
		}
	}
	if from >= 0 {
		out = append(out, span{from, len(text)})
	}
	return out
}

// matchesWord - whether an indexed word w is one of the query's (a form
// of one, or the beginning of a word for the last).
func (q Query) matchesWord(w string) bool {
	for i, forms := range q.Words {
		for _, f := range forms {
			if w == f || q.Prefix(i) && strings.HasPrefix(w, f) {
				return true
			}
		}
	}
	return false
}

// Snippet - the piece of the plain text around its first word of the
// query (the words stemmed in lang, as Words does): from snippetLead
// letters before it, about width letters, cut between words, the query's
// words marked. False - the text has none of them.
func (q Query) Snippet(text, lang string, width int) (Snippet, bool) {
	spans := wordSpans(text)
	match := make([]bool, len(spans))
	first := -1
	for i, s := range spans {
		w := Fold(text[s.from:s.to])
		if utf8.RuneCountInString(w) < minWordLen {
			continue
		}
		if match[i] = q.matchesWord(cut(Stem(w, lang))); match[i] && first < 0 {
			first = i
		}
	}
	if first < 0 {
		return Snippet{}, false
	}
	// The window: whole words from about snippetLead letters before the
	// first match to about width letters on.
	from := spans[first].from
	for i := first - 1; i >= 0 && utf8.RuneCountInString(text[spans[i].from:spans[first].from]) <= snippetLead; i-- {
		from = spans[i].from
	}
	last := first
	for last+1 < len(spans) && utf8.RuneCountInString(text[from:spans[last+1].to]) <= width {
		last++
	}
	to := spans[last].to
	// At an end of the text: its punctuation too.
	if from == spans[0].from {
		from = 0
	}
	if last == len(spans)-1 {
		to = len(text)
	}
	out := Snippet{Before: strings.TrimSpace(text[:from]) != "", After: strings.TrimSpace(text[to:]) != ""}
	at := from
	for i, s := range spans {
		if s.from < from || s.to > to || !match[i] {
			continue
		}
		if s.from > at {
			out.Parts = append(out.Parts, Part{Text: text[at:s.from]})
		}
		out.Parts = append(out.Parts, Part{Text: text[s.from:s.to], Match: true})
		at = s.to
	}
	if at < to {
		out.Parts = append(out.Parts, Part{Text: text[at:to]})
	}
	return out, true
}
