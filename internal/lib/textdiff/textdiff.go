// Package textdiff compares two texts for a side-by-side view: by
// paragraphs, then by words inside the paragraphs that changed.
package textdiff

import (
	"regexp"
	"strings"
)

// Op - what happened to a Segment's text.
type Op int

const (
	// Equal - in both texts.
	Equal Op = iota
	// Delete - only in the old text.
	Delete
	// Insert - only in the new text.
	Insert
)

// Segment - a run of text with one Op.
type Segment struct {
	Text string
	Op   Op
}

// Changed - the segment is a deletion or an insertion.
func (s Segment) Changed() bool { return s.Op != Equal }

// Paragraph - one paragraph's segments.
type Paragraph []Segment

// maxCells - the largest table the LCS fills (old × new items): beyond
// it the paragraphs or words are taken as wholly changed.
const maxCells = 1_000_000

// Compare - before and after (paragraphs each) side by side: the old
// side's paragraphs with Delete segments, the new side's with Insert ones.
// Equal paragraphs come as they are; a changed paragraph paired with the
// new one in its place is compared word by word.
func Compare(before, after []string) (oldSide, newSide []Paragraph) {
	pairs := lcs(len(before), len(after), func(i, j int) bool { return before[i] == after[j] })
	i, j := 0, 0
	flush := func(toI, toJ int) {
		dels, ins := before[i:toI], after[j:toJ]
		for k := 0; k < len(dels) || k < len(ins); k++ {
			switch {
			case k < len(dels) && k < len(ins):
				o, n := words(dels[k], ins[k])
				oldSide, newSide = append(oldSide, o), append(newSide, n)
			case k < len(dels):
				oldSide = append(oldSide, Paragraph{{Text: dels[k], Op: Delete}})
			default:
				newSide = append(newSide, Paragraph{{Text: ins[k], Op: Insert}})
			}
		}
		i, j = toI, toJ
	}
	for _, p := range pairs {
		flush(p[0], p[1])
		oldSide = append(oldSide, Paragraph{{Text: before[i]}})
		newSide = append(newSide, Paragraph{{Text: after[j]}})
		i, j = i+1, j+1
	}
	flush(len(before), len(after))
	return oldSide, newSide
}

// tokenRe - a word or a run of white space.
var tokenRe = regexp.MustCompile(`\s+|[^\s]+`)

// words - a and b compared word by word.
func words(a, b string) (Paragraph, Paragraph) {
	at, bt := tokenRe.FindAllString(a, -1), tokenRe.FindAllString(b, -1)
	if len(at)*len(bt) > maxCells {
		return Paragraph{{Text: a, Op: Delete}}, Paragraph{{Text: b, Op: Insert}}
	}
	pairs := lcs(len(at), len(bt), func(i, j int) bool { return at[i] == bt[j] })
	var oldP, newP Paragraph
	i, j := 0, 0
	for _, p := range pairs {
		oldP = appendText(oldP, strings.Join(at[i:p[0]], ""), Delete)
		newP = appendText(newP, strings.Join(bt[j:p[1]], ""), Insert)
		oldP = appendText(oldP, at[p[0]], Equal)
		newP = appendText(newP, bt[p[1]], Equal)
		i, j = p[0]+1, p[1]+1
	}
	oldP = appendText(oldP, strings.Join(at[i:], ""), Delete)
	newP = appendText(newP, strings.Join(bt[j:], ""), Insert)
	return oldP, newP
}

// appendText adds text to p, joining the last segment of the same Op;
// a changed run of white space alone counts as unchanged.
func appendText(p Paragraph, text string, op Op) Paragraph {
	if text == "" {
		return p
	}
	if op != Equal && strings.TrimSpace(text) == "" {
		op = Equal
	}
	if n := len(p); n > 0 && p[n-1].Op == op {
		p[n-1].Text += text
		return p
	}
	return append(p, Segment{Text: text, Op: op})
}

// lcs - the index pairs of a longest common subsequence of n old and m new
// items (eq compares them), in order; none when the table would exceed
// maxCells.
func lcs(n, m int, eq func(i, j int) bool) [][2]int {
	if n == 0 || m == 0 || n*m > maxCells {
		return nil
	}
	// t[i][j] - the LCS length of old[i:] and new[j:].
	w := m + 1
	t := make([]int32, (n+1)*w)
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if eq(i, j) {
				t[i*w+j] = t[(i+1)*w+j+1] + 1
			} else {
				t[i*w+j] = max(t[(i+1)*w+j], t[i*w+j+1])
			}
		}
	}
	var out [][2]int
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case eq(i, j):
			out = append(out, [2]int{i, j})
			i, j = i+1, j+1
		case t[(i+1)*w+j] >= t[i*w+j+1]:
			i++
		default:
			j++
		}
	}
	return out
}
