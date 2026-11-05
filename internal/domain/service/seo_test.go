package service

import "testing"

func TestRenderSEOTemplate(t *testing.T) {
	values := map[string]string{"a": "A", "b": "B", "empty": ""}
	resolve := func(token string) string { return values[token] }
	for tpl, want := range map[string]string{
		"[a] - [b]":         "A - B",
		"[a]{ ([b])}":       "A (B)",
		"[a]{ ([empty])}":   "A",
		"[a]{ ([empty|b])}": "A (B)",
		"[empty|a|b]":       "A",
		"[unknown]x":        "x",
		"broken [a":         "broken [a",
		"broken {a":         "broken {a",
		"{[a] and [empty]}": "",
		"{ - [a]}{ - [b]}":  " - A - B",
		// Variants: the first whose tokens all render.
		"{[empty] ([a])|[a]}":   "A",
		"{[b] ([a])|[a]}":       "B (A)",
		"{[empty]|[empty]}x":    "x",
		"{[empty|b] ([a])|[a]}": "B (A)",
		"{([empty])|-}":         "-",
	} {
		if got, _ := renderSEOTemplate(tpl, resolve); got != want {
			t.Errorf("%q → %q, want %q", tpl, got, want)
		}
	}
}
