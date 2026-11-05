package service

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template/parse"

	"github.com/kosatka-cms/kosatka/internal/paths"
)

// Template debugging (SetDebug; config templates_debug, like Twig debug):
// the pages carry HTML comments naming the template files that rendered
// them - the page's template and the theme chain at the top, BEGIN/END
// around each define that outputs HTML elements, the names tried where a
// template is picked by suggestion (renderSuggestion). Defines that
// output text (a URL, a script's value) aren't wrapped: a comment there
// would break it.

// SetDebug turns template debugging on (wiring, at startup).
func (t *DefaultTemplateService) SetDebug(on bool) {
	t.debug = on
}

// templateDefineRe - a define's start in a template file; the body's
// beginning follows it.
var templateDefineRe = regexp.MustCompile(`\{\{-?\s*define\s+"([^"]+)"\s*-?\}\}`)

// debugDefines - the files' defines by name, with the file (relative to
// the working directory) that defines each last, as html/template keeps
// the last definition.
func debugDefines(files []string) map[string]string {
	wd, _ := os.Getwd()
	out := map[string]string{}
	for _, f := range files {
		b, err := paths.ReadTemplate(f)
		if err != nil {
			continue
		}
		rel := f
		if r, err := filepath.Rel(wd, f); err == nil {
			rel = r
		}
		for _, m := range templateDefineRe.FindAllStringSubmatch(string(b), -1) {
			out[m[1]] = rel
		}
	}
	return out
}

// outputsElements - whether a define's first literal text (its actions
// skipped: {{if}}, {{template}}…) starts with an HTML element: its
// output is markup, a comment around it is safe; one of a URL or a
// script's value isn't wrapped.
func outputsElements(tree *parse.Tree) bool {
	if tree == nil || tree.Root == nil {
		return false
	}
	text, _ := firstText(tree.Root)
	return strings.HasPrefix(text, "<")
}

// firstText - the first non-blank literal text of node, depth first.
func firstText(node parse.Node) (string, bool) {
	switch n := node.(type) {
	case *parse.TextNode:
		if t := strings.TrimSpace(string(n.Text)); t != "" {
			return t, true
		}
	case *parse.ListNode:
		if n == nil {
			return "", false
		}
		for _, c := range n.Nodes {
			if t, ok := firstText(c); ok {
				return t, true
			}
		}
	case *parse.IfNode:
		return firstText(n.List)
	case *parse.RangeNode:
		return firstText(n.List)
	case *parse.WithNode:
		return firstText(n.List)
	}
	return "", false
}

// debugComment - an HTML comment (its text can't close it).
func debugComment(text string) template.HTML {
	return template.HTML("<!-- " + strings.ReplaceAll(text, "--", "- -") + " -->")
}

// wrapDebugDefines puts BEGIN/END comments around set's defines that
// output elements (outputsElements; files from debugDefines): an action
// calling templateDebug at both ends of each one's tree, before the
// set's first execution (when html/template escapes it).
func wrapDebugDefines(set *template.Template, fm template.FuncMap, defines map[string]string) error {
	for name, file := range defines {
		tmpl := set.Lookup(name)
		if tmpl == nil || !outputsElements(tmpl.Tree) {
			continue
		}
		probe, err := template.New("debug").Funcs(fm).Parse(`{{templateDebug "BEGIN" ` + quoteTemplate(name) + ` ` + quoteTemplate(file) + `}}{{templateDebug "END" ` + quoteTemplate(name) + ` ""}}`)
		if err != nil {
			return err
		}
		nodes := probe.Tree.Root.Nodes
		root := tmpl.Tree.Root
		root.Nodes = append(append(nodes[:1:1], root.Nodes...), nodes[1])
	}
	return nil
}

// quoteTemplate - s as a template string literal.
func quoteTemplate(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// debugSuggestions - the comment of a template picked by suggestion: the
// names tried in order, the one rendered marked.
func debugSuggestions(tried []string, used string) template.HTML {
	parts := make([]string, len(tried))
	for i, n := range tried {
		parts[i] = n
		if n == used {
			parts[i] = n + " ✓"
		}
	}
	return debugComment("SUGGESTIONS: " + strings.Join(parts, ", "))
}

// debugPageComment - the page's comment: its template and the theme
// chain (paths.ThemeChain; the engine's templates last), put after the
// doctype.
func debugPageComment(out []byte, page string, theme themeScope) []byte {
	chain := append(paths.ThemeChain(theme.name()), "(engine)")
	comment := debugComment("PAGE: " + page + " - theme: " + strings.Join(chain, " → "))
	i := bytes.Index(bytes.ToLower(out), []byte("<!doctype"))
	if i < 0 {
		return append([]byte(comment+"\n"), out...)
	}
	end := bytes.IndexByte(out[i:], '>')
	if end < 0 {
		return out
	}
	at := i + end + 1
	return append(append(append([]byte{}, out[:at]...), "\n"+string(comment)...), out[at:]...)
}
