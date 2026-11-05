package textdiff

import (
	"reflect"
	"strings"
	"testing"
)

// render - a side as text: deletions in [-…-], insertions in {+…+},
// paragraphs separated by " | ".
func render(side []Paragraph) string {
	var parts []string
	for _, p := range side {
		var b strings.Builder
		for _, s := range p {
			switch s.Op {
			case Delete:
				b.WriteString("[-" + s.Text + "-]")
			case Insert:
				b.WriteString("{+" + s.Text + "+}")
			default:
				b.WriteString(s.Text)
			}
		}
		parts = append(parts, b.String())
	}
	return strings.Join(parts, " | ")
}

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		name           string
		old, new       []string
		oldOut, newOut string
	}{
		{"equal", []string{"a b", "c"}, []string{"a b", "c"}, "a b | c", "a b | c"},
		{"word changed", []string{"the quick fox"}, []string{"the slow fox"}, "the [-quick-] fox", "the {+slow+} fox"},
		{"word added", []string{"Сеть (Grid)"}, []string{"Смертельный Шторм, Сеть (Grid)"}, "Сеть (Grid)", "{+Смертельный Шторм, +}Сеть (Grid)"},
		{"paragraph added", []string{"one", "three"}, []string{"one", "two", "three"}, "one | three", "one | {+two+} | three"},
		{"paragraph removed", []string{"one", "two"}, []string{"one"}, "one | [-two-]", "one"},
		{"from nothing", nil, []string{"new"}, "", "{+new+}"},
		{"to nothing", []string{"old"}, nil, "[-old-]", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			o, n := Compare(c.old, c.new)
			if got := render(o); got != c.oldOut {
				t.Errorf("old side: %q, want %q", got, c.oldOut)
			}
			if got := render(n); got != c.newOut {
				t.Errorf("new side: %q, want %q", got, c.newOut)
			}
		})
	}
}

// TestCompareTooLarge - beyond the table's limit a paragraph is changed
// as a whole instead of being compared word by word.
func TestCompareTooLarge(t *testing.T) {
	a := strings.Repeat("a ", 1200)
	b := strings.Repeat("b ", 1200)
	o, n := Compare([]string{a}, []string{b})
	want := [][]Paragraph{{{{Text: a, Op: Delete}}}, {{{Text: b, Op: Insert}}}}
	if !reflect.DeepEqual([][]Paragraph{o, n}, want) {
		t.Errorf("got %v / %v", o, n)
	}
}
