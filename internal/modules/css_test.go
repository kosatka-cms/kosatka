package modules

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// bootstrapClassPrefixes - Bootstrap's component classes (a class is one
// when it is a prefix or starts with "<prefix>-": btn, btn-primary).
var bootstrapClassPrefixes = []string{
	"btn", "alert", "form", "input-group", "card", "modal", "badge", "table",
	"nav", "navbar", "dropdown", "pagination", "page", "list-group", "toast",
	"tooltip", "popover", "offcanvas", "accordion", "breadcrumb", "carousel",
	"spinner", "progress",
}

func isBootstrapClass(class string) bool {
	for _, p := range bootstrapClassPrefixes {
		if class == p || strings.HasPrefix(class, p+"-") {
			return true
		}
	}
	return false
}

var (
	cssCommentRe  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssClassRe    = regexp.MustCompile(`\.([a-zA-Z_][a-zA-Z0-9_-]*)`)
	cssScopeOther = regexp.MustCompile(`#[a-zA-Z_][a-zA-Z0-9_-]*|\[[^\]]+\]`)
)

// cssSelectors - the selectors of css's rules (inside at-rules and
// nested rules too), one per comma-separated part.
func cssSelectors(css string) []string {
	css = cssCommentRe.ReplaceAllString(css, "")
	var out []string
	var buf strings.Builder
	for _, r := range css {
		switch r {
		case '{':
			prelude := strings.TrimSpace(buf.String())
			buf.Reset()
			if prelude != "" && !strings.HasPrefix(prelude, "@") {
				out = append(out, splitSelectors(prelude)...)
			}
		case '}', ';':
			buf.Reset()
		default:
			buf.WriteRune(r)
		}
	}
	return out
}

// splitSelectors - a selector list split at its top-level commas (not
// those inside :is(…), :where(…)).
func splitSelectors(list string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range list {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(list[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(list[start:]))
}

// globalBootstrap - whether selector restyles a Bootstrap class
// everywhere: it names one, and nothing of the module's own (a class of
// its, an id, an attribute) limits it.
func globalBootstrap(selector string) bool {
	bootstrap, own := false, cssScopeOther.MatchString(selector)
	for _, m := range cssClassRe.FindAllStringSubmatch(selector, -1) {
		if isBootstrapClass(m[1]) {
			bootstrap = true
		} else {
			own = true
		}
	}
	return bootstrap && !own
}

// TestModuleCSSDoesNotRestyleBootstrap - a module's stylesheet styles
// Bootstrap's classes only under its own markup (".gallery-list .btn"),
// never globally (".btn-primary", "body .alert"): the theme decides how
// they look (docs/themes/assets-and-translations.md); the galleries' global
// .btn-primary once made every theme's buttons on its pages green.
func TestModuleCSSDoesNotRestyleBootstrap(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("*", "static", "*.css"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, ".min.css") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, sel := range cssSelectors(string(b)) {
			if globalBootstrap(sel) {
				t.Errorf("%s: %q restyles a Bootstrap class globally; scope it under the module's own class", f, sel)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no module stylesheets found")
	}
}

func TestGlobalBootstrap(t *testing.T) {
	for sel, want := range map[string]bool{
		".btn-primary":                 true,
		".btn-primary:hover":           true,
		"body .alert":                  true,
		"button.btn":                   true,
		".form-control:focus":          true,
		".gallery-list .btn":           false,
		".btn.gallery-btn":             false,
		"#poll .badge":                 false,
		"[data-forum] .table":          false,
		".image-actions .btn":          false,
		".required":                    false,
		".navigation-x":                false,
		":where(.forum-topic) .btn-sm": false,
		".pagination-like-forum-pager": true,
		".pager-item":                  false,
	} {
		if got := globalBootstrap(sel); got != want {
			t.Errorf("%q: %v, want %v", sel, got, want)
		}
	}
	got := cssSelectors(`/* a, b { } */ .a, .b .btn { color: red; } @media (max-width: 1px) { .c { x: y; .btn { z: 1 } } }`)
	want := []string{".a", ".b .btn", ".c", ".btn"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("selectors: %q, want %q", got, want)
	}
}
