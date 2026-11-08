package service

import "testing"

// TestGetPlainPreview - a feed item's text: the part before
// <!--break-->, blocks apart, entities decoded, cut to characters with "…".
func TestGetPlainPreview(t *testing.T) {
	for _, c := range []struct {
		in    string
		limit int
		want  string
	}{
		{"<p>Первый абзац</p><p>второй &amp; третий</p>", 0, "Первый абзац второй & третий"},
		{"<p>Начало</p><!--break--><p>скрытое</p>", 0, "Начало"},
		{"<p>Первый абзац</p>", 6, "Первый…"},
		{"<p>Run it:</p><pre><code class=\"language-shell\">make test</code></pre><p>Done.</p>", 0, "Run it: Done."},
	} {
		if got := GetPlainPreview(c.in, c.limit); got != c.want {
			t.Errorf("GetPlainPreview(%q, %d) = %q, want %q", c.in, c.limit, got, c.want)
		}
	}
}

// TestPlainLead - the first block of a text as plain text: a body's
// opening paragraph without the headings and lists after it, cut at a
// word; embeds and empty blocks left out; a plain text's first line.
func TestPlainLead(t *testing.T) {
	for _, tc := range []struct {
		in    string
		limit int
		want  string
	}{
		{"<p>Apps for iOS &amp; Android.</p><h2>What's included</h2><ul><li>Design</li></ul>", 0, "Apps for iOS & Android."},
		{"[[block:7]]<p></p><p>First words of a long opening paragraph</p>", 20, "First words of a…"},
		{"Plain text\nsecond line", 0, "Plain text"},
		{"<h2>A heading first</h2><p>Text</p>", 0, "A heading first"},
		{"<pre>go test ./...</pre><p>The tests first.</p>", 0, "The tests first."},
		{"", 10, ""},
	} {
		if got := PlainLead(tc.in, tc.limit); got != tc.want {
			t.Errorf("PlainLead(%q, %d) = %q, want %q", tc.in, tc.limit, got, tc.want)
		}
	}
}

// TestPlainExcerptLeavesOutCode - a text's preview is its prose: the
// code blocks (<pre>, any case, with attributes) left out, the text around
// them kept apart; a text that is all code keeps it; cut at a word.
func TestPlainExcerptLeavesOutCode(t *testing.T) {
	for _, tc := range []struct {
		in    string
		limit int
		want  string
	}{
		{"<p>Build it in one stage:</p><pre><code class=\"language-dockerfile\">FROM golang:1.23 AS build\nRUN go build</code></pre><p>The image went from 800 MB to 12.</p>", 0, "Build it in one stage: The image went from 800 MB to 12."},
		{"<p>First</p><PRE class=\"x\">a &lt; b</PRE><p>second</p><pre>c</pre>", 0, "First second"},
		{"<pre><code>only code &amp; nothing else</code></pre>", 0, "only code & nothing else"},
		{"<p>Retry with backoff, without a library</p><pre>import time</pre>", 16, "Retry with…"},
		{"<p>No code here</p>", 0, "No code here"},
	} {
		if got := PlainExcerpt(tc.in, tc.limit); got != tc.want {
			t.Errorf("PlainExcerpt(%q, %d) = %q, want %q", tc.in, tc.limit, got, tc.want)
		}
	}
}
