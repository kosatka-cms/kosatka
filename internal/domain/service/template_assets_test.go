package service

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestModuleAssetsExist - every asset a module registers has its source
// in its own directory (internal/modules/<module>/static/) or in
// web/static/modules/<module>/.
func TestModuleAssetsExist(t *testing.T) {
	web := filepath.Join("..", "..", "..", "web", "static", "modules")
	own := filepath.Join("..", "..", "..", "internal", "modules")
	exists := func(module, file string) error {
		if _, err := os.Stat(filepath.Join(own, module, "static", file)); err == nil {
			return nil
		}
		_, err := os.Stat(filepath.Join(web, module, file))
		return err
	}
	for _, m := range entity.Modules {
		for _, f := range m.CSS {
			if err := exists(m.Name, f+".css"); err != nil {
				t.Errorf("module %s: CSS %q: %v", m.Name, f, err)
			}
		}
		for _, f := range m.JS {
			if err := exists(m.Name, f+".js"); err != nil {
				t.Errorf("module %s: JS %q: %v", m.Name, f, err)
			}
		}
	}
}

// TestModuleAssetTags - a page includes the assets of the modules it
// attached ({{attach}}, plugin blocks), only while they are enabled; the
// markers leave the page.
func TestModuleAssetTags(t *testing.T) {
	on := map[string]bool{"subscriptions": true, "ratings": true, "galleries": true}
	t.Cleanup(func() { SetModules(on) })
	SetModules(on)
	page := func() string {
		return "<head>" + string(moduleStyles()) + string(moduleScripts()) + "</head><body>" +
			string(attachModule("subscriptions")) + "<p>content</p>" + string(attachPluginBlock("node_rating")) +
			string(attachModule("subscriptions")) + string(attachPluginBlock("logo")) + "</body>"
	}
	got := string(resolveModuleAssets([]byte(page())))
	if !strings.Contains(got, "/static/modules/subscriptions/subscriptions.min.css?v=") || strings.Count(got, "subscriptions.min.css") != 2 { // preload + noscript
		t.Errorf("subscriptions CSS missing or repeated: %s", got)
	}
	if !strings.Contains(got, `<script src="/static/modules/ratings/ratings.min.js?v=`) || !strings.Contains(got, "defer") {
		t.Errorf("the rating block's JS missing: %s", got)
	}
	if strings.Contains(got, "galleries") {
		t.Errorf("galleries not attached, yet included: %s", got)
	}
	if strings.Contains(got, "<!--kosatka:") || !strings.Contains(got, "<body><p>content</p></body>") {
		t.Errorf("markers left: %s", got)
	}

	SetModules(map[string]bool{"subscriptions": false, "ratings": false})
	if got := string(resolveModuleAssets([]byte(page()))); strings.Contains(got, "subscriptions") || strings.Contains(got, "ratings") || strings.Contains(got, "<!--kosatka:") {
		t.Errorf("disabled modules' assets: %s", got)
	}
	// Nothing attached: the placeholders go, nothing else changes.
	if got := string(resolveModuleAssets([]byte("<head>" + string(moduleStyles()) + "</head>"))); got != "<head></head>" {
		t.Errorf("empty page: %s", got)
	}
}

// TestCDNVersionsPinned - a library the templates load from a CDN names an
// exact version (pkg@1.2.3): "latest" or no version would let a new
// release change the site unannounced.
func TestCDNVersionsPinned(t *testing.T) {
	pinned := regexp.MustCompile(`^(@[^/@]+/)?[^/@]+@\d+\.\d+\.\d+(/|$)`)
	cdn := regexp.MustCompile(`https://cdn\.jsdelivr\.net/npm/([^"' )]+)`)
	for _, f := range allTemplateFiles(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range cdn.FindAllStringSubmatch(string(data), -1) {
			if !pinned.MatchString(m[1]) {
				t.Errorf("%s: %s has no exact version", f, m[0])
			}
		}
	}
}

// TestTemplatesLinkNodesByURL - templates link to a node with its URL
// (entity.Node.URL), never with the bare slug: a module's node type with
// a PathPrefix keeps the prefix out of the slug.
func TestTemplatesLinkNodesByURL(t *testing.T) {
	bare := regexp.MustCompile(`(?i)\bnode\.Slug\b`)
	for _, f := range allTemplateFiles(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range bare.FindAllString(string(data), -1) {
			t.Errorf("%s: %s - use .URL", f, m)
		}
	}
}

// allTemplateFiles - every template of the engine, its themes and the
// modules.
func allTemplateFiles(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..", "..")
	var files []string
	for _, dir := range []string{"web/templates", "internal/modules"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".html") && strings.Contains(filepath.ToSlash(path), "/templates/") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(files) == 0 {
		t.Fatal("no templates found")
	}
	return files
}

func TestAttachedModules(t *testing.T) {
	html := []byte(`<p>a</p>` + string(attachModule("ratings")) + `<b>` + string(attachModule("polls")) + string(attachModule("ratings")))
	if got := AttachedModules(html); !slices.Equal(got, []string{"ratings", "polls"}) {
		t.Errorf("AttachedModules = %v", got)
	}
	if got := AttachedModules([]byte("<p>none</p>")); got != nil {
		t.Errorf("no markers: %v", got)
	}
}
