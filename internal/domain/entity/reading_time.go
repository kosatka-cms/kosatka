package entity

import (
	"strings"
	"unicode"

	"github.com/kosatka-cms/kosatka/pkg/metadescription"
)

// ReadingWordsPerMinute - the reading speed ReadingMinutes assumes for
// the text written with spaces between its words: words a minute.
const ReadingWordsPerMinute = 200

// readingCharsPerMinute - the speed of the scripts written without
// spaces (Han, Hiragana, Katakana): characters a minute.
const readingCharsPerMinute = 500

// ReadingMinutes - the minutes reading the texts (HTML or plain) takes:
// their words at ReadingWordsPerMinute, a character of a script written
// without spaces at readingCharsPerMinute, rounded up; 0 - no text.
func ReadingMinutes(texts ...string) int {
	var words, chars int
	for _, text := range texts {
		if text == "" {
			continue
		}
		for word := range strings.FieldsSeq(metadescription.PlainText(text)) {
			spaced := false
			for _, r := range word {
				switch {
				case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana):
					chars++
				case unicode.IsLetter(r) || unicode.IsNumber(r):
					spaced = true
				}
			}
			// A word has a letter or a digit of a spaced script; a dash
			// or a lone mark isn't one.
			if spaced {
				words++
			}
		}
	}
	if words == 0 && chars == 0 {
		return 0
	}
	// words/ReadingWordsPerMinute + chars/readingCharsPerMinute minutes,
	// over a common denominator, rounded up.
	total := words*readingCharsPerMinute + chars*ReadingWordsPerMinute
	perMinute := ReadingWordsPerMinute * readingCharsPerMinute
	return (total + perMinute - 1) / perMinute
}

// ReadsAsText - a field whose value ReadingMinutesOf counts: a text of
// more than a line (textarea, HTML, the editor) shown on the content
// page.
func (f FieldInstanceDef) ReadsAsText() bool {
	return f.FieldType == FieldTypeText && f.Widget() != WidgetInput && f.ShowOnDetail
}

// ReadingMinutesOf - ReadingMinutes of the values of the fields that
// ReadsAsText; value - a field's value (nil - none).
func ReadingMinutesOf(fields FieldInstanceDefs, value func(FieldInstanceDef) any) int {
	var texts []string
	for _, f := range fields {
		if f.ReadsAsText() {
			texts = append(texts, textValues(value(f))...)
		}
	}
	return ReadingMinutes(texts...)
}

// textValues - a text field's value as its texts: one, or each of a
// multiple field's.
func textValues(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
